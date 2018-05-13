package bundle

import (
	"bytes"
	"fmt"
	"log"
	"testing"
)

func TestCedarEncryptor(t *testing.T) {
	//ek := newEncryptionKey("test", SimpleKDF{})
	encryptor := NewCedarCryptoIO("test_test_test")

	frw := bytes.NewBuffer(nil)
	//ssKey := SimpleKDF{}.Generate("gg", "session", 512)

	for r := 0; r < 20; r++ {
		for i := 5; i < 65; i++ {
			msg := make([]byte, i)
			fmt.Println(i, r)
			DefaultRNG.Read(msg)

			//encryptor.sessionKey = nil
			encryptor.WritePacket(frw, msg)
			p, err := encryptor.ReadPacket(frw)

			if err != nil {
				panic(err.Error())
			}
			if !bytes.Equal(msg, p) {
				log.Println("r=", r, "i=", i)
				panic("message changed after enc/dec")
			}

			/*encryptor.sessionKey = new([32]byte)
			copy(encryptor.sessionKey[:], ssKey)
			encryptor.WritePacket(frw, msg)
			q, err := encryptor.ReadPacket(frw)
			if err != nil {
				panic(err.Error())
			}
			if !bytes.Equal(msg, q) {
				log.Println("r=", r, "i=", i)
				panic("message changed after enc/dec, using session key")
			}*/
		}
	}

	return
}
