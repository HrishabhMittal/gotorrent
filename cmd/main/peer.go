package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

type MessageID uint8

const (
	CHOKE MessageID = iota
	UNCHOKE
	INTERESTED
	NOT_INTERESTED
	HAVE
	BITFIELD
	REQUEST
	PIECE
	CANCEL
)

type PeerCon struct {
	myBitfield   Bitfield
	peerBitfield Bitfield
	tf           *TorrentFile
	p            *Peer
	con          *TCPConnector
	choked       bool
}

func NewPeerCon(tf *TorrentFile, p *Peer, bits Bitfield) *PeerCon {
	con := NewTCPConnector(p)
	
	
	
	numPieces := len(tf.PieceHashes)
	bitfieldSize := (numPieces + 7) / 8

	return &PeerCon{
		tf:           tf,
		p:            p,
		con:          con,
		myBitfield:   bits,
		
		peerBitfield: make(Bitfield, bitfieldSize), 
		choked:       true,
	}
}
func (p *PeerCon) ShakeHands() error {
	req := new(bytes.Buffer)
	binary.Write(req, binary.BigEndian, uint8(19))
	req.Write([]byte("BitTorrent protocol"))
	req.Write([]byte{0, 0, 0, 0, 0, 0x10, 0, 0x05})
	req.Write(p.tf.InfoHash[:])
	req.Write([]byte(genPeerID("-GT0001-XXXXXXXXXXXX")))

	if err := p.con.Send(req.Bytes()); err != nil {
		return err
	}

	resp, _, err := p.con.RecvAll(68, 5)
	if err != nil {
		return fmt.Errorf("handshake recv failed: %v", err)
	}

	if len(resp) != 68 {
		return fmt.Errorf("invalid handshake length")
	}

	
	if !bytes.Equal(p.tf.InfoHash[:], resp[28:48]) {
		return fmt.Errorf("info hash mismatch")
	}

	return nil
}

func (p *PeerCon) ReadMessage() (*Message, error) {
	lenBuf, _, err := p.con.RecvAll(4, 120) 
	if err != nil {
		return nil, err
	}

	length := binary.BigEndian.Uint32(lenBuf)
	if length == 0 {
		return nil, nil 
	}

	msgBuf, _, err := p.con.RecvAll(int32(length), 120)
	if err != nil {
		return nil, err
	}

	return &Message{
		ID:      MessageID(msgBuf[0]),
		Payload: msgBuf[1:],
	}, nil
}

func (p *PeerCon) SendMessage(msg *Message) error {
	return p.con.Send(msg.Serialize())
}

func (p *PeerCon) SendInterested() error {
	return p.SendMessage(&Message{ID: INTERESTED})
}
func (p *PeerCon) SendUnchoke() error {
	return p.SendMessage(&Message{ID: UNCHOKE})
}
func (p *PeerCon) SendRequest(index, begin, length int) error {
	payload := make([]byte, 12)
	binary.BigEndian.PutUint32(payload[0:4], uint32(index))
	binary.BigEndian.PutUint32(payload[4:8], uint32(begin))
	binary.BigEndian.PutUint32(payload[8:12], uint32(length))
	return p.SendMessage(&Message{ID: REQUEST, Payload: payload})
}


func (p *PeerCon) DownloadLoop(workQueue chan int64, results chan Piece) {
	defer p.con.Close()
	p.SendUnchoke()
	p.SendInterested()

	
	pieceBuffers := make(map[uint32][]byte)
	
	pieceProgress := make(map[uint32]int)

	for {
		msg, err := p.ReadMessage()
		if err != nil {
			return
		}
		if msg == nil {
			continue 
		}

		switch msg.ID {
		case UNCHOKE:
			p.choked = false
		case CHOKE:
			p.choked = true
		case HAVE:
			index := binary.BigEndian.Uint32(msg.Payload)
			if int(index) < len(p.peerBitfield)*8 {
				p.peerBitfield.SetPiece(int(index))
			}
		case BITFIELD:
			if len(msg.Payload) == len(p.peerBitfield) {
				copy(p.peerBitfield, msg.Payload)
			}
		case PIECE:
			index := binary.BigEndian.Uint32(msg.Payload[0:4])
			begin := binary.BigEndian.Uint32(msg.Payload[4:8])
			block := msg.Payload[8:]

			
			expectedSize := p.tf.PieceLength
			if int(index) == len(p.tf.PieceHashes)-1 {
				expectedSize = p.tf.Length - (int(index) * p.tf.PieceLength)
			}

			
			if _, exists := pieceBuffers[index]; !exists {
				pieceBuffers[index] = make([]byte, expectedSize)
			}

			
			if int(begin)+len(block) <= expectedSize {
				copy(pieceBuffers[index][begin:], block)
				pieceProgress[index] += len(block)
			}

			
			if pieceProgress[index] == expectedSize {
				results <- Piece{id: int64(index), data: pieceBuffers[index]}
				delete(pieceBuffers, index)
				delete(pieceProgress, index)
			}
		}
	}
}
