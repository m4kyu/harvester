package handshake

import (
	"io"
	"net"
	"time"
)

type Handshake struct {
	Pstr     string
	InfoHash [20]byte
	PeerID   [20]byte
}

func New(hash [20]byte, peerid [20]byte) *Handshake {
	handshake := Handshake{
		"BitTorrent protocol",
		hash,
		peerid,
	}

	return &handshake
}

func (handshake *Handshake) Serialize() []byte {
	buffer := make([]byte, len(handshake.Pstr)+49)
	buffer[0] = byte(len(handshake.Pstr[:]))

	copy(buffer[1:], handshake.Pstr[:])
	copy(buffer[28:], handshake.InfoHash[:])
	copy(buffer[48:], "harvester11111111111")

	return buffer
}

func Read(conn net.Conn) (*[68]byte, error) {
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	defer conn.SetDeadline(time.Time{})

	var buffer [68]byte
	_, err := io.ReadFull(conn, buffer[:])
	if err != nil {
		return nil, err
	}

	return &buffer, nil
}
