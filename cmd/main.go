package main

import (
	"fmt"
	"encoding/hex"
	"github.com/m4kyu/harvester/internal/torrent"
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


}
