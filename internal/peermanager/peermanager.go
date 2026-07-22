package peermanager

import (
	"fmt"
	"log"
	"strconv"
	"sync"

	"github.com/m4kyu/harvester/internal/message"
	"github.com/m4kyu/harvester/internal/p2p"
	"github.com/m4kyu/harvester/internal/peer"
	"github.com/m4kyu/harvester/internal/torrent"
	"github.com/m4kyu/harvester/internal/tracker"
)

type PeerManager struct {
	peers map[string]*peer.Peer

	m sync.RWMutex

	writeErrC chan error
	readErrC  chan error
	dieC      chan string

	blockC chan message.Block

	notifieC chan string
}

func Init(torrent torrent.Torrent, notifieC chan string, blockC chan message.Block) (*PeerManager, error) {
	peers, err := tracker.PeersList(torrent)
	if err != nil {
		fmt.Println("Couldnt get peers list")
		return nil, err
	}

	manager := PeerManager{}
	manager.writeErrC = make(chan error, 128)
	manager.readErrC = make(chan error, 128)
	manager.dieC = make(chan string, 128)

	manager.notifieC = notifieC
	manager.blockC = blockC
	manager.peers = make(map[string]*peer.Peer)

	for i := range len(peers) {
		pr := &peers[i]
		pr.WriterC = make(chan message.Message, 128)
		pr.MessageC = make(chan message.Message, 128)
		pr.DoneC = make(chan int)

		pr.WriterErrC = manager.writeErrC
		pr.ReadErrC = manager.readErrC
		pr.DieC = manager.dieC

		fmt.Printf("IP: %v. PORT: %v\n", pr.IP, pr.Port)

		pr.ID = pr.IP.String() + ":" + strconv.Itoa(int(pr.Port))

		manager.peers[pr.ID] = pr

		go p2p.HandlePeer(pr, torrent)
		go func(peer *peer.Peer) {
			for {
				select {
				case <-peer.DoneC:
					return
				case msg, ok := <-peer.MessageC:
					if !ok {
						return
					}

					_ = manager.processMsg(peer, msg)
				}
			}
		}(pr)
	}
	go manager.cleanup()

	return &manager, nil
}

func (manager *PeerManager) cleanup() {
	for id := range manager.dieC {
		fmt.Printf("Died: %v\n", id)
		manager.m.Lock()
		delete(manager.peers, id)
		manager.m.Unlock()

		manager.notifieC <- id
	}
}

func (manager *PeerManager) processMsg(peer *peer.Peer, msg message.Message) error {
	switch msg.ID {
	case message.MsgKeepAlive:
		log.Println("Keep alive")
	case message.MsgChoke:
		peer.Choking = true
	case message.MsgUnchoke:
		peer.Choking = false
	case message.MsgBitfield:
		peer.Bitfield = msg.Payload
	case message.MsgPiece:
		block, err := msg.ParsePiece()
		if err != nil {
			return err
		}

		block.PeerID = peer.ID
		manager.blockC <- block
	}

	return nil
}

func (manager *PeerManager) PeersForPiece(index int) []string {
	manager.m.Lock()
	defer manager.m.Unlock()

	res := make([]string, 0)
	for _, p := range manager.peers {
		if p.Choking {
			continue
		}

		if !p.Bitfield.Has(index) {
			continue
		}

		if p.OnGoing >= 32 {
			continue
		}

		res = append(res, p.ID)
	}

	return res
}

func (manager *PeerManager) PeerForPiece(index int) (string, bool) {
	manager.m.Lock()
	defer manager.m.Unlock()

	var best *peer.Peer
	for _, p := range manager.peers {
		if p.Choking {
			continue
		}

		if !p.Bitfield.Has(index) {
			continue
		}

		if p.OnGoing >= 1 {
			continue
		}

		if best == nil {
			best = p
			continue
		}

		if p.OnGoing < best.OnGoing {
			best = p
		}
	}

	if best == nil {
		return "", false
	}

	return best.ID, true
}

func (manager *PeerManager) Exists(id string) bool {
	manager.m.Lock()
	_, ok := manager.peers[id]
	manager.m.Unlock()

	return ok
}

func (manager *PeerManager) CancelBlock(id string, block message.Block) {
	manager.m.Lock()
	p, ok := manager.peers[id]
	if ok {
		p.Cancel(block.Index, block.Begin, block.Len)
	}
	manager.m.Unlock()
}

func (manager *PeerManager) Get(id string) *peer.Peer {
	manager.m.RLock()
	p := manager.peers[id]
	manager.m.RUnlock()
	return p
}

func (manager *PeerManager) Delete(id string) {
	manager.m.Lock()
	delete(manager.peers, id)
	manager.m.Unlock()
}

func (manager *PeerManager) PeersCount() int {
	manager.m.Lock()
	len := len(manager.peers)
	manager.m.Unlock()

	return len
}
