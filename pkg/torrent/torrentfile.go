package torrent

import (
	"fmt"
	"os"
)

func NewTorrentFile(path string) (*TorrentFile, error) {
	reader, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("couldnt open torrent file: %v", err)
	}
	bt, err := Open(reader)
	if err != nil {
		return nil, fmt.Errorf("couldnt parse torrent file: %v", err)
	}
	tf, err := bt.toTorrentFile()
	if err != nil {
		return nil, fmt.Errorf("couldnt convert to torrent file: %v", err)
	}
	return &tf, nil
}
