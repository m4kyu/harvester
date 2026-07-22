package harvester

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/m4kyu/harvester/internal/bitfield"
	"github.com/m4kyu/harvester/internal/p2p"
	"github.com/m4kyu/harvester/internal/piecemanager"
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
	client := Harvester{
		Bitfield: make([]byte, (torrent.PiecesCount+8-1)/8),
	}

	pieces := make(chan p2p.Piece, torrent.PiecesCount)

	pieceManager, err := piecemanager.Init(torrent)
	if err != nil {
		return err
	}

	time.Sleep(1 * time.Second)
	fmt.Println("Peers count: ", pieceManager.PeerM.PeersCount())

	go pieceManager.PiecesManager(pieces)

	files, err := client.createDir(torrent)
	if err != nil {
		return err
	}

	for pieceManager.Downloaded() < torrent.PiecesCount {
		piece := <-pieces
		fmt.Printf("Got piece: %v\n", piece.Index)
		pieceManager.NextPiece <- struct{}{}
		begin, _ := torrent.PieceBounds(int(piece.Index))

		total := 0
		for _, file := range files {
			total += file.Size
			if begin >= total {
				continue
			}

			avaliable := file.Size - (begin - total - file.Size)
			if avaliable < piece.PieceSize {
				_, err := file.FD.WriteAt(piece.Data[:avaliable], int64(begin))
				if err != nil {
					return err
				}

				begin = avaliable
				continue
			}

			_, err := file.FD.WriteAt(piece.Data, int64(begin))
			if err != nil {
				return err
			}
			break
		}

		fmt.Printf("Downloaded piece from %v peers. Left: %v. Goroutines: %v\n", pieceManager.PeerM.PeersCount(), torrent.PiecesCount-pieceManager.Downloaded(), runtime.NumGoroutine())
	}

	return nil
}

func (h *Harvester) createDir(torrent torrent.Torrent) ([]File, error) {
	ex, err := os.Executable()
	if err != nil {
		return nil, err
	}

	rootPath := filepath.Dir(ex)
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, err
	}

	var files []File
	if !torrent.IsMultiFile {
		file, err := root.OpenFile(torrent.Info.Name, os.O_CREATE|os.O_RDWR, 0o644)
		if err != nil {
			fmt.Println("ERROR: ", err)
			return nil, err
		}

		err = file.Truncate(int64(torrent.Info.Len))
		if err != nil {
			return nil, err
		}

		files = []File{{FD: file, Size: torrent.Info.Len}}
	} else {
		err = root.Mkdir(torrent.Info.Name, 0o755)
		if err != nil {
			return nil, err
		}

		files = make([]File, len(torrent.Info.Files))
		for _, file := range torrent.Info.Files {
			path := filepath.Join(file.Path...)
			path = filepath.Join(torrent.Info.Name, path)

			dir := filepath.Dir(path)
			err := os.MkdirAll(dir, 0o755)
			if err != nil {
				return nil, err
			}

			fd, err := root.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
			if err != nil {
				return nil, err
			}

			err = fd.Truncate(int64(file.Len))
			if err != nil {
				return nil, err
			}

			files = append(files, File{FD: fd, Size: file.Len})
		}
	}

	return files, nil
}
