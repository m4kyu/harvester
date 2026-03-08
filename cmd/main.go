package main

import (
	"encoding/hex"
	"fmt"

	"github.com/m4kyu/harvester/internal/harvester"
	"github.com/m4kyu/harvester/internal/torrent"
)

func main() {
	torrent, _ := torrent.FromFile("debian-13.torrent")
	fmt.Println("\t\t", torrent.Info.Name)
	fmt.Println("Pieces len: ", torrent.Info.PieceSize)
	fmt.Println("Pieces count: ", torrent.PiecesCount)

	fmt.Println("Hash: ", hex.EncodeToString(torrent.InfoHash[:]))
	fmt.Printf("Created by: %v at: %v\n", torrent.CreatedBy, torrent.Created)
	fmt.Println("Comment: ", torrent.Comment)
	fmt.Println(torrent.Announce)

	fmt.Println("\t\tAnnounces")
	for _, i := range torrent.AnnouncesList {
		fmt.Println(i)
	}

	err := harvester.DownloadTorrent(torrent)
	if err != nil {
		fmt.Printf(err.Error())
	}
}
