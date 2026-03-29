package peer

type Command any

type DownloadPiece struct {
	ID        string
	Index     uint32
	PieceSize uint32

	Hash    [20]byte
	Data    []byte
	EndGame bool
}

type Cancel struct {
	ID    string
	Index uint32
}
