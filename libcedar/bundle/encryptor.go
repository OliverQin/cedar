package bundle

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/binary"
	"errors"
	"io"
	"log"
	"time"
)

/*
ErrIllegalPacket is returned when an illegal packet is received.
*/
var ErrIllegalPacket = errors.New("packet is illegal")

const (
	epochStart  = int64(0x5a83c811)
	timeDiffTol = 600 // Tolerance of time difference between two machine, +/- ten minutes
)

func timestamp() uint32 {
	return uint32(time.Now().Unix() - epochStart)
}

func timeMatch(network uint32) bool {
	local := timestamp()
	if network > local {
		return (network - local) <= timeDiffTol
	}
	return (local - network) <= timeDiffTol
}

/*
CryptoIO is an interface for crypto-writing/reading on a stream.
*/
type CryptoIO interface {
	WritePacket(conn io.ReadWriter, msg []byte) (int, error)
	ReadPacket(conn io.ReadWriter) ([]byte, error)
	SetKey(password string)
}

/*
CedarCryptoIO is CryptoIO for Cedar.
*/
type CedarCryptoIO struct {
	ivCipher  cipher.Block
	msgCipher cipher.Block

	macKey []byte
	ivPad  []byte
}

/*
NewCedarCryptoIO create a new CedarCryptoIO.
*/
func NewCedarCryptoIO(password string) *CedarCryptoIO {
	ret := new(CedarCryptoIO)
	ret.SetKey(password)

	return ret
}

/*
SetKey sets password for CedarCryptoIO.
*/
func (ce *CedarCryptoIO) SetKey(password string) {
	//encryption algorithm is aes-256-cbc
	//mac: hmac-sha512(trunc to 64), mac-then-encrypt
	//KDF here is used for strengthening keys and deriving (irrelevant) keys from same password.

	k := SimpleKDF{}
	ce.ivCipher, _ = aes.NewCipher(k.Generate(password, "cedar/ivKey", 256))
	ce.msgCipher, _ = aes.NewCipher(k.Generate(password, "cedar/msgKey", 256))

	ce.macKey = k.Generate(password, "cedar/macKey", 512)
	ce.ivPad = k.Generate(password, "cedar/ivPad", 64)

	return
}

/*
WritePacket writes a block of encrypted message to conn.
It returns the size actually wrote (larger than len(msg)) and error.
*/
func (ce CedarCryptoIO) WritePacket(conn io.ReadWriter, msg []byte) (int, error) {
	//TODO: Add support of session key
	//KeySize := 32 //256-bit
	BlockSize := 16
	FakeIVLength := 8

	if aes.BlockSize != BlockSize {
		panic("blocksize should be aes.blocksize")
	}

	//half padding (from ivPad), half random. Random part would be sent.
	//create fake IV
	fakeIV := make([]byte, FakeIVLength)
	DefaultRNG.Read(fakeIV)

	//concat fake IV with IV padding
	iv := make([]byte, BlockSize)
	copy(iv[0:8], ce.ivPad)
	copy(iv[8:BlockSize], fakeIV)

	//encrypt iv with ivKey.
	ce.ivCipher.Encrypt(iv, iv)

	//head = ([fake_iv 8B]) [hmac 8B][timestamp 4B][length 4B]
	HeadLength := 8 + 4 + 4
	HeadIVLen := FakeIVLength + HeadLength

	newLength := FakeIVLength + (HeadLength+len(msg)+(BlockSize-1))/BlockSize*BlockSize
	paddedMsg := make([]byte, newLength)

	//Read random data, fill padding
	DefaultRNG.Read(paddedMsg[HeadIVLen+len(msg):])

	//Fill all part, leave hmac zero
	copy(paddedMsg[0:8], fakeIV)
	binary.BigEndian.PutUint32(paddedMsg[16:20], timestamp())
	binary.BigEndian.PutUint32(paddedMsg[20:24], uint32(len(msg)))
	copy(paddedMsg[HeadIVLen:], msg)

	//compute hmac
	author := hmac.New(sha512.New, ce.macKey)
	author.Write(paddedMsg)
	sig := author.Sum(nil)
	copy(paddedMsg[8:16], sig[:8])

	//then encrypt
	coder := cipher.NewCBCEncrypter(ce.msgCipher, iv)
	coder.CryptBlocks(paddedMsg[FakeIVLength:], paddedMsg[FakeIVLength:])

	ffid := binary.BigEndian.Uint32(msg[1:5])
	if ffid != 0 {
		defer log.Println("[Encryptor.Wrote]", ffid)
	}
	return conn.Write(paddedMsg)
}

/*
ReadPacket reads a packet of encrypted message from conn.
It returns the []byte got and error.
When any error occur, the []byte returned is nil.
*/
func (ce CedarCryptoIO) ReadPacket(conn io.ReadWriter) ([]byte, error) {
	// KeySize := 32 //256-bit
	BlockSize := 16
	FakeIVLength := BlockSize - 8
	HeadLength := 8 + 4 + 4
	HeadIVLen := HeadLength + FakeIVLength

	if aes.BlockSize != BlockSize {
		panic("blocksize should be aes.blocksize")
	}

	//fast check header: if magic str is gone, drop it
	fastCheck := make([]byte, HeadIVLen)
	_, err := io.ReadFull(conn, fastCheck)
	if err != nil {
		//Early returns cause time-based attack possible. (do not care)
		return nil, ErrIllegalPacket
	}

	//half padding (from ivPad), half from packet.
	iv := make([]byte, BlockSize)
	copy(iv[0:8], ce.ivPad)
	copy(iv[8:BlockSize], fastCheck)

	//encrypt iv with ivKey.
	ce.ivCipher.Encrypt(iv, iv)

	//if session Key is allocated, use it.
	//otherwise, use commonKey.
	coder := cipher.NewCBCDecrypter(ce.msgCipher, iv)

	//head = ([fake_iv 8B]) [hmac 8B][timestamp 4B][length 4B]
	coder.CryptBlocks(fastCheck[FakeIVLength:HeadIVLen], fastCheck[FakeIVLength:HeadIVLen])
	if !timeMatch(binary.BigEndian.Uint32(fastCheck[FakeIVLength+8 : FakeIVLength+12])) {
		return nil, ErrIllegalPacket
	}

	msgLen := binary.BigEndian.Uint32(fastCheck[FakeIVLength+12 : FakeIVLength+16])
	newLength := FakeIVLength + (HeadLength+int(msgLen)+(BlockSize-1))/BlockSize*BlockSize
	paddedMsg := make([]byte, newLength)
	copy(paddedMsg, fastCheck)

	n, err := io.ReadFull(conn, paddedMsg[HeadIVLen:])
	if err != nil || n != len(paddedMsg)-HeadIVLen {
		return nil, ErrIllegalPacket
	}
	coder.CryptBlocks(paddedMsg[HeadIVLen:], paddedMsg[HeadIVLen:])

	//check hmac
	transferSig := make([]byte, 8)
	copy(transferSig, paddedMsg[FakeIVLength:FakeIVLength+8])
	binary.BigEndian.PutUint64(paddedMsg[FakeIVLength:FakeIVLength+8], 0)

	author := hmac.New(sha512.New, ce.macKey)
	author.Write(paddedMsg)
	sig := author.Sum(nil)
	if !hmac.Equal(sig[0:8], transferSig) {
		return nil, ErrIllegalPacket
	}

	return paddedMsg[HeadIVLen : HeadIVLen+int(msgLen)], nil
}
