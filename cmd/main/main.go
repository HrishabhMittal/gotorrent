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
	// fmt.Printf("%+v\n",tf)
	fmt.Printf("%v\n",tf.InfoHash)
	for _, tier := range tf.AnnounceList {
	    for _, trackURL := range tier {
	        tracker, err := NewUDPTracker(trackURL)
	        if err != nil { continue }
	        peers, err := tracker.getPeers(&tf)
	        if err == nil {
	            fmt.Printf("found peers: %v\n", peers)
	            return
	        }
	    }
	}
	// tracker,err := NewUDPTracker(tf.Announce)
	// if err != nil {
	// 	fmt.Println("couldnt create udptracker")
	// 	return
	// }
	// peers, err := tracker.getPeers(&tf)
	// if err != nil {
	// 	fmt.Println("couldnt get peers")
	// 	fmt.Printf("err: %v\n", err)
	// 	return
	// }
	// fmt.Printf("peers: %v\n", peers)
}
