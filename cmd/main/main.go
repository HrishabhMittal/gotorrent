package main

import (
	"fmt"
	"os"
)


func main() {
	args := os.Args
	reader, err := os.Open(args[1])
	if err != nil {
		return	
	}
	bt,err := Open(reader)
	if err != nil {
		return	
	}
	tf, err := bt.toTorrentFile()
	if err != nil {
		return	
	}
	fmt.Printf("%+v\n",tf)
}
