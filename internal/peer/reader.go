package peer

import (
	"log"

	"github.com/m4kyu/harvester/internal/message"
)

func (peer *Peer) ReadLoop() {
	for {
		select {
		case <-peer.DoneC:
			return
		default:
		}

		msg, err := message.Read(peer.Conn)
		if err != nil {
			log.Printf("\n\nRead Err: %v. ID: %v\n", err.Error(), peer.ID)
			peer.ReadErrC <- err
			peer.Close()
			return
		}

		peer.MessageC <- *msg
		peer.touch()
	}
}
