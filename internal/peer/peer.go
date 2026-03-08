package peer

import (
	"encoding/binary"
	"fmt"
	"net"

	"github.com/m4kyu/harvester/internal/bitfield"
	"github.com/m4kyu/harvester/internal/message"
)

type Peer struct {
	IP   net.IP
	Port uint16
	Conn net.Conn

	Chocked   bool // We chocked peer
	Intrested bool // We intrested in peer
	Choking   bool // Peer choking us
	Intrest   bool // Peer wants something from us

	Bitfield bitfield.Bitfield
}

func Unmarshall(bin []byte) ([]Peer, error) {
	const peerSize = 6
	peersCount := len(bin) / peerSize
	if len(bin)%peerSize != 0 {
		return nil, fmt.Errorf("malformed peers blob")
	}

	offset := 0
	peers := make([]Peer, peersCount)
	for i := range peersCount {
		peers[i].IP = net.IP(bin[offset : offset+4])
		peers[i].Port = binary.BigEndian.Uint16(bin[offset+4 : offset+6])
		offset += peerSize
	}

	return peers, nil
}

func (peer *Peer) SendUnchoke() error {
	msg := message.Message{ID: message.MsgUnchoke, Payload: nil}
	_, err := peer.Conn.Write(msg.Serialize())
	if err != nil {
		return err
	}

	peer.Chocked = false
	return nil
}

func (peer *Peer) SendIntrested() error {
	msg := message.Message{ID: message.MsgIntrested, Payload: nil}
	_, err := peer.Conn.Write(msg.Serialize())
	if err != nil {
		return err
	}

	peer.Intrested = true
	return nil
}

func (peer *Peer) Request(index uint32, begin uint32, length uint32) error {
	if length == 0 {
		length = message.DEFAULT_BLOCK_SIZE
	}

	msg := message.Requst(index, begin, length)
	_, err := peer.Conn.Write(msg.Serialize())
	if err != nil {
		return err
	}

	return nil
}

func (peer *Peer) SendHave(index uint32) error {
	msg := message.Have(index)
	_, err := peer.Conn.Write(msg.Serialize())
	if err != nil {
		return err
	}

	return nil
}

func (peer *Peer) KeepAlive() error {
	msg := message.Message{ID: message.MsgKeepAlive, Payload: nil}
	_, err := peer.Conn.Write(msg.Serialize())
	if err != nil {
		return err
	}

	return nil
}
