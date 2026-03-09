package p2p

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/m4kyu/harvester/internal/client"
	"github.com/m4kyu/harvester/internal/crypto"
	"github.com/m4kyu/harvester/internal/message"
	pr "github.com/m4kyu/harvester/internal/peer"
	tr "github.com/m4kyu/harvester/internal/torrent"
)

type PieceState struct {
	Peer      *pr.Peer
	Buffer    []byte
	Size      int
	Requested int
	Backlog   int
}

type Block struct {
	Begin uint32
	Size  uint32
}

func HandlePeer(peer pr.Peer, torrent tr.Torrent, wg *sync.WaitGroup, workQueue chan Piece, resQueue chan Piece) {
	defer wg.Done()

	var clientID [20]byte
	copy(clientID[:], "testtttttttttttt322t")
	client := client.Client{ID: clientID}
	err := client.CompleteHandshake(&peer, torrent)
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	defer peer.Conn.Close()

	fmt.Println("Sucesful handshake with: ", peer.IP)

	err = reveiveBitField(&peer)
	if err != nil {
		return
	}

	peer.SendUnchoke()
	peer.SendIntrested()
	peer.KeepAlive()

	fmt.Println("Sucssesful intrested: ", peer.IP)

	for {
		var piece Piece
		var ok bool

		select {
		case piece, ok = <-workQueue:
			if !ok {
				peer.Conn.Close()
				return
			}
		default:
			peer.KeepAlive()
			runtime.Gosched()
			continue
		}

		if !peer.Bitfield.HasPiece(piece.Index) {
			workQueue <- piece
			continue
		}

		data, err := downloadPiece(&peer, piece.Index, torrent.Info.PieceSize)
		if err != nil {
			fmt.Printf("Error downloading: %v. From: %v\n", err, peer.IP)
			workQueue <- piece
			break
		}

		pieceHash := crypto.SHA1(data)
		if pieceHash != piece.Hash {
			workQueue <- piece
			fmt.Println("Wrong hash")
			break
		} else {
			fmt.Printf("Sucssesfuly donwloaded piece #%v From: %v\n", piece.Index, peer.IP)
			piece.Data = data
			resQueue <- piece
		}
	}
}

func downloadPiece(peer *pr.Peer, index int, pieceSize int) ([]byte, error) {
	state := PieceState{Peer: peer, Buffer: make([]byte, pieceSize), Size: 0, Backlog: 0}

	//	peer.Conn.SetDeadline(time.Now().Add(30 * time.Second))
	//	defer peer.Conn.SetDeadline(time.Time{}) // Disable the deadline

	for state.Size < pieceSize {
		if !peer.Choking {
			for state.Backlog < 5 && state.Requested < pieceSize {
				blockSize := message.DEFAULT_BLOCK_SIZE
				if state.Requested+message.DEFAULT_BLOCK_SIZE > pieceSize {
					blockSize = state.Requested + message.DEFAULT_BLOCK_SIZE - pieceSize
				}

				peer.Request(uint32(index), uint32(state.Requested), uint32(blockSize))
				//				fmt.Printf("Request: %v. Begin: %v. Len: %v. Expected: %v. Piece Size: %v. From: %v\n", index, state.Requested, blockSize, state.Requested+blockSize, pieceSize, peer.IP)

				state.Backlog++
				state.Requested += blockSize
			}
		}

		err := state.processMsg()
		if err != nil {
			return nil, err
		}
	}

	return state.Buffer, nil
}

func (state *PieceState) processMsg() error {
	state.Peer.Conn.SetReadDeadline(time.Now().Add(20 * time.Second))
	defer state.Peer.Conn.SetDeadline(time.Time{})

	msg, err := message.Read(state.Peer.Conn)
	if err != nil {
		return err
	}

	switch msg.ID {
	case message.MsgChoke:
		state.Peer.Choking = true
	case message.MsgUnchoke:
		state.Peer.Choking = false
	case message.MsgPiece:
		block, err := msg.ParsePiece()
		if err != nil {
			return err
		}

		copy(state.Buffer[block.Begin:], block.Data)
		state.Size += len(block.Data)
		state.Backlog--
		//		fmt.Printf("Received: %v. Size: %v. From: %v\n", block.Begin, state.Size, state.Peer.IP)
	}

	return nil
}

func reveiveBitField(peer *pr.Peer) error {
	peer.Conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	defer peer.Conn.SetDeadline(time.Time{})

	msg, err := message.Read(peer.Conn)
	if err != nil {
		return err
	}

	if msg.ID != message.MsgBitfield {
		return fmt.Errorf("expected bitfield got: %v", msg.ID)
	}

	peer.Bitfield = msg.Payload
	return nil
}
