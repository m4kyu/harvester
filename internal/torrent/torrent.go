package torrent

import (
	"bytes"
	"crypto/sha1"
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

	//  Files []TFile `bencode:"files"`
}

type Torrent struct {
	InfoHash [20]byte

	Announce      string   `bencode:"announce"`
	AnnouncesList []string `bencode:"announce-list"`

	Info        TorrentInfo `bencode:"info"`
	PiecesCount int

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
	torrent.InfoHash, err = infoHash(torrent.Info)
	torrent.PiecesCount = len(torrent.Info.Pieces) / 20

	return torrent, err
}

func infoHash(info TorrentInfo) ([20]byte, error) {
	buffer := bytes.Buffer{}
	err := bencode.Marshal(&buffer, info)
	if err != nil {
		return [20]byte{}, err
	}

	hash := sha1.Sum(buffer.Bytes())
	return hash, nil
}
