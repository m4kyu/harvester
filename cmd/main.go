package main

import (
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/m4kyu/harvester/internal/torrent"
	"github.com/m4kyu/harvester/internal/tracker"
	"github.com/m4kyu/harvester/internal/p2p"
)




func main() {
	torrent, _ := torrent.FromFile("debian.torrent")
	fmt.Println("\t\t", torrent.Info.Name)
	fmt.Println("Pieces len: ", torrent.Info.PieceLen)

	fmt.Println("Hash: ", hex.EncodeToString(torrent.InfoHash[:]))
	fmt.Printf("Created by: %v at: %v\n", torrent.CreatedBy, torrent.Created)
	fmt.Println("Comment: ", torrent.Comment)
	fmt.Println(torrent.Announce)
	
	fmt.Println("\t\tAnnounces")
	for _, i := range torrent.AnnouncesList {
		fmt.Println(i)
	}


	peers, err := tracker.PeersList(torrent)
	if err != nil {
		fmt.Println("Couldnt get peers list")
		return 
	}


	var wg sync.WaitGroup
	for _, i := range peers {
		fmt.Printf("IP: %v. PORT: %v\n", i.IP, i.Port)
		
    wg.Add(1)
		go p2p.HandlePeer(i, torrent, &wg)
	}

  
	wg.Wait()
}





