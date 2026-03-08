package tracker

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/jackpal/bencode-go"
	"github.com/m4kyu/harvester/internal/peer"
	tr "github.com/m4kyu/harvester/internal/torrent"
)

type trackerResponse struct {
	Interval int    `bencode:"interval"`
	Peers    string `bencode:"peers"`
}

func PeersList(torrent tr.Torrent) ([]peer.Peer, error) {
	url, err := constructRequest(torrent)
	if err != nil {
		return nil, err
	}

	fmt.Println(url)
	c := &http.Client{Timeout: 15 * time.Second}
	resp, err := c.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	trResponse := trackerResponse{}
	err = bencode.Unmarshal(resp.Body, &trResponse)
	if err != nil {
		return nil, err
	}

	return peer.Unmarshall([]byte(trResponse.Peers))
}

func constructRequest(torrent tr.Torrent) (string, error) {
	base, err := url.Parse(torrent.Announce)
	if err != nil {
		return "", err
	}

	params := url.Values{
		"info_hash":  []string{string(torrent.InfoHash[:])},
		"peer_id":    []string{"hfc3xster11111111111"},
		"port":       []string{strconv.Itoa(6969)},
		"uploaded":   []string{"0"},
		"downloaded": []string{"0"},
		"compact":    []string{"1"},
		"left":       []string{strconv.Itoa(torrent.Info.Len)},
		"numwant":    []string{"200"},
	}

	base.RawQuery = params.Encode()
	return base.String(), nil
}
