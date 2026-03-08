package message

import (
	"encoding/binary"
	"fmt"
	"io"
)

type MessageID byte

const (
	MsgChoke MessageID = iota
	MsgUnchoke
	MsgIntrested
	MsgUnintrested
	MsgHave
	MsgBitfield
	MsgRequest
	MsgPiece
	MsgCancel
	MsgPort

	MsgKeepAlive
)

const DEFAULT_BLOCK_SIZE = 16384

type Message struct {
	ID      MessageID
	Payload []byte
}

type Block struct {
	Begin uint32
	Len   uint32
	Data  []byte
}

func Read(r io.Reader) (*Message, error) {
	tmp := make([]byte, 4)
	_, err := io.ReadFull(r, tmp)
	if err != nil {
		return nil, fmt.Errorf("msg: %v", err)
	}

	length := binary.BigEndian.Uint32(tmp[:])
	if length == 0 {
		return &Message{MsgKeepAlive, []byte{}}, nil
	}

	buffer := make([]byte, length)
	_, err = io.ReadFull(r, buffer)
	if err != nil {
		return nil, err
	}

	if buffer[0] > byte(MsgPort) {
		return nil, fmt.Errorf("invalid message id: %v", buffer[0])
	}

	return &Message{MessageID(buffer[0]), buffer[1:]}, nil
}

func (msg *Message) Serialize() []byte {
	if msg.ID == MsgKeepAlive {
		buffer := [4]byte{0, 0, 0, 0}
		return buffer[:]
	}

	length := uint32(len(msg.Payload) + 1)

	buffer := make([]byte, length+4)
	binary.BigEndian.PutUint32(buffer[0:4], length)
	buffer[4] = byte(msg.ID)
	copy(buffer[5:], msg.Payload)

	return buffer
}

func Requst(index uint32, begin uint32, length uint32) *Message {
	buffer := make([]byte, 12)
	binary.BigEndian.PutUint32(buffer[:4], index)
	binary.BigEndian.PutUint32(buffer[4:8], begin)   // Offset
	binary.BigEndian.PutUint32(buffer[8:12], length) // Piece len
	// binary.BigEndian.PutUint32(buffer[8:12], 262144)

	msg := Message{
		ID:      MsgRequest,
		Payload: buffer,
	}

	return &msg
}

func Have(index uint32) *Message {
	buffer := make([]byte, 4)
	binary.BigEndian.PutUint32(buffer[:4], index)

	msg := Message{
		ID:      MsgRequest,
		Payload: buffer,
	}

	return &msg
}

func (msg *Message) ParsePiece() (Block, error) {
	if msg.ID != MsgPiece {
		return Block{}, fmt.Errorf("expected piece id")
	}

	pieceSize := len(msg.Payload) - 8
	if pieceSize <= 0 {
		return Block{}, fmt.Errorf("invalid size")
	}

	var piece Block
	piece.Len = uint32(pieceSize)
	piece.Begin = binary.BigEndian.Uint32(msg.Payload[4:8])
	piece.Data = msg.Payload[8:]
	return piece, nil
}
