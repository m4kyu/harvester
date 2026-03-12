package harvester

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
		fmt.Printf("Downloaded piece from %v peers\n", runtime.NumGoroutine()-1)

	}

	close(workQueue)
	close(resultQueue)

	ex, err := os.Executable()
	if err != nil {
		return err
	}

	rootPath := filepath.Dir(ex)
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return err
	}

	if !torrent.IsMultiFile {
		err = root.WriteFile(torrent.Info.Name, buffer, 0o644)
		if err != nil {
			fmt.Println("ERROR: ", err)
			return err
		}

		return nil
	}

	err = root.Mkdir(torrent.Info.Name, 0o644)
	if err != nil {
		return err
	}

	offset := 0
	for _, file := range torrent.Info.Files {
		if offset > torrent.Info.Len {
			return fmt.Errorf("torrent size doesnt match")
		}

		path := filepath.Join(file.Path...)
		path = filepath.Join(torrent.Info.Name, path)
		err := root.WriteFile(path, buffer[offset:file.Len], 0o644)
		if err != nil {
			return err
		}

		offset += file.Len
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
