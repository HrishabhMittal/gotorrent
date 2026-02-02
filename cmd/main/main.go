package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run . <torrent_file>")
		return
	}

	args := os.Args
	reader, err := os.Open(args[1])
	if err != nil {
		fmt.Println("couldnt open torrent file:", err)
		return
	}

	bt, err := Open(reader)
	if err != nil {
		fmt.Println("couldnt parse torrent file:", err)
		return
	}

	tf, err := bt.toTorrentFile()
	if err != nil {
		fmt.Println("couldnt convert to torrent file:", err)
		return
	}

	fmt.Printf("Downloading: %s\n", tf.Name)

	// Open file and start downloading
	dn, err := NewDownloader(&tf)
	if err != nil {
		fmt.Println("couldnt start download:", err)
		return
	}

	// BLOCK here until download finishes
	dn.Wait()

	fmt.Println("Exiting...")
}
