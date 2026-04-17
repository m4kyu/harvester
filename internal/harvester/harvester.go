package harvester

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/m4kyu/harvester/internal/bitfield"
	"github.com/m4kyu/harvester/internal/crypto"
	"github.com/m4kyu/harvester/internal/message"
	"github.com/m4kyu/harvester/internal/p2p"
	pr "github.com/m4kyu/harvester/internal/peer"
	"github.com/m4kyu/harvester/internal/torrent"
	"github.com/m4kyu/harvester/internal/tracker"
)

type Harvester struct {
	PiecesProgress []PieceProgress
	Bitfield       bitfield.Bitfield
	Peers          map[string]*pr.Peer
	DieChn         chan string
	WriteErrC      chan error
	ReadErrC       chan error

	DoneC chan int

	m          sync.RWMutex
	EndGame    bool
	Downloaded int

	PiecesState []map[string]*p2p.PieceState
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

type File struct {
	FD   *os.File
	Size int
}

func DownloadTorrent(torrent torrent.Torrent) error {
	peers, err := tracker.PeersList(torrent)
	if err != nil {
		fmt.Println("Couldnt get peers list")
		return err
	}

	client := Harvester{
		PiecesProgress: make([]PieceProgress, torrent.PiecesCount),
		Bitfield:       make([]byte, (torrent.PiecesCount+8-1)/8),
		Peers:          make(map[string]*pr.Peer),
		DieChn:         make(chan string, 128),

		WriteErrC: make(chan error, 64),
		ReadErrC:  make(chan error, 64),

		DoneC:       make(chan int),
		PiecesState: make([]map[string]*p2p.PieceState, torrent.PiecesCount),
	}
	for i := range client.PiecesState {
		client.PiecesState[i] = make(map[string]*p2p.PieceState)
	}
	go client.cleanup()

	for i := range len(peers) {
		peer := &peers[i]
		peer.WriterC = make(chan message.Message, 128)
		peer.MessageC = make(chan message.Message, 128)
		peer.DoneC = make(chan int)

		peer.WriterErrC = client.WriteErrC
		peer.ReadErrC = client.ReadErrC
		peer.DieC = client.DieChn

		fmt.Printf("IP: %v. PORT: %v\n", peer.IP, peer.Port)

		peer.ID = peer.IP.String() + ":" + strconv.Itoa(int(peer.Port))

		client.m.Lock()
		client.Peers[peer.ID] = peer
		client.m.Unlock()

		go p2p.HandlePeer(peer, torrent)
		go func(peer *pr.Peer) {
			for {
				select {
				case <-peer.DoneC:
					return
				case msg, ok := <-peer.MessageC:
					if !ok {
						return
					}
					client.processMsg(peer, msg)
				}
			}
		}(peer)
	}

	time.Sleep(1 * time.Second)
	pieces := make(chan p2p.Piece, torrent.PiecesCount)
	go client.piecesManager(torrent, pieces)

	fmt.Println("Peers count: ", len(peers))

	ex, err := os.Executable()
	if err != nil {
		return err
	}

	rootPath := filepath.Dir(ex)
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return err
	}

	var files []File
	if !torrent.IsMultiFile {
		file, err := root.OpenFile(torrent.Info.Name, os.O_CREATE|os.O_RDWR, 0o644)
		if err != nil {
			fmt.Println("ERROR: ", err)
			return err
		}
		defer file.Close()

		err = file.Truncate(int64(torrent.Info.Len))
		if err != nil {
			return err
		}

		files = []File{{FD: file, Size: torrent.Info.Len}}
	} else {
		err = root.Mkdir(torrent.Info.Name, 0o755)
		if err != nil {
			return err
		}

		files = make([]File, len(torrent.Info.Files))
		for _, file := range torrent.Info.Files {
			path := filepath.Join(file.Path...)
			path = filepath.Join(torrent.Info.Name, path)

			dir := filepath.Dir(path)
			err := os.MkdirAll(dir, 0o755)
			if err != nil {
				return err
			}

			fd, err := root.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
			if err != nil {
				return err
			}
			defer fd.Close()

			err = fd.Truncate(int64(file.Len))
			if err != nil {
				return err
			}

			files = append(files, File{FD: fd, Size: file.Len})
		}
	}

	for client.Downloaded < torrent.PiecesCount {
		piece := <-pieces
		begin, _ := torrent.PieceBounds(int(piece.Index))

		total := 0
		for _, file := range files {
			total += file.Size
			if begin >= total {
				continue
			}

			avaliable := file.Size - (begin - total - file.Size)
			if avaliable < piece.PieceSize {
				_, err := file.FD.WriteAt(piece.Data[:avaliable], int64(begin))
				if err != nil {
					return err
				}

				begin = avaliable
				continue
			}

			_, err := file.FD.WriteAt(piece.Data, int64(begin))
			if err != nil {
				return err
			}
			break
		}

		client.m.Lock()
		fmt.Printf("Downloaded piece from %v peers. Left: %v\n", len(client.Peers), torrent.PiecesCount-client.Downloaded)
		client.m.Unlock()
	}

	close(client.DoneC)
	close(client.DieChn)

	return nil
}

func (h *Harvester) piecesManager(torrent torrent.Torrent, res chan p2p.Piece) {
	for {
		select {
		case <-h.DoneC:
			return
		default:
		}

		time.Sleep(5 * time.Microsecond)

		h.m.Lock()
		remaining := torrent.PiecesCount - h.Downloaded
		if !h.EndGame && remaining <= 10 {
			h.EndGame = true
			fmt.Println("\n\n\t\t\tEntering End Game mode!\n")
		}
		h.m.Unlock()

		for i := range torrent.PiecesCount {
			h.m.Lock()
			if h.PiecesProgress[i].Status == Completed {
				h.m.Unlock()
				continue
			}
			h.m.Unlock()

			if !h.EndGame {
				h.orderPiece(torrent, i, res)
			} else {
				h.m.Lock()
				if h.PiecesProgress[i].Status != InEndGame {
					h.m.Unlock()
					h.orderPieceEndGame(torrent, i, res)
					continue
				}
				h.m.Unlock()
			}
		}
	}
}

func (h *Harvester) orderPiece(torrent torrent.Torrent, index int, res chan p2p.Piece) {
	h.m.Lock()
	if h.PiecesProgress[index].Status == InProgress {
		h.m.Unlock()
		return
	}
	h.m.Unlock()

	id, ok := h.peerForPiece(index)
	if !ok {
		return
	}

	pieceSize := torrent.Info.PieceSize
	if index == torrent.PiecesCount-1 {
		pieceSize = torrent.Info.Len - (torrent.Info.PieceSize * index)
	}

	var hash [20]byte
	copy(hash[:], torrent.Info.Pieces[index*20:(index+1)*20])
	piece := p2p.Piece{
		PeerID:    id,
		Index:     index,
		Hash:      hash,
		PieceSize: pieceSize,
	}

	go h.handlePiece(h.Peers[id], piece, res)

	h.m.Lock()
	_, ok = h.Peers[id]
	if !ok {
		h.m.Unlock()
		return
	}

	h.Peers[id].OnGoing++
	h.PiecesProgress[index].Status = InProgress
	h.PiecesProgress[index].PeerID = id
	h.m.Unlock()
}

func (h *Harvester) orderPieceEndGame(torrent torrent.Torrent, index int, res chan p2p.Piece) {
	ids := h.peersForPiece(index)
	if len(ids) == 0 {
		return
	}

	pieceSize := torrent.Info.PieceSize
	if index == torrent.PiecesCount-1 {
		pieceSize = torrent.Info.Len - (torrent.Info.PieceSize * index)
	}

	var hash [20]byte
	copy(hash[:], torrent.Info.Pieces[index*20:(index+1)*20])

	for _, i := range ids {
		piece := p2p.Piece{
			PeerID:    i,
			Index:     index,
			Hash:      hash,
			PieceSize: pieceSize,
		}

		go h.handlePiece(h.Peers[i], piece, res)

		h.m.Lock()
		_, ok := h.Peers[i]
		if !ok {
			h.m.Unlock()
			return
		}

		h.Peers[i].OnGoing++
		h.m.Unlock()
	}

	h.m.Lock()
	h.PiecesProgress[index].Status = InEndGame
	h.m.Unlock()
}

func (h *Harvester) peersForPiece(index int) []string {
	h.m.Lock()
	defer h.m.Unlock()

	res := make([]string, 0)
	for _, p := range h.Peers {
		if p.Choking {
			continue
		}

		if !p.Bitfield.Has(index) {
			continue
		}

		res = append(res, p.ID)
	}

	return res
}

func (h *Harvester) peerForPiece(index int) (string, bool) {
	h.m.Lock()
	defer h.m.Unlock()

	var best *pr.Peer
	for _, p := range h.Peers {
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

func (h *Harvester) handlePiece(peer *pr.Peer, piece p2p.Piece, res chan p2p.Piece) {
	h.downloadPiece(peer, uint32(piece.Index), piece.PieceSize)

	h.m.Lock()
	if h.PiecesProgress[piece.Index].Status == Completed {
		peer.OnGoing--
		h.m.Unlock()
		return
	}
	h.m.Unlock()

	state := h.PiecesState[piece.Index][peer.ID]
	state.Mu.Lock()
	peer.OnGoing--
	pieceHash := crypto.SHA1(state.Buffer)
	state.Mu.Unlock()

	if pieceHash != piece.Hash {
		h.m.Lock()
		h.PiecesProgress[piece.Index].Status = Empty
		h.m.Unlock()

		fmt.Println("Wrong hash")
		return
	} else {
		h.m.Lock()
		h.PiecesProgress[piece.Index].Status = Completed
		h.Downloaded++
		res <- p2p.Piece{
			Index: piece.Index,
			Data:  state.Buffer,
		}

		h.PiecesState[piece.Index] = nil
		h.m.Unlock()

		if h.EndGame {
			fmt.Println("Cancel: ", piece.Index)
			h.cancelPiece(piece.Index, peer.ID)
		}

		if peer.Intrest {
			peer.SendHave(uint32(piece.Index))
		}

		fmt.Printf("Sucssesfuly donwloaded piece #%v From: %v\n", piece.Index, peer.IP)
	}
}

func (h *Harvester) cancelPiece(index int, finishedID string) {
	h.m.Lock()
	defer h.m.Unlock()

	for peerid := range h.PiecesState[index] {
		if peerid == finishedID {
			continue
		}
		_, ok := h.Peers[peerid]
		if !ok {
			continue
		}

		for _, block := range h.PiecesState[index][peerid].Blocks {
			h.Peers[peerid].Cancel(block.Index, block.Begin, block.Len)
		}
	}
}

func (h *Harvester) downloadPiece(peer *pr.Peer, index uint32, pieceSize int) {
	h.m.Lock()
	h.PiecesState[index][peer.ID] = &p2p.PieceState{
		Buffer:    make([]byte, pieceSize),
		Blocks:    make(map[uint32]message.Block),
		Requested: 0,
		Backlog:   0,
		Size:      0,
	}

	state := h.PiecesState[index][peer.ID]
	h.m.Unlock()

	fmt.Printf("Peer: %v. Dowloading: %v\n", peer.ID, index)
	for {
		select {
		case <-peer.DoneC:
			return
		default:
		}

		h.m.Lock()
		if h.PiecesProgress[index].Status == Completed {
			h.m.Unlock()
			return
		}

		done := state.Size >= pieceSize
		h.m.Unlock()

		if done {
			break
		}

		state.Mu.Lock()
		if !peer.Choking {
			for state.Backlog < 5 && state.Requested < pieceSize {
				blockSize := message.DEFAULT_BLOCK_SIZE
				if state.Requested+message.DEFAULT_BLOCK_SIZE > pieceSize {
					blockSize = pieceSize - state.Requested
				}

				peer.Request(index, uint32(state.Requested), uint32(blockSize))

				state.Backlog++
				state.Requested += blockSize
				state.Blocks[uint32(state.Requested)] = message.Block{Index: index, Begin: uint32(state.Requested), Len: uint32(blockSize)}
			}
		}

		state.Mu.Unlock()
		time.Sleep(2 * time.Microsecond)
	}
}

func (h *Harvester) handleBlock(block message.Block) {
	h.m.Lock()
	state, ok := h.PiecesState[block.Index][block.PeerID]
	if !ok {
		h.m.Unlock()
		return
	}
	h.m.Unlock()

	state.Mu.Lock()

	copy(state.Buffer[block.Begin:], block.Data)
	state.Size += len(block.Data)
	if state.Backlog > 0 {
		state.Backlog--
	}

	delete(state.Blocks, block.Begin)
	state.Mu.Unlock()
}

func (h *Harvester) processMsg(peer *pr.Peer, msg message.Message) error {
	switch msg.ID {
	case message.MsgKeepAlive:
		log.Println("\n\nKeep alive\n\n")
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
		h.handleBlock(block)
	}

	return nil
}

func (h *Harvester) cleanup() {
	for id := range h.DieChn {
		h.m.Lock()
		delete(h.Peers, id)
		for i := range h.PiecesProgress {
			if h.PiecesProgress[i].PeerID == id && h.PiecesProgress[i].Status == InProgress {
				h.PiecesProgress[i].Status = Empty
			}
		}
		h.m.Unlock()
	}
}

func prepearWorkChan(queue chan p2p.Piece, torrent torrent.Torrent) {
	for i := range torrent.PiecesCount {
		var hash [20]byte
		copy(hash[:], torrent.Info.Pieces[i*20:(i+1)*20])
		queue <- p2p.Piece{Index: i, Hash: hash}
	}
}
