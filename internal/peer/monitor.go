package peer

import (
	"fmt"
	"time"
)

func (peer *Peer) Monitor() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	interval := time.Duration(2) * time.Minute
	for {
		select {
		case <-peer.DoneC:
			return
		case <-ticker.C:
			last := peer.lastSeen()

			if time.Since(last) >= interval {
				fmt.Println("\nDied from inactivity\n\n")
				peer.Close()
				return
			}

			last = peer.lastInteract()
			if time.Since(last) >= interval {
				peer.KeepAlive()
				peer.sent()
			}
		}
	}
}
