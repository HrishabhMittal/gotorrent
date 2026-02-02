package main

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"sync"
	"time"
)

type Piece struct {
	id   int64
	data []byte
}

type Downloader struct {
	p            []*PeerCon
	peerMu       sync.Mutex
	pieceQueue   chan Piece
	field        Bitfield
	workQueue    chan int64
	downloadOver chan struct{}
	tf           *TorrentFile
	piecesDone   int
	writer       *TorrentWriter
}

func (d *Downloader) AddPeer(p *PeerCon) {
	d.peerMu.Lock()
	defer d.peerMu.Unlock()
	d.p = append(d.p, p)
	go p.DownloadLoop(d.workQueue, d.pieceQueue)
	go d.startRequestWorker(p)
}

func (d *Downloader) Wait() {
	<-d.downloadOver
}

func NewDownloader(tf *TorrentFile) (*Downloader, error) {

	writer, err := NewTorrentWriter(tf)
	if err != nil {
		return nil, fmt.Errorf("failed to create torrent writer: %v", err)
	}

	down := &Downloader{
		field:        make(Bitfield, len(tf.PieceHashes)/8+1),
		pieceQueue:   make(chan Piece, 200),
		workQueue:    make(chan int64, len(tf.PieceHashes)),
		downloadOver: make(chan struct{}),
		tf:           tf,
		writer:       writer,
	}

	for i := 0; i < len(tf.PieceHashes); i++ {
		down.workQueue <- int64(i)
	}

	go down.processResults()

	confirm := make(chan *PeerCon, 200)
	go down.startDiscovery(confirm)
	go down.manageNewPeers(confirm)

	return down, nil
}

func (d *Downloader) processResults() {

	for {
		select {
		case <-d.downloadOver:
			return
		case piece := <-d.pieceQueue:
			if d.field.HasPiece(int(piece.id)) {
				continue
			}
			hash := sha1.Sum(piece.data)
			expected := d.tf.PieceHashes[piece.id]
			if !bytes.Equal(hash[:], expected[:]) {
				fmt.Printf("\rHash fail: %d. Retrying...", piece.id)
				d.workQueue <- piece.id
				continue
			}

			err := d.writer.Write(int(piece.id), 0, piece.data)
			if err != nil {
				fmt.Printf("Disk error piece %d: %v\n", piece.id, err)
				d.workQueue <- piece.id
				continue
			}

			d.field.SetPiece(int(piece.id))
			d.piecesDone++
			fmt.Printf("\rDownloaded: %d/%d pieces", d.piecesDone, len(d.tf.PieceHashes))

			if d.piecesDone == len(d.tf.PieceHashes) {
				fmt.Println("\nDownload Complete!")
				close(d.downloadOver)
				return
			}
		}
	}
}

func (d *Downloader) startRequestWorker(p *PeerCon) {
	const BlockSize = 16384
	for {
		select {
		case <-d.downloadOver:
			return
		default:
		}

		if p.choked {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		index := <-d.workQueue

		if !p.peerBitfield.HasPiece(int(index)) {
			go func() {
				time.Sleep(1 * time.Second)
				d.workQueue <- index
			}()
			continue
		}

		pieceSize := d.tf.PieceLength
		if index == int64(len(d.tf.PieceHashes)-1) {
			pieceSize = d.tf.Length - (int(index) * d.tf.PieceLength)
		}

		for offset := 0; offset < pieceSize; offset += BlockSize {
			currentBlockSize := BlockSize
			if offset+currentBlockSize > pieceSize {
				currentBlockSize = pieceSize - offset
			}

			err := p.SendRequest(int(index), offset, currentBlockSize)
			if err != nil {
				d.workQueue <- index
				return
			}
		}
	}
}

func (d *Downloader) startDiscovery(confirm chan *PeerCon) {
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
					n := NewPeerCon(d.tf, &p, d.field)
					go func(pCon *PeerCon) {
						if err := pCon.ShakeHands(); err == nil {
							confirm <- pCon
						}
					}(n)
				}
			}
		}
		time.Sleep(2 * time.Minute)
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
