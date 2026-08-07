package harvester

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/m4kyu/harvester/internal/bitfield"
	"github.com/m4kyu/harvester/internal/p2p"
	"github.com/m4kyu/harvester/internal/piecemanager"
	"github.com/m4kyu/harvester/internal/storage"
	"github.com/m4kyu/harvester/internal/torrent"
)

type Harvester struct {
	Bitfield  bitfield.Bitfield
	WriteErrC chan error
	ReadErrC  chan error
}

type File struct {
	FD   *os.File
	Size int
}

func DownloadTorrent(torrent torrent.Torrent) error {
	//	client := Harvester{
	//		Bitfield: make([]byte, (torrent.PiecesCount+8-1)/8),
	//	}

	pieces := make(chan p2p.Piece, torrent.PiecesCount)

	pieceManager, err := piecemanager.Init(torrent)
	if err != nil {
		return err
	}

	time.Sleep(1 * time.Second)
	fmt.Println("Peers count: ", pieceManager.PeerM.PeersCount())

	go pieceManager.PiecesManager(pieces)

	st, err := storage.Init(torrent)
	if err != nil {
		return err
	}
	defer st.Finish()

	for pieceManager.Downloaded() < torrent.PiecesCount {
		piece := <-pieces
		fmt.Printf("Got piece: %v\n", piece.Index)
		pieceManager.NextPiece <- struct{}{}

		err = st.AddBlock(piece)
		if err != nil {
			return err
		}

		fmt.Printf("Downloaded piece from %v peers. Left: %v. Goroutines: %v\n", pieceManager.PeerM.PeersCount(), torrent.PiecesCount-pieceManager.Downloaded(), runtime.NumGoroutine())
	}

	return nil
}
