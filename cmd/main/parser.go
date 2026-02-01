package main

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"github.com/jackpal/bencode-go"
)
type fileInfo struct {
	Length  int 	`bencode:"length"`
	Path	[]string	`bencode:"path"`
}
type bencodeInfo struct {
    Pieces      string 		`bencode:"pieces"`
    PieceLength int    		`bencode:"piece length"`
    Length      int    		`bencode:"length"`
	Files		[]fileInfo	`bencode:"files"`
    Name        string 		`bencode:"name"`
}
func (i *bencodeInfo) hash() ([20]byte, error) {
	var buf bytes.Buffer
	err := bencode.Marshal(&buf, *i)
	if err != nil {
		return [20]byte{}, err
	}
	h := sha1.Sum(buf.Bytes())
	return h, nil
}
type bencodeTorrent struct {
    Announce 		string      `bencode:"announce"`
	AnnounceList 	[][]string 	`bencode:"announce-list"`
    Info     		bencodeInfo `bencode:"info"`
}

func Open(r io.Reader) (*bencodeTorrent,error) {
    bto:=bencodeTorrent{}
    err:=bencode.Unmarshal(r,&bto)
    if err!=nil {
        return nil,err
    }
    return &bto,nil
}

type TorrentFile struct {
    Announce    string
    AnnounceList [][]string
	InfoHash    [20]byte
    PieceHashes [][20]byte
    PieceLength int
    Length      int
    Name        string
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
func (bto bencodeTorrent) toTorrentFile() (TorrentFile, error) {
	InfoHash, err := bto.Info.hash()
	if err != nil {
		return TorrentFile{},err
	}
	const hashLen = 20
	piecesBytes := []byte(bto.Info.Pieces)
	if len(piecesBytes)%hashLen!=0 {
		return TorrentFile{},fmt.Errorf("invalid hashlen")
	}
	num := len(piecesBytes)/hashLen
	pieceHashes := make([][20]byte,num)
	for i := range num {
		copy(pieceHashes[i][:],piecesBytes[i*hashLen:(i+1)*hashLen])
	}
	length := *bto.Info.Length
	if length == 0 {
		for _, file := range *bto.Info.Files {
			length+=file.Length
		}
	}
	return TorrentFile{
		Announce: bto.Announce,
		AnnounceList: bto.AnnounceList,
		InfoHash: InfoHash,
		PieceHashes: pieceHashes,
		PieceLength: bto.Info.PieceLength,
		Length: length,
		Name: bto.Info.Name,
	}, nil
}
