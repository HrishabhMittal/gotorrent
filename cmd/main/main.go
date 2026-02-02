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

	dn, err := NewDownloader(&tf)
	if err != nil {
		fmt.Println("couldnt start download:", err)
		return
	}

	dn.Wait()

	fmt.Println("Exiting...")
	err = Verify(&tf)
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Println("Files verified successfully.")
	}
}
