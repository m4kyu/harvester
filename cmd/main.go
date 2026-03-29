package main

import (
	"encoding/hex"
	"fmt"

	"github.com/m4kyu/harvester/internal/harvester"
	"github.com/m4kyu/harvester/internal/torrent"
)

func main() {
	torrent, err := torrent.FromFile("debian.torrent")
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Println("\t\t", torrent.Info.Name)
	fmt.Println("Pieces len: ", torrent.Info.PieceSize)
	fmt.Println("Pieces count: ", torrent.PiecesCount)

	fmt.Println("Hash: ", hex.EncodeToString(torrent.InfoHash[:]))
	fmt.Printf("Created by: %v at: %v\n", torrent.CreatedBy, torrent.Created)
	fmt.Println("Comment: ", torrent.Comment)
	fmt.Println(torrent.Announce)

	if torrent.IsMultiFile {
		fmt.Printf("Its a multi file torrent with total len of %v bytes\n", torrent.Info.Len)
		for _, file := range torrent.Info.Files {
			fmt.Printf("Name: %v. Len: %v.\n", file.Path, file.Len)
		}
	} else {
		fmt.Printf("Its a single file torrent with total len of %v bytes\n", torrent.Info.Len)
	}

	fmt.Println("\t\tAnnounces")
	for _, i := range torrent.AnnouncesList {
		fmt.Println(i)
	}

	err = harvester.DownloadTorrent(torrent)
	if err != nil {
		fmt.Printf(err.Error())
	}
}
