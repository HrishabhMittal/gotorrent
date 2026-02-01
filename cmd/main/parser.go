package main

import (
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


type TorrentFile struct {
    Announce    string
    InfoHash    [20]byte
    PieceHashes [][20]byte
    PieceLength int
    Length      int
    Name        string
}

func Open(r io.Reader) (*bencodeTorrent,error) {
    bto:=bencodeTorrent{}
    err:=bencode.Unmarshal(r,&bto)
    if err!=nil {
        return nil,err
    }
    return &bto,nil
}

