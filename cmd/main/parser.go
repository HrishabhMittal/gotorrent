package main

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io"

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


func (bto bencodeTorrent) toTorrentFile() (TorrentFile, error) {
	var buf bytes.Buffer
	err := bencode.Marshal(&buf,bto.Info)
	if err != nil {
		return TorrentFile{},fmt.Errorf("failed to convert to struct")
	}
	InfoHash := sha1.Sum(buf.Bytes())
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
	length := bto.Info.Length
	if length == 0 {
		for _, file := range bto.Info.Files {
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
