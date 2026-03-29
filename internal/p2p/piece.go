package p2p

type Piece struct {
	PeerID    string
	Index     int
	Hash      [20]byte
	PieceSize int

	Data []byte
}
