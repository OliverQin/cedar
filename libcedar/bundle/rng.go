package bundle

import (
	"crypto/rand"
	"encoding/binary"
)

type CryptoRng struct {
}

func NewCryptoRng() *CryptoRng {
	c := new(CryptoRng)

	return c
}

func (c *CryptoRng) Read(buf []byte) int {
	n, _ := rand.Read(buf)
	return n
}

func (c *CryptoRng) Uint16() uint16 {
	t := make([]byte, 2)
	c.Read(t)
	return binary.LittleEndian.Uint16(t)
}

func (c *CryptoRng) Uint32() uint32 {
	t := make([]byte, 4)
	c.Read(t)
	return binary.LittleEndian.Uint32(t)
}

func (c *CryptoRng) Uint64() uint64 {
	t := make([]byte, 8)
	c.Read(t)
	return binary.LittleEndian.Uint64(t)
}

func (c *CryptoRng) Int16() int16 {
	t := make([]byte, 2)
	c.Read(t)
	return int16(binary.LittleEndian.Uint16(t))
}

func (c *CryptoRng) Int32() int32 {
	t := make([]byte, 4)
	c.Read(t)
	return int32(binary.LittleEndian.Uint32(t))
}

func (c *CryptoRng) Int64() int64 {
	t := make([]byte, 8)
	c.Read(t)
	return int64(binary.LittleEndian.Uint64(t))
}

var DefaultRNG = NewCryptoRng()
