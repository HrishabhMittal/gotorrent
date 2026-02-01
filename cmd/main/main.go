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
	fmt.Println(bt.Info.Files)
	fmt.Println(bt.Info.Length)
}
