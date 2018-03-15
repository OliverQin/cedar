package bundle

import (
	"crypto/sha512"
)

type kdf interface {
	generate(masterPhrase string, salt string, bit int) []byte
}

type cedarKdf struct {
}

func (cedarKdf) generate(masterPhrase string, salt string, bit int) []byte {
	Rounds := 233

	tLen := len(masterPhrase) + len(salt)
	buf := make([]byte, tLen+sha512.Size*Rounds)
	copy(buf, []byte(masterPhrase))
	copy(buf[len(masterPhrase):], []byte(salt))

	for i := 0; i < Rounds; i++ {
		endPos := tLen + i*sha512.Size
		hash := sha512.Sum512(buf[:endPos])
		copy(buf[endPos:], hash[:])
	}

	final := sha512.Sum512(buf)
	if bit%8 != 0 {
		panic("bit length must be integer multiple of 8")
	}
	if bit > 512 || bit <= 0 {
		panic("bit should be > 0 and <= 512")
	}

	ret := make([]byte, bit/8)
	copy(ret, final[:])

	return ret
}
