package bundle

import (
	"crypto/sha512"
)

/*
KDF is an interface for key derivation function.
Number of iterations is fixed.
*/
type KDF interface {
	Generate(password string, salt string, bit int) []byte
}

/*
SimpleKDF is a simple and casual KDF.
*/
type SimpleKDF struct {
}

/*
Generate accepts password, salt, and number of bits (must be not larger than 512 and divided by 8) and returns a key.
*/
func (SimpleKDF) Generate(password string, salt string, bit int) []byte {
	// TODO:	Design of this function is quite casual.
	//			Implement some standards and replace it in the future.

	if bit%8 != 0 {
		panic("bit length must be integer multiple of 8")
	}
	if bit > 512 || bit <= 0 {
		panic("bit should be > 0 and <= 512")
	}

	Rounds := 23333
	MagicStr := []byte("Shall I compare thee to a summer's day?")

	hasher := sha512.New()

	for i := 0; i < Rounds; i++ {
		hasher.Write([]byte(password))
		hasher.Write([]byte(salt))
		hasher.Write(hasher.Sum(nil))
		hasher.Write(MagicStr)
	}

	final := hasher.Sum(nil)

	ret := make([]byte, bit/8)
	copy(ret, final[:])

	return ret
}
