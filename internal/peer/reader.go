package peer

import (
	"log"
	"time"

	"github.com/m4kyu/harvester/internal/message"
)

func (peer *Peer) ReadLoop() {
	for {
		select {
		case <-peer.DoneC:
			return
		default:
		}

		peer.Conn.SetReadDeadline(time.Now().Add(10 * time.Minute))
		defer peer.Conn.SetReadDeadline(time.Time{})

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
