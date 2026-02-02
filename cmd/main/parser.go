package main

import (
	"crypto/sha1"
	"fmt"
	"io"
	"net/url"
	"strconv"
)

func Open(r io.Reader) (*bencodeObject, error) {
	bto := bencodeObject{}
	err := Unmarshal(r, &bto)
	if err != nil {
		return nil, err
	}
	return &bto, nil
}

type TorrentFile struct {
	Announce     string
	AnnounceList [][]string
	InfoHash     [20]byte
	PieceHashes  [][20]byte
	PieceLength  int
	Length       int
	Name         string
}

func (t *TorrentFile) requestHTTP(peerID [20]byte, port uint16) (string, error) {
	base, err := url.Parse(t.Announce)
	if err != nil {
		return "", err
	}
	params := url.Values{
		"info_hash":  []string{string(t.InfoHash[:])},
		"peer_id":    []string{string(peerID[:])},
		"port":       []string{strconv.Itoa(int(port))},
		"uploaded":   []string{"0"},
		"downloaded": []string{"0"},
		"compact":    []string{"1"},
		"left":       []string{strconv.Itoa(t.Length)},
	}
	base.RawQuery = params.Encode()
	return base.String(), nil
}
func (bto *bencodeObject) toTorrentFile() (TorrentFile, error) {
	infoObj, err := bto.valAt("info")
	if err != nil {
		return TorrentFile{}, fmt.Errorf("missing info dictionary: %v", err)
	}
	marshaledInfo, err := infoObj.Marshal()
	if err != nil {
		return TorrentFile{}, fmt.Errorf("failed to marshal info for hashing: %v", err)
	}
	infoHash := sha1.Sum([]byte(marshaledInfo))
	nameObj, _ := infoObj.valAt("name")
	pieceLengthObj, _ := infoObj.valAt("piece length")
	piecesObj, _ := infoObj.valAt("pieces")
	const hashLen = 20
	piecesBytes := []byte(piecesObj.str)
	if len(piecesBytes)%hashLen != 0 {
		return TorrentFile{}, fmt.Errorf("invalid pieces hash length")
	}
	numPieces := len(piecesBytes) / hashLen
	pieceHashes := make([][20]byte, numPieces)
	for i := range numPieces {
		copy(pieceHashes[i][:], piecesBytes[i*hashLen:(i+1)*hashLen])
	}
	var totalLength int64
	if lenObj, err := infoObj.valAt("length"); err == nil {
		totalLength = lenObj.val
	} else if filesObj, err := infoObj.valAt("files"); err == nil {
		for i := 0; i < len(filesObj.list); i++ {
			f, _ := filesObj.valAtIndex(i)
			fLen, _ := f.valAt("length")
			totalLength += fLen.val
		}
	}
	announceObj, _ := bto.valAt("announce")
	announceListObj, _ := bto.valAt("announce-list")
	var announceList [][]string
	for _, v := range announceListObj.list {
		announceList = append(announceList, []string{})
		for _, v2 := range v.list {
			announceList[len(announceList)-1] = append(announceList[len(announceList)-1], v2.str)
		}
	}
	return TorrentFile{
		Announce:     announceObj.str,
		AnnounceList: announceList,
		InfoHash:     infoHash,
		PieceHashes:  pieceHashes,
		PieceLength:  int(pieceLengthObj.val),
		Length:       int(totalLength),
		Name:         nameObj.str,
	}, nil
}
