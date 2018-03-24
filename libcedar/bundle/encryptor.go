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

var errIllegalPacket = errors.New("packet is illegal")

const (
	magicStr = "cEdR"
)

const (
	epochStart = 0x5a83c811
)

func timeStamp() uint32 {
	//TODO: not used yet, probably useful for anti-replay
	return uint32(time.Now().Unix() - epochStart)
}

type encryptor interface {
	WritePacket(conn io.ReadWriter, msg []byte) (int, error)
	ReadPacket(conn io.ReadWriter) ([]byte, error)
	SetKey(key []byte)
}

type encryptionKey struct {
	ivPad     [64]byte //we assume 512-bit is enough
	ivKey     [64]byte
	macKey    [64]byte
	commonKey [64]byte
}

func newEncryptionKey(masterPhrase string, k kdf) *encryptionKey {
	ek := new(encryptionKey)

	copy(ek.ivPad[:], k.generate(masterPhrase, "cedar/ivPad", 512))
	copy(ek.ivKey[:], k.generate(masterPhrase, "cedar/ivKey", 512))
	copy(ek.macKey[:], k.generate(masterPhrase, "cedar/macKey", 512))
	copy(ek.commonKey[:], k.generate(masterPhrase, "cedar/commonKey", 512))

	return ek
}

type cedarEncryptor struct {
	keys       *encryptionKey
	sessionKey *[32]byte
}

func (ce *cedarEncryptor) SetKey(key []byte) {
	if len(key) < 32 {
		panic("SetKey should work with 256-bit key")
	}
	ce.sessionKey = new([32]byte)
	copy(ce.sessionKey[:], key)
}

func (ce cedarEncryptor) WritePacket(conn io.ReadWriter, msg []byte) (int, error) {
	//encryption algorithm is aes-256-cbc
	//mac: hmac-sha512(trunc to 64), mac-then-encrypt

	KeySize := 32 //256-bit
	BlockSize := 16
	FakeIvLength := BlockSize - 8

	if aes.BlockSize != BlockSize {
		panic("blocksize should be aes.blocksize")
	}

	//half padding (from ivPad), half random. Random part would be sent.
	fakeIv := make([]byte, FakeIvLength)
	DefaultRNG.Read(fakeIv) //fake IV
	iv := make([]byte, BlockSize)
	copy(iv[0:8], ce.keys.ivPad[0:8])
	copy(iv[8:BlockSize], fakeIv)

	//encrypt iv with ivKey.
	ivEnc, _ := aes.NewCipher(ce.keys.ivKey[0:KeySize])
	ivEnc.Encrypt(iv, iv)

	//if session Key is allocated, use it.
	//otherwise, use commonKey.
	var msgEnc cipher.Block
	var msgCipher cipher.BlockMode
	if ce.sessionKey != nil {
		msgEnc, _ = aes.NewCipher(ce.sessionKey[:])
		msgCipher = cipher.NewCBCEncrypter(msgEnc, iv)
	} else {
		msgEnc, _ = aes.NewCipher(ce.keys.commonKey[0:KeySize])
		msgCipher = cipher.NewCBCEncrypter(msgEnc, iv)
	}

	//head = ([fake_iv 8B]) [hmac 8B][magic 4B][length 4B]
	HeadLength := 8 + 4 + 4
	HeadIvLen := HeadLength + FakeIvLength

	newLength := FakeIvLength + (HeadLength+len(msg)+(BlockSize-1))/BlockSize*BlockSize
	paddedMsg := make([]byte, newLength)

	//Read random data, fill padding
	DefaultRNG.Read(paddedMsg[HeadIvLen+len(msg):])

	//Fill all part, leave hmac zero
	copy(paddedMsg[0:8], fakeIv)
	copy(paddedMsg[16:20], []byte(magicStr))
	copy(paddedMsg[HeadIvLen:], msg)
	binary.BigEndian.PutUint32(paddedMsg[20:24], uint32(len(msg)))

	//compute hmac
	author := hmac.New(sha512.New, ce.keys.macKey[:])
	author.Write(paddedMsg)
	sig := author.Sum(nil)
	copy(paddedMsg[8:16], sig[:8])

	//then encrypt
	msgCipher.CryptBlocks(paddedMsg[FakeIvLength:], paddedMsg[FakeIvLength:])

	ffid := binary.BigEndian.Uint32(msg[1:5])
	if ffid != 0 {
		defer log.Println("[Encryptor.Wrote]", ffid)
	}
	return conn.Write(paddedMsg)
}

func (ce cedarEncryptor) ReadPacket(conn io.ReadWriter) ([]byte, error) {
	KeySize := 32 //256-bit
	BlockSize := 16
	FakeIvLength := BlockSize - 8
	HeadLength := 8 + 4 + 4
	HeadIvLen := HeadLength + FakeIvLength

	if aes.BlockSize != BlockSize {
		panic("blocksize should be aes.blocksize")
	}

	//fast check header: if magic str is gone, drop it
	fastCheck := make([]byte, HeadIvLen)
	_, err := io.ReadFull(conn, fastCheck)
	if err != nil {
		//Early returns cause time-based attack possible. (do not care)
		//panic("read error at cedarEncryptor.ReadPacket") //for debug
		return nil, errIllegalPacket
	}

	//half padding (from ivPad), half from packet.
	iv := make([]byte, BlockSize)
	copy(iv[0:8], ce.keys.ivPad[0:8])
	copy(iv[8:BlockSize], fastCheck)

	//encrypt iv with ivKey.
	ivEnc, _ := aes.NewCipher(ce.keys.ivKey[0:KeySize])
	ivEnc.Encrypt(iv, iv)

	//if session Key is allocated, use it.
	//otherwise, use commonKey.
	var msgEnc cipher.Block
	var msgCipher cipher.BlockMode
	if ce.sessionKey != nil {
		msgEnc, _ = aes.NewCipher(ce.sessionKey[:])
		msgCipher = cipher.NewCBCDecrypter(msgEnc, iv)
	} else {
		msgEnc, _ = aes.NewCipher(ce.keys.commonKey[0:KeySize])
		msgCipher = cipher.NewCBCDecrypter(msgEnc, iv)
	}

	//head = ([fake_iv 8B]) [hmac 8B][magic 4B][length 4B]
	msgCipher.CryptBlocks(fastCheck[FakeIvLength:HeadIvLen], fastCheck[FakeIvLength:HeadIvLen])
	if string(fastCheck[FakeIvLength+8:FakeIvLength+12]) != magicStr {
		//panic("cedarEncryptor.ReadPacket failed checking magic string") //for debug
		return nil, errIllegalPacket
	}

	msgLen := binary.BigEndian.Uint32(fastCheck[FakeIvLength+12 : FakeIvLength+16])
	newLength := FakeIvLength + (HeadLength+int(msgLen)+(BlockSize-1))/BlockSize*BlockSize
	paddedMsg := make([]byte, newLength)
	copy(paddedMsg, fastCheck)

	n, err := io.ReadFull(conn, paddedMsg[HeadIvLen:])
	if err != nil || n != len(paddedMsg)-HeadIvLen {
		//panic("cedarEncryptor.ReadPacket failed checking padding") //for debug
		return nil, errIllegalPacket
	}
	msgCipher.CryptBlocks(paddedMsg[HeadIvLen:], paddedMsg[HeadIvLen:])

	//check hmac
	transferSig := make([]byte, 8)
	copy(transferSig, paddedMsg[FakeIvLength:FakeIvLength+8])
	binary.BigEndian.PutUint64(paddedMsg[FakeIvLength:FakeIvLength+8], 0)

	author := hmac.New(sha512.New, ce.keys.macKey[:])
	author.Write(paddedMsg)
	sig := author.Sum(nil)
	if !hmac.Equal(sig[0:8], transferSig) {
		//panic("cedarEncryptor.ReadPacket failed checking signature") //for debug
		return nil, errIllegalPacket
	}

	ffid := binary.BigEndian.Uint32(paddedMsg[HeadIvLen+1 : HeadIvLen+5])
	if ffid != 0 {
		defer log.Println("[Encryptor.Read]", ffid)
	}
	return paddedMsg[HeadIvLen : HeadIvLen+int(msgLen)], nil
}
