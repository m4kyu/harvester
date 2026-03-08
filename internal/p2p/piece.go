package p2p

type Piece struct {
	Index int
	Hash  [20]byte

	Data []byte
}
