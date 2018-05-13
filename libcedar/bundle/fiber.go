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
	"log"
	"sync/atomic"
	"time"
)

type FiberPacket struct {
	id      uint32
	msgType uint32
	message []byte
}

type Fiber struct {
	conn      io.ReadWriteCloser
	encryptor CryptoIO
	bundle    *FiberBundle

	lastRead  int64
	lastWrite int64

	closeSignal chan error
	cleaned     uint32
}

var errFiberWrite = errors.New("Fiber fails during writing")
var errFiberRead = errors.New("Fiber fails during reading")
var ErrConnectionTimeout = errors.New("connection timeout")

func NewFiber(conn io.ReadWriteCloser, encryptor CryptoIO, bundle *FiberBundle) *Fiber {
	ret := new(Fiber)

	ret.conn = conn
	ret.encryptor = encryptor
	ret.bundle = bundle

	ret.lastRead = time.Now().Unix()
	ret.lastWrite = time.Now().Unix()

	ret.closeSignal = make(chan error, 88)
	ret.cleaned = 0

	go ret.keepHeartbeating()
	go ret.keepReading()

	if bundle != nil {
		bundle.FiberCreated(ret)
	}

	return ret
}

func (fb *Fiber) keepHeartbeating() {
	for {
		select {
		case err := <-fb.closeSignal:
			fb.close(err)
			return

		case t := <-time.After(globalMinHeartbeat): //TODO: Add randomness
			lrt := atomic.LoadInt64(&fb.lastRead)
			ddl := lrt + int64(GlobalConnectionTimeout/time.Second)
			if ddl < t.Unix() {
				fb.close(ErrConnectionTimeout)
				return
			}

			hbt := lrt + int64(globalMinHeartbeat/time.Second)
			if hbt <= t.Unix() {
				fb.sendHeartbeat()
			}
		}
	}
}

func (fb *Fiber) keepReading() {
	for {
		select {
		case err := <-fb.closeSignal:
			fb.close(err)
			return
		default:
			//do nothing
		}

		pkt, err := fb.read()
		if err != nil {
			fb.close(err)
			return
		}

		if fb.bundle != nil {
			fb.bundle.PacketReceived(pkt)
		}
	}
}

func (fb *Fiber) sendHeartbeat() {
	fb.write(FiberPacket{0, typeHeartbeat, nil})
}

func (fb *Fiber) pack(f *FiberPacket) []byte {
	ret := make([]byte, len(f.message)+1+4)

	ret[0] = uint8(f.msgType)
	binary.BigEndian.PutUint32(ret[1:5], f.id)
	copy(ret[5:], f.message)

	return ret
}

func (fb *Fiber) unpack(msg []byte) *FiberPacket {
	ret := new(FiberPacket)
	ret.message = msg[5:]
	ret.msgType = uint32(msg[0])
	ret.id = binary.BigEndian.Uint32(msg[1:5])

	return ret
}

func (fb *Fiber) read() (*FiberPacket, error) {
	log.Println("[Fiber.read.reading]", fb)
	msg, err := fb.encryptor.ReadPacket(fb.conn)

	if err != nil {
		//panic("read error should not happen") //for debug
		log.Println(err)
		return nil, errFiberRead
	}
	ret := fb.unpack(msg)

	if ret.msgType == typeSendData {
		log.Println("[Fiber.read]", ret.id)
	}

	atomic.StoreInt64(&fb.lastRead, time.Now().Unix())

	return ret, nil
}

func (fb *Fiber) write(f FiberPacket) error {
	packed := fb.pack(&f)
	n, err := fb.encryptor.WritePacket(fb.conn, packed)
	if f.msgType == typeSendData {
		log.Println("[Fiber.write]", f.id)
	}
	if n < len(packed) || err != nil {
		//panic("write error should not happen") //for debug
		return errFiberWrite
	}

	atomic.StoreInt64(&fb.lastWrite, time.Now().Unix())

	return nil
}

func (fb *Fiber) close(err error) {
	if 1 == atomic.AddUint32(&fb.cleaned, 1) {
		for i := 0; i < 5; i++ {
			fb.closeSignal <- err
		}

		fb.conn.Close()
		if fb.bundle != nil {
			fb.bundle.FiberClosed(fb)
		}
	}
	log.Println("[Fiber.close]", fb, err)
}
