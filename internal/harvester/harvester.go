package harvester

import (
	"fmt"
	"os"
	"sync"

	"github.com/m4kyu/harvester/internal/p2p"
	"github.com/m4kyu/harvester/internal/torrent"
	"github.com/m4kyu/harvester/internal/tracker"
)

func DownloadTorrent(torrent torrent.Torrent) error {
	peers, err := tracker.PeersList(torrent)
	if err != nil {
		fmt.Println("Couldnt get peers list")
		return err
	}

	workQueue := make(chan p2p.Piece, torrent.PiecesCount)
	prepearWorkChan(workQueue, torrent)

	resultQueue := make(chan p2p.Piece, torrent.PiecesCount)

	var wg sync.WaitGroup
	for i := range len(peers) {
		peer := peers[i]
		fmt.Printf("IP: %v. PORT: %v\n", peer.IP, peer.Port)

		wg.Add(1)
		go p2p.HandlePeer(peer, torrent, &wg, workQueue, resultQueue)
	}

	fmt.Println("Peers count: ", len(peers))

	buffer := make([]byte, torrent.Info.Len)
	downloaded := 0
	for downloaded < torrent.PiecesCount {
		piece := <-resultQueue
		begin, end := torrent.PieceBounds(piece.Index)
		copy(buffer[begin:end], piece.Data)

		downloaded++
	}

	close(workQueue)
	close(resultQueue)

	err = os.WriteFile("debian.iso", buffer, 0o644)
	if err != nil {
		fmt.Println("ERROR: ", err)
		return err
	}

	return nil
}

func prepearWorkChan(queue chan p2p.Piece, torrent torrent.Torrent) {
	for i := range torrent.PiecesCount {
		var hash [20]byte
		copy(hash[:], torrent.Info.Pieces[i*20:(i+1)*20])
		queue <- p2p.Piece{Index: i, Hash: hash}
	}
}
