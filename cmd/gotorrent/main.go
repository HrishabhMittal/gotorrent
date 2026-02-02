package main
import (
	"fmt"
	"os"
	"github.com/HrishabhMittal/gotorrent/pkg/torrent"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run . <torrent_file>")
		return
	}	

	args := os.Args
	tf, err := torrent.NewTorrentFile(args[1])
	fmt.Printf("Downloading: %s\n", tf.Name)

	dn,err := torrent.NewDownloader(tf)
	if err != nil {
		fmt.Println("couldnt start download:", err)
		return
	}

	dn.Wait()
	fmt.Println("Exiting...")
	err = torrent.Verify(tf)
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Println("Files verified successfully.")
	}
}
