package protocol

import (
	"net"

	"github.com/m4kyu/harvester/internal/message"
)

func SendMessage(msg message.Message, conn net.Conn) error {
	_, err := conn.Write(msg.Serialize())
	return err
}
