package bundle

import (
	"bytes"
	"log"
	"testing"
)

func TestCedarEncryptor(t *testing.T) {
	ek := newEncryptionKey("test", cedarKdf{})
	encryptor := cedarEncryptor{ek, nil}

	frw := bytes.NewBuffer(nil)

	for r := 0; r < 20; r++ {
		for i := 0; i < 65; i++ {
			msg := make([]byte, i)
			DefaultRNG.Read(msg)

			encryptor.WritePacket(frw, msg)
			p, err := encryptor.ReadPacket(frw)

			if err != nil {
				panic(err.Error())
			}
			if !bytes.Equal(msg, p) {
				log.Println("r=", r, "i=", i)
				panic("message changed after enc/dec")
			}
		}
	}

	return
}
