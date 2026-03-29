package peer

import (
	"log"
	"time"
)

func (peer *Peer) WriteLoop() {
	for {
		select {
		case <-peer.DoneC:
			return
		default:
		}

		msg := <-peer.WriterC

		peer.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		defer peer.Conn.SetWriteDeadline(time.Time{})

		//	log.Printf("Peer: %v. MSG: %v\n", peer.ID, msg.ID)
		_, err := peer.Conn.Write(msg.Serialize())
		if err != nil {
			log.Printf("\n\nWrite Err: %v. ID: %v\n", err.Error(), peer.ID)
			peer.WriterErrC <- err
			peer.Close()
			return
		}

		peer.touch()
	}
}
