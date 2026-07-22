package piecemanager

import (
	"fmt"
	"sync"

	"github.com/m4kyu/harvester/internal/crypto"
	"github.com/m4kyu/harvester/internal/message"
	"github.com/m4kyu/harvester/internal/p2p"
	"github.com/m4kyu/harvester/internal/peer"
	"github.com/m4kyu/harvester/internal/peermanager"
	"github.com/m4kyu/harvester/internal/torrent"
)

type PieceManager struct {
	PeerM *peermanager.PeerManager

	torrent torrent.Torrent

	m              sync.RWMutex
	PiecesState    []map[string]*p2p.PieceState
	PiecesProgress []PieceProgress

	downloaded int
	endGame    bool

	NextPiece chan any // Indicates that new piece was received
	DoneC     chan int
	notifieC  chan string
	blockC    chan message.Block
}

type Status int

const (
	Empty Status = iota
	Completed
	InProgress
	InEndGame
)

type PieceProgress struct {
	PeerID string
	Status Status
}

func Init(torrent torrent.Torrent) (*PieceManager, error) {
	notifieC := make(chan string, 128)
	blockC := make(chan message.Block, 128)
	peerM, err := peermanager.Init(torrent, notifieC, blockC)
	if err != nil {
		return nil, err
	}

	pm := &PieceManager{
		PeerM:          peerM,
		PiecesProgress: make([]PieceProgress, torrent.PiecesCount),
		torrent:        torrent,

		NextPiece: make(chan any, 512),

		DoneC:       make(chan int),
		PiecesState: make([]map[string]*p2p.PieceState, torrent.PiecesCount),
		notifieC:    notifieC,
		blockC:      blockC,
	}

	for i := range pm.PiecesState {
		pm.PiecesState[i] = make(map[string]*p2p.PieceState)
	}

	return pm, nil
}

func (pm *PieceManager) PiecesManager(res chan p2p.Piece) {
	go pm.cleanup()
	go pm.handleBlocks()

	pm.NextPiece <- struct{}{}
	for {
		select {
		case <-pm.DoneC:
			return
		case <-pm.NextPiece:
			fmt.Println("Next piece")
			pm.m.Lock()
			remaining := pm.torrent.PiecesCount - pm.downloaded
			if !pm.endGame && remaining <= 10 {
				pm.endGame = true
				fmt.Println("Entering End Game mode!")
			}
			pm.m.Unlock()

			for i := range pm.torrent.PiecesCount {
				pm.m.Lock()
				if pm.PiecesProgress[i].Status == Completed {
					pm.m.Unlock()
					continue
				}
				pm.m.Unlock()

				if !pm.endGame {
					pm.orderPiece(i, res)
				} else {
					pm.m.Lock()
					if pm.PiecesProgress[i].Status != InEndGame {
						pm.m.Unlock()
						pm.orderPieceEndGame(i, res)
						continue
					}
					pm.m.Unlock()
				}
			}
		}
	}
}

func (pm *PieceManager) orderPieceEndGame(index int, res chan p2p.Piece) {
	ids := pm.PeerM.PeersForPiece(index)
	if len(ids) == 0 {
		return
	}

	// Claim the piece before starting any workers.  Otherwise a worker can
	// finish (or a second scheduling pass can run) while the piece is still
	// marked as available.
	pm.m.Lock()
	if pm.PiecesProgress[index].Status == Completed || pm.PiecesProgress[index].Status == InEndGame {
		pm.m.Unlock()
		return
	}
	pm.PiecesProgress[index].Status = InEndGame
	// Reserve a slot before launching goroutines.  A peer can disconnect
	// immediately after scheduling; cleanup must still be able to see that
	// assignment and release the piece.
	for _, id := range ids {
		pm.PiecesState[index][id] = nil
	}
	pm.m.Unlock()

	pieceSize := pm.torrent.Info.PieceSize
	if index == pm.torrent.PiecesCount-1 {
		pieceSize = pm.torrent.Info.Len - (pm.torrent.Info.PieceSize * index)
	}

	var hash [20]byte
	copy(hash[:], pm.torrent.Info.Pieces[index*20:(index+1)*20])

	for _, i := range ids {
		p := pm.PeerM.Get(i)
		if p == nil {
			continue
		}
		piece := p2p.Piece{
			PeerID:    i,
			Index:     index,
			Hash:      hash,
			PieceSize: pieceSize,
		}

		p.OnGoing++
		go pm.handlePiece(p, piece, res)
	}
}

func (pm *PieceManager) orderPiece(index int, res chan p2p.Piece) {
	pm.m.Lock()
	if pm.PiecesProgress[index].Status == InProgress {
		pm.m.Unlock()
		return
	}
	pm.m.Unlock()

	id, ok := pm.PeerM.PeerForPiece(index)
	if !ok {
		return
	}
	p := pm.PeerM.Get(id)
	if p == nil {
		return
	}

	fmt.Printf("Peer for piece: %v\n", id)
	pieceSize := pm.torrent.Info.PieceSize
	if index == pm.torrent.PiecesCount-1 {
		pieceSize = pm.torrent.Info.Len - (pm.torrent.Info.PieceSize * index)
	}

	var hash [20]byte
	copy(hash[:], pm.torrent.Info.Pieces[index*20:(index+1)*20])
	piece := p2p.Piece{
		PeerID:    id,
		Index:     index,
		Hash:      hash,
		PieceSize: pieceSize,
	}

	pm.m.Lock()
	p.OnGoing++
	fmt.Printf("Ongoin in %v peer is %v\n", id, pm.PeerM.Get(id).OnGoing)
	pm.PiecesProgress[index].Status = InProgress
	pm.PiecesProgress[index].PeerID = id
	pm.m.Unlock()

	go pm.handlePiece(p, piece, res)
}

func (pm *PieceManager) handlePiece(peer *peer.Peer, piece p2p.Piece, res chan p2p.Piece) {
	pm.downloadPiece(peer, uint32(piece.Index), piece.PieceSize)

	pm.m.Lock()
	if pm.PiecesProgress[piece.Index].Status == Completed {
		peer.OnGoing--
		pm.m.Unlock()
		return
	}
	pm.m.Unlock()

	pm.m.RLock()
	state := pm.PiecesState[piece.Index][peer.ID]
	pm.m.RUnlock()
	if state == nil {
		peer.OnGoing--
		return
	}

	state.Mu.Lock()
	peer.OnGoing--
	pieceHash := crypto.SHA1(state.Buffer)
	state.Mu.Unlock()

	if pieceHash != piece.Hash {
		pm.m.Lock()
		if pm.PiecesProgress[piece.Index].Status != Completed {
			pm.PiecesProgress[piece.Index].Status = Empty
		}
		pm.m.Unlock()
		select {
		case pm.NextPiece <- struct{}{}:
		default:
		}

		fmt.Println("Wrong hash")
		return
	} else {
		pm.m.Lock()
		// Several peers may have been downloading this piece in endgame.
		// Only the first valid response is allowed to complete it.
		if pm.PiecesProgress[piece.Index].Status == Completed {
			pm.m.Unlock()
			return
		}
		pm.PiecesProgress[piece.Index].Status = Completed
		pm.downloaded++
		res <- p2p.Piece{
			Index: piece.Index,
			Data:  state.Buffer,
		}

		pm.PiecesState[piece.Index] = nil
		pm.m.Unlock()

		if pm.endGame {
			fmt.Println("\nCancel: ", piece.Index)
			pm.cancelPiece(piece.Index, peer.ID)
		}

		if peer.Intrest {
			peer.SendHave(uint32(piece.Index))
		}

		fmt.Printf("Sucssesfuly donwloaded piece #%v From: %v\n", piece.Index, peer.IP)
	}
}

func (pm *PieceManager) cancelPiece(index int, finishedID string) {
	pm.m.Lock()
	defer pm.m.Unlock()

	for peerid := range pm.PiecesState[index] {
		if peerid == finishedID {
			continue
		}
		if !pm.PeerM.Exists(peerid) {
			continue
		}

		state := pm.PiecesState[index][peerid]
		if state == nil {
			continue
		}
		for _, block := range state.Blocks {
			pm.PeerM.CancelBlock(peerid, block)
		}
	}
}

func (pm *PieceManager) downloadPiece(peer *peer.Peer, index uint32, pieceSize int) {
	if peer == nil {
		return
	}
	pm.m.Lock()
	if pm.PiecesState[index] == nil {
		pm.m.Unlock()
		return
	}

	pm.PiecesState[index][peer.ID] = &p2p.PieceState{
		Buffer:    make([]byte, pieceSize),
		Blocks:    make(map[uint32]message.Block),
		Requested: 0,
		Backlog:   0,
		Size:      0,
		New:       make(chan any, 64),
	}

	state := pm.PiecesState[index][peer.ID]
	pm.m.Unlock()

	state.New <- struct{}{}
	fmt.Printf("Peer: %v. Dowloading: %v\n", peer.ID, index)
	for {
		select {
		case <-peer.DoneC:
			return
		case <-state.New:
			pm.m.Lock()
			if pm.PiecesProgress[index].Status == Completed {
				pm.m.Unlock()
				return
			}
			pm.m.Unlock()

			state.Mu.Lock()
			if state.Size >= pieceSize {
				state.Mu.Unlock()
				return
			}
			if !peer.Choking {
				for state.Backlog < 5 && state.Requested < pieceSize {
					blockSize := message.DEFAULT_BLOCK_SIZE
					if state.Requested+message.DEFAULT_BLOCK_SIZE > pieceSize {
						blockSize = pieceSize - state.Requested
					}

					begin := uint32(state.Requested)
					state.Blocks[begin] = message.Block{Index: index, Begin: begin, Len: uint32(blockSize)}
					state.Backlog++
					state.Requested += blockSize

					// Record the request before sending it. A fast peer can return
					// the block immediately, and handleBlocks must already be able
					// to recognize that response.
					peer.Request(index, begin, uint32(blockSize))
				}
			}
			state.Mu.Unlock()
		}
	}
}

func (pm *PieceManager) handleBlocks() {
	for block := range pm.blockC {
		pm.m.Lock()
		if int(block.Index) >= len(pm.PiecesState) {
			pm.m.Unlock()
			continue
		}
		state, ok := pm.PiecesState[block.Index][block.PeerID]
		if !ok || state == nil {
			pm.m.Unlock()
			// A duplicate endgame response is expected after another peer
			// completed the piece.  It must not stop handling future blocks.
			continue
		}
		pm.m.Unlock()

		state.Mu.Lock()
		if _, requested := state.Blocks[block.Begin]; !requested {
			state.Mu.Unlock()
			continue
		}

		end := int(block.Begin) + len(block.Data)
		if int(block.Begin) < 0 || end > len(state.Buffer) {
			state.Mu.Unlock()
			continue
		}
		copy(state.Buffer[block.Begin:end], block.Data)
		state.Size += len(block.Data)
		if state.Backlog > 0 {
			state.Backlog--
		}
		delete(state.Blocks, block.Begin)
		state.Mu.Unlock()

		state.New <- struct{}{}
	}
}

func (pm *PieceManager) cleanup() {
	for id := range pm.notifieC {
		pm.m.Lock()
		changed := false
		for i := range pm.PiecesProgress {
			if pm.PiecesProgress[i].PeerID == id && pm.PiecesProgress[i].Status == InProgress {
				pm.PiecesProgress[i].Status = Empty
				pm.PiecesProgress[i].PeerID = ""
				changed = true
			}

			// Endgame assignments are made to several peers and therefore do
			// not have a single PeerID.  Remove the dead peer's state and make
			// the piece retryable when it has no surviving assignment.
			if _, ok := pm.PiecesState[i][id]; ok {
				delete(pm.PiecesState[i], id)
				changed = true
				if pm.PiecesProgress[i].Status == InEndGame && len(pm.PiecesState[i]) == 0 {
					pm.PiecesProgress[i].Status = Empty
				}
			}
		}
		pm.m.Unlock()
		if changed {
			select {
			case pm.NextPiece <- struct{}{}:
			default:
			}
		}
	}
}

func (pm *PieceManager) Downloaded() int {
	pm.m.Lock()
	val := pm.downloaded
	pm.m.Unlock()

	return val
}
