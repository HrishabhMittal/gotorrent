package torrent

import (
	"crypto/sha1"
	"fmt"
	"io"
	"path/filepath"
)

type FileInfo struct {
	Path   string
	Length int
}

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
	Files        []FileInfo
	Name         string
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
	var files []FileInfo
	if lenObj, err := infoObj.valAt("length"); err == nil {
		totalLength = lenObj.val
		files = append(files, FileInfo{
			Path:   nameObj.str,
			Length: int(lenObj.val),
		})
	} else if filesObj, err := infoObj.valAt("files"); err == nil {
		baseDir := nameObj.str
		for i := 0; i < len(filesObj.list); i++ {
			fObj, _ := filesObj.valAtIndex(i)
			fLen, _ := fObj.valAt("length")
			pathListObj, _ := fObj.valAt("path")
			fullPath := baseDir
			for _, p := range pathListObj.list {
				fullPath = filepath.Join(fullPath, p.str)
			}
			files = append(files, FileInfo{
				Path:   fullPath,
				Length: int(fLen.val),
			})
			totalLength += fLen.val
		}
	}
	announceObj, _ := bto.valAt("announce")
	announceListObj, _ := bto.valAt("announce-list")
	var announceList [][]string
	if announceListObj.list != nil {
		for _, v := range announceListObj.list {
			announceList = append(announceList, []string{})
			for _, v2 := range v.list {
				announceList[len(announceList)-1] = append(announceList[len(announceList)-1], v2.str)
			}
		}
	} else {
		announceList = [][]string{{announceObj.str}}
	}
	return TorrentFile{
		Announce:     announceObj.str,
		AnnounceList: announceList,
		InfoHash:     infoHash,
		PieceHashes:  pieceHashes,
		PieceLength:  int(pieceLengthObj.val),
		Length:       int(totalLength),
		Name:         nameObj.str,
		Files:        files,
	}, nil
}
