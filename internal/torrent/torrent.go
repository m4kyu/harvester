package torrent

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io"
	"os"

	bencode "github.com/jackpal/bencode-go"
)

type TFile struct {
	Len  int      `bencode:"length"`
	Path []string `bencode:"path"`
}

type TorrentInfo struct {
	PieceSize int    `bencode:"piece length"`
	Pieces    string `bencode:"pieces"`

	Name string `bencode:"name"`
	Len  int    `bencode:"length"`

	Files []TFile `bencode:"files"`
}

type Torrent struct {
	InfoHash [20]byte

	Announce      string   `bencode:"announce"`
	AnnouncesList []string `bencode:"announce-list"`

	Info        TorrentInfo `bencode:"info"`
	PiecesCount int
	IsMultiFile bool

	Created   int    `bencode:"creation date"`
	CreatedBy string `bencode:"created by"`
	Comment   string `bencode:"comment"`
}

func FromFile(path string) (Torrent, error) {
	file, err := os.Open(path)
	if err != nil {
		return Torrent{}, err
	}
	defer file.Close()

	torrent := Torrent{}
	err = bencode.Unmarshal(file, &torrent)
	if err != nil {
		return Torrent{}, err
	}

	// TODO: Fix tmp solution
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return Torrent{}, err
	}

	torrent.InfoHash, err = infoHash(file)
	if err != nil {
		return Torrent{}, err
	}
	torrent.PiecesCount = len(torrent.Info.Pieces) / 20

	if len(torrent.Info.Files) > 0 {
		torrent.IsMultiFile = true
		torrent.Info.Len = 0
		for _, f := range torrent.Info.Files {
			torrent.Info.Len += f.Len
		}
	}

	return torrent, err
}

func (t *Torrent) PieceBounds(index int) (int, int) {
	begin := index * t.Info.PieceSize
	end := begin + t.Info.PieceSize
	end = min(end, t.Info.Len)

	return begin, end
}

func infoHash(r io.Reader) ([20]byte, error) {
	var m any
	m, err := bencode.Decode(r)
	if err != nil {
		return [20]byte{}, err
	}

	topMap, ok := m.(map[string]any)
	if !ok {
		return [20]byte{}, fmt.Errorf("coldnt map bencode")
	}

	infoMap, ok := topMap["info"]
	if !ok {
		return [20]byte{}, fmt.Errorf("couldnt extract info")
	}

	buffer := bytes.Buffer{}
	err = bencode.Marshal(&buffer, infoMap)
	if err != nil {
		return [20]byte{}, err
	}

	hash := sha1.Sum(buffer.Bytes())
	return hash, nil
}
