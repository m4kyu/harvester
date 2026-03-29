package bitfield

type Bitfield []byte

func (bf Bitfield) Has(index int) bool {
	byteIndex := index / 8
	offset := index % 8
	if byteIndex < 0 || byteIndex >= len(bf) {
		return false
	}
	return bf[byteIndex]>>uint(7-offset)&1 != 0
}

func (bf Bitfield) Set(index int) {
	byteIdx := index / 8
	bitIdx := 7 - (index % 8)
	bf[byteIdx] |= (1 << bitIdx)
}
