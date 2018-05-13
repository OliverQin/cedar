package bundle

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"sync"
	"sync/atomic"
)

/*
Handshaker manages the handshakes.
It accepts a handshake packet from server's side, and also sends handshake packet from client.
*/
type Handshaker struct {
	encryptor CryptoIO
	bundles   *BundleCollection

	nonceLock    sync.Mutex
	nonceArray   []uint64
	nonceCounter uint64
}

type HandshakeResult struct {
	id    uint32
	idS2C uint32 // ID of next packet from Server/Client to Client/Server.
	idC2S uint32
	conn  io.ReadWriteCloser
}

const (
	nonceArraySize = 4096
	applyMagic     = "cEdr_Go!"
	addMagic       = "gO_ceDR!"
	replyMagic     = "AccEPt!!"
	refuseMagic    = "!fAiLEd!"
)

var ErrHandshakeFailed = errors.New("handshake failed")

func NewHandshaker(encryptor CryptoIO, bundles *BundleCollection) *Handshaker {
	ret := new(Handshaker)
	ret.encryptor = encryptor
	ret.bundles = bundles
	ret.nonceArray = make([]uint64, nonceArraySize)
	ret.nonceCounter = 0
	return ret
}

/*func (hs *Handshaker) Send(id uint32) (HandshakeResult error) {
	//If id is 0, ask server for a new id.
	//Otherwise, tell server to add this Fiber to the bundle with this id.
	if id == 0 {
		err := fb.write(FiberPacket{[]byte(""), typeRequestAllocation, 0})
		if err != nil {
			return 0, 0, 0, err
		}
	} else {
		var sendBuf [4]byte
		binary.BigEndian.PutUint32(sendBuf[:], id)
		err := fb.write(FiberPacket{sendBuf[:], typeAddNewFiber, 0})
		if err != nil {
			return 0, 0, 0, err
		}
	}

	//Read message sent back.
	frm, err := fb.read()
	if err != nil {
		return 0, 0, 0, err
	}

	//Check message sent back.
	tp := frm.msgType
	if id == 0 {
		if tp != typeAllocationConfirm || len(frm.message) < 12 {
			return 0, 0, 0, errAllocationFailed
		}
	} else {
		if tp != typeFiberAdded {
			return 0, 0, 0, errAddingFailed
		}
	}

	//Modify metadata if it is allocation
	if id == 0 {
		bufBack := frm.message

		newID := binary.BigEndian.Uint32(bufBack[:4])
		s2c := binary.BigEndian.Uint32(bufBack[4:8])
		c2s := binary.BigEndian.Uint32(bufBack[8:12])

		return newID, c2s, s2c, nil
	}

	return id, 0, 0, nil
}*/

func (hs *Handshaker) RequestNewBundle(conn io.ReadWriteCloser) (HandshakeResult, error) {
	//Prepare for message
	msg := make([]byte, 16)
	nonce := DefaultRNG.Uint64()
	copy(msg[0:8], applyMagic)
	binary.BigEndian.PutUint64(msg[8:16], nonce)

	//Ask server for new ID
	_, err := hs.encryptor.WritePacket(conn, msg)
	if err != nil {
		return HandshakeResult{}, err
	}

	return hs.getResponse(conn)
}

func (hs *Handshaker) createNewBundle(conn io.ReadWriteCloser) (HandshakeResult, error) {
	id := uint32(0)
	for id == 0 || hs.bundles.HasID(id) {
		id = DefaultRNG.Uint32()
	}

	var msg [20]byte
	seqC2s := DefaultRNG.Uint32()
	seqS2c := DefaultRNG.Uint32()

	copy(msg[0:8], []byte(replyMagic))
	binary.BigEndian.PutUint32(msg[8:12], id)
	binary.BigEndian.PutUint32(msg[12:16], seqS2c)
	binary.BigEndian.PutUint32(msg[16:20], seqC2s)

	_, err := hs.encryptor.WritePacket(conn, msg[:])
	if err != nil {
		return HandshakeResult{}, err
	}

	return HandshakeResult{id, seqS2c, seqC2s, conn}, nil
}

func (hs *Handshaker) addNonce(nonce uint64) bool {
	hs.nonceLock.Lock()

	var size int
	if hs.nonceCounter >= nonceArraySize {
		size = nonceArraySize
	} else {
		size = int(hs.nonceCounter)
	}
	for i := 0; i < size; i++ {
		if hs.nonceArray[i] == nonce {
			return false
		}
	}

	hs.nonceArray[hs.nonceCounter%nonceArraySize] = nonce
	hs.nonceCounter++
	return true
}

func (hs *Handshaker) addBundle(conn io.ReadWriteCloser, id uint32) (HandshakeResult, error) {
	msg := make([]byte, 4)
	binary.BigEndian.PutUint32(msg, id)

	if !hs.bundles.HasID(id) {
		return HandshakeResult{}, ErrHandshakeFailed
	}

	bd := hs.bundles.GetBundle(id)

	copy(msg[0:8], []byte(replyMagic))
	c2s := atomic.LoadUint32(&bd.seqs[download])
	s2c := atomic.LoadUint32(&bd.seqs[upload])
	binary.BigEndian.PutUint32(msg[8:12], id)
	binary.BigEndian.PutUint32(msg[12:16], s2c)
	binary.BigEndian.PutUint32(msg[16:20], c2s)

	_, err := hs.encryptor.WritePacket(conn, msg[:])
	if err != nil {
		return HandshakeResult{}, err
	}

	return HandshakeResult{id, s2c, c2s, conn}, nil

}

func (hs *Handshaker) ConfirmHandshake(conn io.ReadWriteCloser) (HandshakeResult, error) {
	msg, err := hs.encryptor.ReadPacket(conn)
	if err != nil {
		return HandshakeResult{}, ErrHandshakeFailed
	}
	if len(msg) >= 16 {
		nonce := binary.BigEndian.Uint64(msg[8:16])
		if !hs.addNonce(nonce) {
			return HandshakeResult{}, ErrHandshakeFailed
		}
	}

	if len(msg) == 16 && bytes.Equal(msg[0:8], []byte(applyMagic)) {
		return hs.createNewBundle(conn)
	}
	if len(msg) == 20 && bytes.Equal(msg[0:8], []byte(addMagic)) {
		id := binary.BigEndian.Uint32(msg[16:20])
		return hs.addBundle(conn, id)
	}

	return HandshakeResult{}, ErrHandshakeFailed
}

func (hs *Handshaker) getResponse(conn io.ReadWriteCloser) (HandshakeResult, error) {
	msg, err := hs.encryptor.ReadPacket(conn)
	if err != nil {
		return HandshakeResult{}, err
	}

	if len(msg) != 20 || bytes.Equal(msg[0:8], []byte(replyMagic)) {
		return HandshakeResult{}, ErrHandshakeFailed
	}

	ret := HandshakeResult{}
	ret.id = binary.BigEndian.Uint32(msg[8:12])
	ret.idS2C = binary.BigEndian.Uint32(msg[12:16])
	ret.idC2S = binary.BigEndian.Uint32(msg[16:20])

	return ret, nil
}
