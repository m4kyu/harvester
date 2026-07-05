package peer

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/m4kyu/harvester/internal/bitfield"
	"github.com/m4kyu/harvester/internal/message"
)

type Peer struct {
	ID   string
	IP   net.IP
	Port uint16
	Conn net.Conn

	Chocked   bool // We chocked peer
	Intrested bool // We intrested in peer
	Choking   bool // Peer choking us
	Intrest   bool // Peer wants something from us

	Bitfield bitfield.Bitfield

	DieC     chan string // Tells manager that peer has died
	MessageC chan message.Message

	WriterC    chan message.Message
	WriterErrC chan error

	DoneC    chan int
	ReadErrC chan error

	OnGoing int
	once    sync.Once

	lastReceive atomic.Int64
	lastSent    atomic.Int64
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

func (peer *Peer) SendUnchoke() {
	msg := message.Message{ID: message.MsgUnchoke, Payload: nil}
	peer.WriterC <- msg

	peer.Chocked = false
}

func (peer *Peer) SendIntrested() {
	msg := message.Message{ID: message.MsgIntrested, Payload: nil}
	peer.WriterC <- msg

	peer.Intrested = true
}

func (peer *Peer) Request(index uint32, begin uint32, length uint32) {
	if length == 0 {
		length = message.DEFAULT_BLOCK_SIZE
	}

	msg := message.Request(index, begin, length)
	peer.WriterC <- *msg
}

func (peer *Peer) Cancel(index uint32, begin uint32, length uint32) {
	if length == 0 {
		length = message.DEFAULT_BLOCK_SIZE
	}

	msg := message.Request(index, begin, length)
	msg.ID = message.MsgCancel
	peer.WriterC <- *msg
}

func (peer *Peer) SendHave(index uint32) {
	msg := message.Have(index)
	peer.WriterC <- *msg
}

func (peer *Peer) KeepAlive() {
	msg := message.Message{ID: message.MsgKeepAlive, Payload: nil}
	peer.WriterC <- msg
}

func (peer *Peer) Close() {
	peer.once.Do(func() {
		close(peer.DoneC)
		peer.Conn.Close()
	})
}

func (peer *Peer) touch() {
	peer.lastReceive.Store(time.Now().UnixNano())
}

func (peer *Peer) sent() {
	peer.lastSent.Store(time.Now().UnixNano())
}

func (peer *Peer) lastSeen() time.Time {
	return time.Unix(0, peer.lastReceive.Load())
}

func (peer *Peer) lastInteract() time.Time {
	return time.Unix(0, peer.lastSent.Load())
}
