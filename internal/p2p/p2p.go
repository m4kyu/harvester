package p2p

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/m4kyu/harvester/internal/client"
	"github.com/m4kyu/harvester/internal/message"
	pr "github.com/m4kyu/harvester/internal/peer"
	tr "github.com/m4kyu/harvester/internal/torrent"
)

type PieceState struct {
	Buffer    []byte
	Size      int
	Requested int
	Backlog   int

	Blocks map[uint32]message.Block
	New    chan any

	Mu sync.Mutex
}

func HandlePeer(peer *pr.Peer, torrent tr.Torrent) {
	defer func() {
		fmt.Println("DIe: ", peer.ID)
		peer.DieC <- peer.ID
		time.Sleep(10 * time.Microsecond)
	}()

	var clientID [20]byte
	copy(clientID[:], "testtttttttttttt322t")
	client := client.Client{ID: clientID}
	err := client.CompleteHandshake(peer, torrent)
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	defer peer.Conn.Close()

	fmt.Println("Sucesful handshake with: ", peer.IP)

	go peer.ReadLoop()
	go peer.WriteLoop()
	go peer.Monitor()

	peer.SendUnchoke()
	peer.SendIntrested()
	peer.KeepAlive()

	fmt.Println("Sucssesful intrested: ", peer.IP)

	<-peer.DoneC
	log.Printf("Peer: %v. DoneC\n", peer.ID)
}
