package client

import (
	"bytes"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/m4kyu/harvester/internal/handshake"
	pr "github.com/m4kyu/harvester/internal/peer"
	tr "github.com/m4kyu/harvester/internal/torrent"
)

type Client struct {
	ID [20]byte
}

func (cl *Client) CompleteHandshake(peer *pr.Peer, torrent tr.Torrent) error {
	var err error
	peer.Conn, err = net.DialTimeout("tcp", peer.IP.String()+":"+strconv.Itoa(int(peer.Port)), 5*time.Second)
	if err != nil {
		return err
	}

	payload := handshake.New(torrent.InfoHash, cl.ID)
	_, err = peer.Conn.Write(payload.Serialize())
	if err != nil {
		return err
	}

	buffer, err := handshake.Read(peer.Conn)
	if err != nil {
		return fmt.Errorf("cant read handshake: %v", err.Error())
	}
	if !verifyHandshake(*buffer, torrent.InfoHash) {
		return fmt.Errorf("bad handshake")
	}

	return nil
}

func verifyHandshake(buffer [68]byte, hash [20]byte) bool {
	if buffer[0] != 0x13 {
		return false
	}
	if string(buffer[1:20]) != "BitTorrent protocol" {
		return false
	}
	if !bytes.Equal(hash[:], buffer[28:48]) {
		return false
	}

	return true
}
