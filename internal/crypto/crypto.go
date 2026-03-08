package crypto

import (
	"crypto/sha1"
)

func SHA1(data []byte) [20]byte {
	return sha1.Sum(data)
}
