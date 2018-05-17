package bundle

import (
	"testing"
)

func TestHandshakeBasic(t *testing.T) {
	conns := localConnPairs("127.0.0.1:20003", 2)

	encryptor := NewCedarCryptoIO("12345")

	bdc := NewBundleCollection()
	handshaker := NewHandshaker(encryptor, bdc)

	go handshaker.ConfirmHandshake(conns[0])
	hsr, err := handshaker.RequestNewBundle(conns[2])

	LogDebug("ID:", hsr.id)
	if err != nil {
		panic("RequestNewBundle failed")
	}

	bdc.AddBundle(NewFiberBundle(50, "server", &hsr))

	go handshaker.ConfirmHandshake(conns[1])
	hsr2, err := handshaker.RequestAddToBundle(conns[3], hsr.id)
	if err != nil {
		panic("RequestAddToBundle failed")
	}
	if hsr2.id != hsr.id || hsr2.idC2S != hsr.idC2S || hsr2.idS2C != hsr.idS2C {
		panic("hsr should be equal to hsr2 (id, seqs2c, seqc2s)")
	}
}
