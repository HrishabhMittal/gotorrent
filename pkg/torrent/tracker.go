package torrent

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const MAGIC_CONSTANT = 0x41727101980

type UDPTracker struct {
	uc            *UDPConnector
	connection_id uint64
}

func NewUDPTracker(rawURL string) (*UDPTracker, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	con, err := NewUDPConnector(u.Host)
	if err != nil {
		return nil, err
	}
	return &UDPTracker{
		uc: con,
	}, nil
}
func (t *UDPTracker) connect() error {
	packet := new(bytes.Buffer)
	binary.Write(packet, binary.BigEndian, uint64(MAGIC_CONSTANT))
	binary.Write(packet, binary.BigEndian, uint32(0))
	tid := rand.Uint32()
	binary.Write(packet, binary.BigEndian, tid)
	t.uc.Send(packet.Bytes())
	resp, _, err := t.uc.Recv(16, 5)
	if err != nil {
		return err
	}
	action := binary.BigEndian.Uint32(resp[:4])
	transaction_id := binary.BigEndian.Uint32(resp[4:8])
	connection_id := binary.BigEndian.Uint64(resp[8:])
	if action == 0 && transaction_id == tid {
		t.connection_id = connection_id
		return nil
	} else {
		return fmt.Errorf("couldn't get a proper response for connection request")
	}
}

type Peer struct {
	IP   net.IP
	port uint16
}

func UnmarshalPeers(data []byte) []Peer {
	const peerSize = 6
	numPeers := len(data) / peerSize
	peers := make([]Peer, numPeers)
	for i := range numPeers {
		offset := i * peerSize
		peers[i].IP = net.IP(data[offset : offset+4])
		peers[i].port = binary.BigEndian.Uint16(data[offset+4 : offset+6])
	}
	return peers
}
func (t *UDPTracker) getPeers(tf *TorrentFile) ([]Peer, error) {
	if t.connection_id == 0 {
		if err := t.connect(); err != nil {
			return nil, err
		}
	}
	tid := rand.Uint32()
	packet := new(bytes.Buffer)
	binary.Write(packet, binary.BigEndian, uint64(t.connection_id))
	binary.Write(packet, binary.BigEndian, uint32(1))
	binary.Write(packet, binary.BigEndian, tid)
	packet.Write(tf.InfoHash[:])
	peerID := []byte(genPeerID("-GT0001-XXXXXXXXXXXX"))
	packet.Write(peerID)
	binary.Write(packet, binary.BigEndian, uint64(0))
	binary.Write(packet, binary.BigEndian, uint64(tf.Length))
	binary.Write(packet, binary.BigEndian, uint64(0))
	binary.Write(packet, binary.BigEndian, uint32(0))
	binary.Write(packet, binary.BigEndian, uint32(0))
	randkey := rand.Uint32()
	binary.Write(packet, binary.BigEndian, randkey)
	binary.Write(packet, binary.BigEndian, int32(-1))
	binary.Write(packet, binary.BigEndian, uint16(6881))
	err := t.uc.Send(packet.Bytes())
	if err != nil {
		return nil, err
	}
	resp, _, err := t.uc.Recv(2048, 1)
	if err != nil {
		return nil, err
	}
	action := binary.BigEndian.Uint32(resp[:4])
	transaction_id := binary.BigEndian.Uint32(resp[4:8])
	if action == 3 {
		errMsg := string(resp[8:])
		return nil, fmt.Errorf("tracker error: %s", errMsg)
	}
	if len(resp) < 20 {
		return nil, fmt.Errorf("response too short")
	}
	if action != 1 || transaction_id != tid {
		return nil, fmt.Errorf("invalid announce response expected: %v %v got %v %v\n", 1, tid, action, transaction_id)
	}
	return UnmarshalPeers(resp[20:]), nil
}

type HTTPTracker struct {
	hc *HTTPConnector
}

func NewHTTPTracker(rawURL string) *HTTPTracker {
	return &HTTPTracker{
		hc: NewHTTPConnector(rawURL),
	}
}

func (ht *HTTPTracker) SendRequest(hr *http.Request) (*http.Response, error) {
	resp, err := ht.hc.client.Do(hr)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (ht *HTTPConnector) getPeers(tf *TorrentFile) ([]Peer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	base, err := url.Parse(ht.baseURL)
	if err != nil {
		return nil, err
	}
	peerID := []byte(genPeerID("-GT0001-XXXXXXXXXXXX"))
	params := url.Values{}
	params.Set("info_hash", string(tf.InfoHash[:]))
	params.Set("peer_id", string(peerID[:]))
	params.Set("port", "6881")
	params.Set("uploaded", "0")
	params.Set("downloaded", "0")
	params.Set("compact", "1")
	params.Set("left", strconv.Itoa(tf.Length))
	base.RawQuery = params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := ht.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tracker responded with status %d", resp.StatusCode)
	}
	var ben bencodeObject
	err = Unmarshal(resp.Body, &ben)
	if err != nil {
		return nil, fmt.Errorf("failed to parse tracker response: %v", err)
	}
	peersObj, err := ben.valAt("peers")
	if err != nil {
		if failObj, failErr := ben.valAt("failure reason"); failErr == nil {
			return nil, fmt.Errorf("tracker failure: %s", failObj.str)
		}
		return nil, fmt.Errorf("peers key missing in tracker response")
	}
	return UnmarshalPeers([]byte(peersObj.str)), nil
}
