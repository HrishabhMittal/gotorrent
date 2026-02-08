package torrent

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"
)

type Piece struct {
	id   int64
	data []byte
}

type Downloader struct {
	peerMu       sync.Mutex
	pieceQueue   chan Piece
	field        Bitfield
	requested    Bitfield
	mu           sync.Mutex
	downloadOver chan struct{}
	tf           *TorrentFile
	piecesDone   int
	writer       *TorrentWriter
	pexCh        chan string
	seenPeers    map[string]bool
	seenMu       sync.Mutex
	Stats        Stats
}

func NewDownloader(tf *TorrentFile) (*Downloader, error) {
	writer, err := NewTorrentWriter(tf, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create torrent writer: %v", err)
	}
	bfSize := len(tf.PieceHashes)/8 + 1
	down := &Downloader{
		field:        make(Bitfield, bfSize),
		requested:    make(Bitfield, bfSize),
		pieceQueue:   make(chan Piece, PIECE_QUEUE),
		downloadOver: make(chan struct{}),
		tf:           tf,
		writer:       writer,
		pexCh:        make(chan string, PEX_CHANNEL),
		seenPeers:    make(map[string]bool),
	}
	limit := make(chan struct{}, DISCOVERY_LIMIT)
	writer.d = down
	down.Stats.StartTime = time.Now()
	down.Stats.TotalSize = tf.DownloadLength()
	go down.processResults()
	confirm := make(chan *PeerCon, CONFIRMED_PEER_QUEUE)

	go down.startDiscovery(confirm, limit)
	go down.processPEX(confirm, limit)
	go down.manageNewPeers(confirm)

	return down, nil
}

func (d *Downloader) attemptConnection(p Peer, limit chan struct{}, confirm chan *PeerCon) {
	limit <- struct{}{}
	defer func() { <-limit }()
	d.Stats.PeersProcessed.Add(1)
	n := NewPeerCon(d.tf, &p, d.field, d.pexCh)
	if err := n.ShakeHands(); err == nil {
		d.Stats.PeersConfirmed.Add(1)
		confirm <- n
	} else {
		d.Stats.PeersDenied.Add(1)
	}
}

func (d *Downloader) startDiscovery(confirm chan *PeerCon, limit chan struct{}) {
	for {
		select {
		case <-d.downloadOver:
			return
		default:
		}

		d.Stats.ValidTrackers.Store(0)
		for _, tier := range d.tf.AnnounceList {
			for _, announceURL := range tier {
				go func(url string) {
					var peers []Peer
					var err error
					switch url[0] {
					case 'h':
						tracker := NewHTTPTracker(url)
						peers, err = tracker.hc.getPeers(d.tf)
					case 'u':
						tracker, err := NewUDPTracker(url)
						if err == nil {
							peers, err = tracker.getPeers(d.tf)
						}
					}

					if err != nil || len(peers) == 0 {
						return
					}

					d.Stats.ValidTrackers.Add(1)

					for _, v := range peers {
						addr := fmt.Sprintf("%s:%d", v.IP.String(), v.port)

						d.seenMu.Lock()
						if d.seenPeers[addr] {
							d.seenMu.Unlock()
							continue
						}
						d.seenPeers[addr] = true
						d.Stats.PeersProvided.Add(1)
						d.seenMu.Unlock()

						go d.attemptConnection(v, limit, confirm)
					}
				}(announceURL)
			}
		}
		time.Sleep(1 * time.Minute)
	}
}

func (d *Downloader) processPEX(confirm chan *PeerCon, limit chan struct{}) {
	for {
		select {
		case <-d.downloadOver:
			return
		case addr := <-d.pexCh:
			d.Stats.PexProcessed.Add(1)
			d.seenMu.Lock()
			if d.seenPeers[addr] {
				d.seenMu.Unlock()
				continue
			}
			d.seenPeers[addr] = true
			d.seenMu.Unlock()
			go func(address string) {
				host, portStr, err := net.SplitHostPort(address)
				if err != nil {
					return
				}
				port, _ := strconv.Atoi(portStr)
				p := Peer{
					IP:   net.ParseIP(host),
					port: uint16(port),
				}
				d.Stats.PexAdded.Add(1)
				d.attemptConnection(p, limit, confirm)
			}(addr)
		}
	}
}

func (d *Downloader) manageNewPeers(confirm chan *PeerCon) {
	for {
		select {
		case <-d.downloadOver:
			return
		case ans := <-confirm:
			if ans != nil {
				d.AddPeer(ans)
			}
		}
	}
}

func (d *Downloader) AddPeer(p *PeerCon) {
	d.peerMu.Lock()
	defer d.peerMu.Unlock()
	peerPieces := make(chan Piece)
	go func() {
		p.DownloadLoop(d, peerPieces)
		close(peerPieces)
	}()
	go d.startRequestWorker(p, peerPieces)
}

func (d *Downloader) PickPiece(peerBitfield Bitfield) (int, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := 0; i < len(d.tf.PieceHashes); i++ {
		if !d.field.HasPiece(i) && !d.requested.HasPiece(i) && peerBitfield.HasPiece(i) {
			d.requested.SetPiece(i)
			return i, true
		}
	}
	return 0, false
}

func (d *Downloader) startRequestWorker(p *PeerCon, peerPieces chan Piece) {
	d.Stats.NumPeers.Add(1)
	defer d.Stats.NumPeers.Add(-1)

	const BlockSize = REQUEST_BLOCK_SIZE

	timeChoked := int64(0)

	failPiece := func(index int) {
		d.mu.Lock()
		d.requested.ClearPiece(index)
		d.mu.Unlock()
		d.Stats.CurrentlyDownloading.Add(-1)
		d.Stats.Failed.Add(1)
	}

	for {
		select {
		case <-d.downloadOver:
			return
		default:
		}

		for p.choked {
			time.Sleep(100 * time.Millisecond)
			if timeChoked >= int64(MAX_CHOKED_TIME) {
				p.con.Close()
				return
			}
			timeChoked += int64(100 * time.Millisecond)
			continue
		}
		timeChoked = 0

		d.Stats.Searching.Add(1)
		index, found := d.PickPiece(p.peerBitfield)
		d.Stats.Searching.Add(-1)

		if !found {
			d.Stats.NotFound.Add(1)
			time.Sleep(1 * time.Second)
			continue
		}

		d.Stats.CurrentlyDownloading.Add(1)
		pieceSize := d.tf.PieceLength
		if int64(index) == int64(len(d.tf.PieceHashes)-1) {
			pieceSize = d.tf.Length - (int(index) * d.tf.PieceLength)
		}
		for offset := 0; offset < pieceSize; offset += BlockSize {
			currentBlockSize := BlockSize
			if offset+currentBlockSize > pieceSize {
				currentBlockSize = pieceSize - offset
			}

			select {
			case <-p.backlog:
			case <-time.After(15 * time.Second):
				failPiece(index)
				p.con.Close()
				return
			case <-d.downloadOver:
				return
			}

			if err := p.SendRequest(int(index), offset, currentBlockSize); err != nil {
				failPiece(index)
				return
			}
		}

		select {
		case piece, ok := <-peerPieces:
			d.Stats.CurrentlyDownloading.Add(-1)
			if !ok {
				failPiece(index)
				return
			}
			d.pieceQueue <- piece
		case <-time.After(30 * time.Second):
			failPiece(index)
			p.con.Close()
			return
		}
	}
}

func (d *Downloader) PrintLogs() {
	logTicker := time.NewTicker(1 * time.Second)
	defer logTicker.Stop()
	for {
		<-logTicker.C
		d.printStats()
	}
}

func (d *Downloader) processResults() {
	for {
		select {
		case <-d.downloadOver:
			return
		case piece := <-d.pieceQueue:
			d.mu.Lock()
			if d.field.HasPiece(int(piece.id)) {
				d.mu.Unlock()
				continue
			}
			d.mu.Unlock()

			hash := sha1.Sum(piece.data)
			expected := d.tf.PieceHashes[piece.id]
			if !bytes.Equal(hash[:], expected[:]) {
				d.mu.Lock()
				d.requested.ClearPiece(int(piece.id))
				d.mu.Unlock()
				d.Stats.Failed.Add(1)
				continue
			}

			if err := d.writer.Write(int(piece.id), 0, piece.data); err != nil {
				d.mu.Lock()
				d.requested.ClearPiece(int(piece.id))
				d.mu.Unlock()
				d.Stats.Failed.Add(1)
				continue
			}

			d.mu.Lock()
			d.field.SetPiece(int(piece.id))
			d.piecesDone++
			d.mu.Unlock()

			if d.piecesDone == len(d.tf.PieceHashes) {
				fmt.Println("\nDownload Complete!")
				close(d.downloadOver)
				return
			}
		}
	}
}

func (d *Downloader) Wait() {
	<-d.downloadOver
}
