package bundle

/*
how to send:
	write one_by_one

total buffer -> for read and rearrange
individual channel -> for send

id: seqS2C, seqC2S
*/

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
	"time"
)

type rwcDeadliner interface {
	io.ReadWriteCloser
	SetDeadline(time.Time) error
}

type fiberFrame struct {
	message []byte
	msgType uint32
	id      uint32
}

type fiber struct {
	conn rwcDeadliner
	enc  encryptor

	lastActivity time.Time

	sigChan chan os.Signal
}

var errFiberWrite = errors.New("fiber fails during writing")
var errFiberRead = errors.New("fiber fails during reading")

func newFiber(conn rwcDeadliner, key encryptionKey) *fiber {
	ret := new(fiber)
	ret.conn = conn
	ret.sigChan = make(chan os.Signal, 1) //TODO: use this

	ret.lastActivity = time.Now()

	newKey := key
	ret.enc = &cedarEncryptor{&newKey, nil}

	go ret.keepHeartbeating()

	return ret
}

func (fb *fiber) keepHeartbeating() {
	for {
		select {
		case <-fb.sigChan:
			fb.conn.Close()
			return
		case t := <-time.After(globalResend):
			if fb.lastActivity.Add(globalResend).Before(t) {
				fb.sendHeartbeat()
			}
		}
	}
}

func (fb *fiber) sendHeartbeat() {
	fb.write(fiberFrame{nil, typeHeartbeat, 0})
}

func (fb *fiber) pack(f *fiberFrame) []byte {
	ret := make([]byte, len(f.message)+1+4)

	ret[0] = uint8(f.msgType)
	binary.BigEndian.PutUint32(ret[1:5], f.id)
	copy(ret[5:], f.message)

	return ret
}

func (fb *fiber) unpack(msg []byte) *fiberFrame {
	ret := new(fiberFrame)
	ret.message = msg[5:]
	ret.msgType = uint32(msg[0])
	ret.id = binary.BigEndian.Uint32(msg[1:5])

	return ret
}

func (fb *fiber) read() (*fiberFrame, error) {
	msg, err := fb.enc.ReadPacket(fb.conn)

	if err != nil {
		return nil, errFiberRead
	}
	ret := fb.unpack(msg)

	return ret, nil
}

func (fb *fiber) write(f fiberFrame) error {
	packed := fb.pack(&f)
	n, err := fb.enc.WritePacket(fb.conn, packed)
	if n < len(packed) || err != nil {
		return errFiberWrite
	}
	return nil
}
