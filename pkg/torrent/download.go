package torrent

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type Piece struct {
	id   int64
	data []byte
}

var currentlyDownloading atomic.Int32 = atomic.Int32{}
var failed atomic.Int32 = atomic.Int32{}

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
}

func (d *Downloader) AddPeer(p *PeerCon) {
	d.peerMu.Lock()
	defer d.peerMu.Unlock()
	peerPieces := make(chan Piece)
	go func() {
		p.DownloadLoop(peerPieces)
		close(peerPieces)
	}()
	go d.startRequestWorker(p, peerPieces)
}
func (d *Downloader) Wait() {
	<-d.downloadOver
}
func NewDownloader(tf *TorrentFile) (*Downloader, error) {
	writer, err := NewTorrentWriter(tf)
	if err != nil {
		return nil, fmt.Errorf("failed to create torrent writer: %v", err)
	}
	bfSize := len(tf.PieceHashes)/8 + 1
	down := &Downloader{
		field:        make(Bitfield, bfSize),
		requested:    make(Bitfield, bfSize),
		pieceQueue:   make(chan Piece, 200),
		downloadOver: make(chan struct{}),
		tf:           tf,
		writer:       writer,
		pexCh:        make(chan string, 500),
		seenPeers:    make(map[string]bool),
	}
	go down.processResults()
	confirm := make(chan *PeerCon, 200)
	go down.startDiscovery(confirm)
	go down.processPEX(confirm)
	go down.manageNewPeers(confirm)
	return down, nil
}
func (d *Downloader) processPEX(confirm chan *PeerCon) {
	for {
		select {
		case <-d.downloadOver:
			return
		case addr := <-d.pexCh:
			d.seenMu.Lock()
			if d.seenPeers[addr] {
				d.seenMu.Unlock()
				continue
			}
			d.seenPeers[addr] = true
			d.seenMu.Unlock()
			go func(address string) {
				host, portStr, _ := net.SplitHostPort(address)
				port, _ := strconv.Atoi(portStr)
				p := Peer{
					IP:   net.ParseIP(host),
					port: uint16(port),
				}
				n := NewPeerCon(d.tf, &p, d.field, d.pexCh)
				if err := n.ShakeHands(); err == nil {
					confirm <- n
				}
			}(addr)
		}
	}
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
func (d *Downloader) processResults() {
	logTicker := time.NewTicker(1 * time.Second)
	defer logTicker.Stop()
	for {
		select {
		case <-d.downloadOver:
			return
		case <-logTicker.C:
			fmt.Printf("\rDownloaded: %d/%d pieces | Failed: %d | Active: %d | Searching: %d | Peers: %d | Unchoked: %d | PEX Peers: %d | Seeds: %d   ",
				d.piecesDone, len(d.tf.PieceHashes), failed.Load(), currentlyDownloading.Load(), searching.Load(), numPeers.Load(), unchokedPeers.Load(), len(d.seenPeers), seeders.Load())
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
				fmt.Printf("\n[Error] Hash fail: %d\n", piece.id)
				d.mu.Lock()
				d.requested.ClearPiece(int(piece.id))
				d.mu.Unlock()
				failed.Add(1)
				continue
			}
			err := d.writer.Write(int(piece.id), 0, piece.data)
			if err != nil {
				fmt.Printf("\n[Error] Disk write fail piece %d: %v\n", piece.id, err)
				d.mu.Lock()
				d.requested.ClearPiece(int(piece.id))
				d.mu.Unlock()
				failed.Add(1)
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

var numPeers atomic.Int32 = atomic.Int32{}
var searching atomic.Int32 = atomic.Int32{}
var notFound atomic.Int32 = atomic.Int32{}

func (d *Downloader) startRequestWorker(p *PeerCon, peerPieces chan Piece) {
	numPeers.Add(1)
	defer numPeers.Add(-1)
	timeChoked := 0
	const BlockSize = 16384
	failPiece := func(index int) {
		d.mu.Lock()
		d.requested.ClearPiece(index)
		d.mu.Unlock()
		currentlyDownloading.Add(-1)
		failed.Add(1)
	}
	for {
		select {
		case <-d.downloadOver:
			return
		default:
		}
		for p.choked {
			timeChoked++
			time.Sleep(100 * time.Millisecond)
			if timeChoked == 100 {
				p.con.Close()
				return
			}
			continue
		}
		searching.Add(1)
		timeChoked = 0
		index, found := d.PickPiece(p.peerBitfield)
		if !found {
			time.Sleep(1 * time.Second)
			notFound.Add(1)
			searching.Add(-1)
			continue
		}
		searching.Add(-1)
		currentlyDownloading.Add(1)
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
			case <-time.After(10 * time.Second):
				failPiece(index)
				p.con.Close()
				return
			case <-d.downloadOver:
				return
			}
			err := p.SendRequest(int(index), offset, currentBlockSize)
			if err != nil {
				failPiece(index)
				return
			}
		}
		select {
		case piece, ok := <-peerPieces:
			if !ok {
				failPiece(index)
				return
			}
			d.pieceQueue <- piece
			currentlyDownloading.Add(-1)
		case <-time.After(30 * time.Second):
			failPiece(index)
			p.con.Close()
			return
		}
	}
}
func (d *Downloader) startDiscovery(confirm chan *PeerCon) {
	fmt.Println("[Discovery] Starting tracker announce loop...")
	for {
		select {
		case <-d.downloadOver:
			return
		default:
		}
		for _, tier := range d.tf.AnnounceList {
			for _, announceURL := range tier {
				tracker, err := NewUDPTracker(announceURL)
				if err != nil {
					continue
				}
				peers, err := tracker.getPeers(d.tf)
				if err != nil {
					continue
				}
				for _, v := range peers {
					p := v
					n := NewPeerCon(d.tf, &p, d.field, d.pexCh)
					go func(pCon *PeerCon) {
						if err := pCon.ShakeHands(); err == nil {
							confirm <- pCon
						}
					}(n)
				}
			}
		}
		time.Sleep(time.Second * 30)
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
