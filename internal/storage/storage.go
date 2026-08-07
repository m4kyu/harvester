package storage

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/m4kyu/harvester/internal/p2p"
	"github.com/m4kyu/harvester/internal/torrent"
)

type Storage struct {
	t     torrent.Torrent
	Files []File
}

type File struct {
	FD   *os.File
	Size int
}

func Init(t torrent.Torrent) (Storage, error) {
	ex, err := os.Executable()
	if err != nil {
		return Storage{}, err
	}

	rootPath := filepath.Dir(ex)

	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return Storage{}, err
	}

	s := Storage{}
	s.t = t
	if !t.IsMultiFile {
		file, err := root.OpenFile(t.Info.Name, os.O_CREATE|os.O_RDWR, 0o644)
		if err != nil {
			fmt.Println("ERROR: ", err)
			return Storage{}, err
		}

		err = file.Truncate(int64(t.Info.Len))
		if err != nil {
			file.Close()
			return Storage{}, err
		}

		s.Files = []File{{FD: file, Size: t.Info.Len}}
	} else {
		err = root.Mkdir(t.Info.Name, 0o755)
		if err != nil {
			return Storage{}, err
		}

		s.Files = make([]File, len(t.Info.Files))
		for _, file := range t.Info.Files {
			path := filepath.Join(file.Path...)
			path = filepath.Join(t.Info.Name, path)

			dir := filepath.Dir(path)
			err := os.MkdirAll(dir, 0o755)
			if err != nil {
				s.Finish()
				return Storage{}, err
			}

			fd, err := root.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
			if err != nil {
				s.Finish()
				return Storage{}, err
			}
			defer fd.Close()

			err = fd.Truncate(int64(file.Len))
			if err != nil {
				s.Finish()
				return Storage{}, err
			}

			s.Files = append(s.Files, File{FD: fd, Size: file.Len})
		}
	}

	return s, nil
}

func (s *Storage) AddBlock(piece p2p.Piece) error {
	begin, _ := s.t.PieceBounds(int(piece.Index))

	total := 0
	for _, file := range s.Files {
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

	return nil
}

func (s *Storage) Finish() {
	for _, file := range s.Files {
		file.FD.Close()
	}
}
