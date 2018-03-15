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
	"sync"
	"sync/atomic"
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

	lastActivity     time.Time
	lastActivityLock sync.Mutex

	activated uint32
	ended     chan error
}

var errFiberWrite = errors.New("fiber fails during writing")
var errFiberRead = errors.New("fiber fails during reading")

func newFiber(conn rwcDeadliner, key encryptionKey) *fiber {
	ret := new(fiber)
	ret.conn = conn
	ret.ended = make(chan error, 1) //TODO: use this

	ret.lastActivity = time.Now()

	newKey := key
	ret.enc = &cedarEncryptor{&newKey, nil}

	ret.activated = 0

	return ret
}

func (fb *fiber) keepHeartbeating() {
	for {
		select {
		case err := <-fb.ended:
			fb.ended <- err
			fb.conn.Close()
			return
		case t := <-time.After(globalResend):
			fb.lastActivityLock.Lock()
			nt := fb.lastActivity.Add(globalResend) //TODO: add randomness
			fb.lastActivity = time.Now()
			fb.lastActivityLock.Unlock()

			if nt.Before(t) {
				fb.sendHeartbeat()
				//log.Println(nt, t, "heartbeat", &fb)
			}
		}
	}
}

func (fb *fiber) sendHeartbeat() {
	fb.write(fiberFrame{nil, typeHeartbeat, 0})
}

/*
handshake send handshake info to remote server.
*/
func (fb *fiber) handshake(id uint32) (uint32, uint32, uint32, error) {
	//If id is 0, ask server for a new id.
	//Otherwise, tell server to add this fiber to the bundle with this id.
	if id == 0 {
		err := fb.write(fiberFrame{[]byte(""), typeRequestAllocation, 0})
		if err != nil {
			return 0, 0, 0, err
		}
	} else {
		var sendBuf [4]byte
		binary.BigEndian.PutUint32(sendBuf[:], id)
		err := fb.write(fiberFrame{sendBuf[:], typeAddNewFiber, 0})
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
}

/*
Add fiber to this bundle, only if id matches. Return 0, nil
If id does not match, return id, nil, do not add fiber
Error happens: return 0, err
*/
func (fb *fiber) waitHandshake() (uint32, uint32, uint32, error) {
	f, err := fb.read()
	if err != nil {
		return 0, 0, 0, err
	}

	id := uint32(0)
	switch f.msgType {
	case typeRequestAllocation:
		for id == 0 {
			id = DefaultRNG.Uint32()
		}

		var bufBack [12]byte
		binary.BigEndian.PutUint32(bufBack[:4], id)
		//seqC2s := uint32(0) //DefaultRNG.Uint32()
		//seqS2c := uint32(0) //DefaultRNG.Uint32()
		seqC2s := DefaultRNG.Uint32()
		seqS2c := DefaultRNG.Uint32()

		binary.BigEndian.PutUint32(bufBack[4:8], seqS2c)
		binary.BigEndian.PutUint32(bufBack[8:12], seqC2s)
		writeError := fb.write(fiberFrame{bufBack[:], typeAllocationConfirm, 0})

		if writeError != nil {
			return id, 0, 0, writeError
		}

		return id, seqC2s, seqS2c, nil
	case typeAddNewFiber:
		id = binary.BigEndian.Uint32(f.message[:4])
		bufBack := make([]byte, 4)
		binary.BigEndian.PutUint32(bufBack, id)

		writeError := fb.write(fiberFrame{bufBack[:], typeFiberAdded, 0})
		if writeError != nil {
			return id, 0, 0, writeError
		}

		return id, 0, 0, nil
	default:
		return id, 0, 0, errUnexpectedRequest
	}
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

	fb.lastActivityLock.Lock()
	fb.lastActivity = time.Now()
	fb.lastActivityLock.Unlock()

	return ret, nil
}

func (fb *fiber) write(f fiberFrame) error {
	packed := fb.pack(&f)
	n, err := fb.enc.WritePacket(fb.conn, packed)
	if n < len(packed) || err != nil {
		return errFiberWrite
	}

	fb.lastActivityLock.Lock()
	fb.lastActivity = time.Now()
	fb.lastActivityLock.Unlock()

	return nil
}

func (fb *fiber) activate() {
	if atomic.AddUint32(&fb.activated, 1) == 1 {
		go fb.keepHeartbeating()
	} else {
		panic("activate should be not called twice")
	}
}

func (fb *fiber) close(err error) {
	fb.ended <- err
	fb.conn.Close()
}
