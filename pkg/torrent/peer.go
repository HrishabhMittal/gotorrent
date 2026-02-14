package torrent

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
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
	EXTENDED = 20
)
const (
	ExtendedHandshakeID = 0
	UtPexID             = 1
)

type PeerCon struct {
	myBitfield   Bitfield
	peerBitfield Bitfield
	backlog      chan struct{}
	tf           *TorrentFile
	p            *Peer
	con          *TCPConnector
	choked       bool
	pexCh        chan string
	remotePexID  int
	RemotePeerID string
}

func NewPeerCon(tf *TorrentFile, p *Peer, bits Bitfield, pexCh chan string) *PeerCon {
	con := NewTCPConnector(p)
	numPieces := len(tf.PieceHashes)
	bitfieldSize := (numPieces + 7) / 8
	bk := make(chan struct{}, MAX_BACKLOG)
	for range MAX_BACKLOG {
		bk <- struct{}{}
	}
	return &PeerCon{
		tf:           tf,
		p:            p,
		con:          con,
		myBitfield:   bits,
		peerBitfield: make(Bitfield, bitfieldSize),
		choked:       true,
		backlog:      bk,
		pexCh:        pexCh,
		remotePexID:  0,
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
	resp, _, err := p.con.RecvAll(68, 2)
	if err != nil {
		return fmt.Errorf("handshake recv failed: %v", err)
	}
	if len(resp) != 68 {
		return fmt.Errorf("invalid handshake length")
	}
	if !bytes.Equal(p.tf.InfoHash[:], resp[28:48]) {
		return fmt.Errorf("info hash mismatch")
	}
	p.RemotePeerID = string(resp[48:68])
	return nil
}
func (p *PeerCon) SendExtendedHandshake() error {
	payload := []byte("d1:md6:ut_pexi1eee")
	buf := new(bytes.Buffer)
	buf.WriteByte(byte(ExtendedHandshakeID))
	buf.Write(payload)
	return p.SendMessage(&Message{ID: EXTENDED, Payload: buf.Bytes()})
}
func (p *PeerCon) SendBitfield() error {
	return p.SendMessage(&Message{ID: BITFIELD, Payload: p.myBitfield})
}
func (p *PeerCon) ReadMessage() (*Message, error) {
	lenBuf, _, err := p.con.RecvAll(4, 10)
	if err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(lenBuf)
	if length == 0 {
		return nil, nil
	}
	if length > MAX_MSG_LEN {
		return nil, fmt.Errorf("message length too large: %d", length)
	}
	msgBuf, _, err := p.con.RecvAll(int32(length), 10)
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
func (p *PeerCon) SendPiece(index, begin uint32, data []byte) error {
	payload := make([]byte, 8+len(data))
	binary.BigEndian.PutUint32(payload[0:4], index)
	binary.BigEndian.PutUint32(payload[4:8], begin)
	copy(payload[8:], data)
	return p.SendMessage(&Message{ID: PIECE, Payload: payload})
}
func (p *PeerCon) DownloadLoop(d *Downloader, results chan Piece) {
	defer p.con.Close()
	defer func() {
		if !p.choked {
			d.Stats.UnchokedPeers.Add(-1)
		}
	}()
	p.SendExtendedHandshake()
	p.SendUnchoke()
	// p.SendBitfield()
	p.SendInterested()
	pieceBuffers := make(map[uint32][]byte)
	pieceProgress := make(map[uint32]int)
	receivedBitfield := false
	for {
		msg, err := p.ReadMessage()
		if err != nil {
			return
		}
		if msg == nil {
			continue
		}
		if !receivedBitfield && msg.ID != BITFIELD && msg.ID != EXTENDED {
			return
		}
		switch msg.ID {
		case UNCHOKE:
			if p.choked {
				d.Stats.UnchokedPeers.Add(1)
			}
			p.choked = false
		case CHOKE:
			if !p.choked {
				d.Stats.UnchokedPeers.Add(-1)
			}
			p.choked = true
		// case REQUEST:
		// 	if len(msg.Payload) < 12 {
		// 	    continue
		// 	}
		// 	index := binary.BigEndian.Uint32(msg.Payload[0:4])
		// 	begin := binary.BigEndian.Uint32(msg.Payload[4:8])
		// 	length := binary.BigEndian.Uint32(msg.Payload[8:12])
		// 	if d.field.HasPiece(int(index)) {
		// 	    data, err := d.writer.Read(int(index), int(begin), int(length))
		// 	    if err == nil {
		// 	        p.SendPiece(index, begin, data)
		// 	    }
		// 	}
		case HAVE:
			index := binary.BigEndian.Uint32(msg.Payload)
			if int(index) < len(p.peerBitfield)*8 {
				p.peerBitfield.SetPiece(int(index))
			}
		case BITFIELD:
			receivedBitfield = true
			if len(msg.Payload) == len(p.peerBitfield) {
				copy(p.peerBitfield, msg.Payload)
				d.Stats.BitfieldRecv.Add(1)
				seed := true
				for _, v := range msg.Payload {
					if v != 0xff {
						seed = false
						break
					}
				}
				if seed {
					d.Stats.Seeders.Add(1)
				}
			} else {
				d.Stats.BitfieldMiss.Add(1)
			}
		case EXTENDED:
			if len(msg.Payload) < 2 {
				continue
			}
			extendedMsgID := msg.Payload[0]
			payloadData := msg.Payload[1:]
			switch extendedMsgID {
			case 0:
				reader := bytes.NewReader(payloadData)
				ben := &bencodeObject{}
				if err := Unmarshal(reader, ben); err == nil {
					if m, err := ben.valAt("m"); err == nil {
						if utPexObj, err := m.valAt("ut_pex"); err == nil {
							p.remotePexID = int(utPexObj.val)
						}
					}
				}
			case UtPexID:
				reader := bytes.NewReader(payloadData)
				ben := &bencodeObject{}
				if err := Unmarshal(reader, ben); err == nil {
					if added, err := ben.valAt("added"); err == nil {
						peersBytes := []byte(added.str)
						for i := 0; i+6 <= len(peersBytes); i += 6 {
							ip := net.IP(peersBytes[i : i+4])
							port := binary.BigEndian.Uint16(peersBytes[i+4 : i+6])
							p.pexCh <- fmt.Sprintf("%s:%d", ip.String(), port)
						}
					}
				}
			}
		case PIECE:
			select {
			case p.backlog <- struct{}{}:
			default:
			}
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
