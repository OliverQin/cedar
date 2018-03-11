package bundle

import (
	"encoding/binary"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type FuncDataReceived func(id uint32, message []byte)
type FuncBundleLost func(id uint32)
type FuncFiberLost func(id uint32)
type empty struct{}

var errUnexpectedRequest = errors.New("unexpected request")

type FiberBundle struct {
	bundleType uint32
	id         uint32
	seqs       [2]uint32
	next       uint32

	fibersLock sync.RWMutex
	fibers     []*fiber

	keys encryptionKey

	bufferLen      uint32
	receiveLock    sync.RWMutex
	receiveBuffer  map[uint32]*fiberFrame
	receiveChannel chan empty

	sendTokens chan empty //token bucket for sending
	sendLock   sync.RWMutex
	//sendBuffer map[uint32]*fiberFrame

	lastActivity time.Time
	closeChan    chan empty

	confirmBuffer []uint32 //store ids to confirm
	confirmLock   sync.RWMutex

	confirmGotSignal map[uint32]chan empty
	confirmGotLock   sync.RWMutex

	onReceived   *FuncDataReceived
	onFiberLost  *FuncFiberLost
	onBundleLost *FuncBundleLost
}

func NewFiberBundle(bufferLen uint32, bundleType string, masterPhrase string) *FiberBundle {
	ret := new(FiberBundle)

	if strings.ToLower(bundleType) == "server" {
		ret.bundleType = serverBundle
	} else if strings.ToLower(bundleType) == "client" {
		ret.bundleType = clientBundle
	} else {
		panic("bundleType should be either `server` or `client`")
	}

	ret.id = 0
	ret.seqs[0] = 0
	ret.seqs[1] = 0
	ret.next = 0

	if bufferLen == 0 {
		bufferLen = 1
	}
	ret.fibers = make([]*fiber, 0)

	ret.keys = *newEncryptionKey(masterPhrase, cedarKdf{})
	//TODO: allow allocation of session key

	ret.bufferLen = bufferLen
	ret.receiveBuffer = make(map[uint32]*fiberFrame)
	ret.receiveChannel = make(chan empty, bufferLen)
	ret.confirmGotSignal = make(map[uint32]chan empty)

	ret.sendTokens = make(chan empty, bufferLen)
	//ret.sendBuffer = make(map[uint32]*fiberFrame)

	ret.lastActivity = time.Now()
	ret.closeChan = make(chan empty, 4)

	ret.confirmBuffer = make([]uint32, 0, bufferLen+1)

	ret.onReceived = nil
	ret.onFiberLost = nil
	ret.onBundleLost = nil

	go ret.keepConfirming()
	go ret.keepForwarding()

	return ret
}

func (bd *FiberBundle) SetOnReceived(f *FuncDataReceived) {
	bd.onReceived = f
}

func (bd *FiberBundle) SetOnFiberLost(f *FuncFiberLost) {
	bd.onFiberLost = f
}

func (bd *FiberBundle) SetOnBundleLost(f *FuncBundleLost) {
	bd.onBundleLost = f
}

func (bd *FiberBundle) GetSize() int {
	bd.fibersLock.RLock()
	x := len(bd.fibers)
	bd.fibersLock.RUnlock()
	return x
}

/*
AddConnection tries to add one connection to this fiber bundle.
It succeeds with (id, nil) as return value.
It returns (0, errCode) at failure.
*/
func (bd *FiberBundle) addConnection(conn rwcDeadliner) (uint32, error) {
	fb := newFiber(conn, bd.keys)

	var err error
	var id uint32
	id, err = 0, nil
	if bd.bundleType == clientBundle {
		id, err = bd.handshake(fb)
	} else {
		id, err = bd.waitHandshake(fb)
	}
	if err != nil {
		return 0, err
	}

	return id, nil
}

func (bd *FiberBundle) addAndReceive(conn rwcDeadliner) {
	fb := newFiber(conn, bd.keys)

	bd.fibersLock.Lock()
	bd.fibers = append(bd.fibers, fb)
	bd.fibersLock.Unlock()

	go bd.keepReceiving(fb)
}

/*
handshake send handshake info to remote server.
*/
func (bd *FiberBundle) handshake(fb *fiber) (uint32, error) {
	//If id is 0, ask server for a new id.
	//Otherwise, tell server to add this fiber to the bundle with this id.
	if bd.id == 0 {
		err := fb.write(fiberFrame{[]byte(""), typeRequestAllocation, 0})
		if err != nil {
			return 0, err
		}
	} else {
		var sendBuf [4]byte
		binary.BigEndian.PutUint32(sendBuf[:], bd.id)
		err := fb.write(fiberFrame{sendBuf[:], typeAddNewFiber, 0})
		if err != nil {
			return 0, err
		}
	}

	//Read message sent back.
	frm, err := fb.read()
	if err != nil {
		return 0, err
	}

	//Check message sent back.
	tp := frm.msgType
	if bd.id == 0 {
		if tp != typeAllocationConfirm || len(frm.message) < 12 {
			return 0, errAllocationFailed
		}
	} else {
		if tp != typeFiberAdded {
			return 0, errAddingFailed
		}
	}

	//Modify metadata if it is allocation
	if bd.id == 0 {
		bufBack := frm.message
		id := binary.BigEndian.Uint32(bufBack[:4])
		s2c := binary.BigEndian.Uint32(bufBack[4:8])
		c2s := binary.BigEndian.Uint32(bufBack[8:12])

		bd.id = id
		bd.seqs[download] = s2c
		bd.seqs[upload] = c2s
	}

	return bd.id, nil
}

/*
Add fiber to this bundle, only if id matches. Return 0, nil
If id does not match, return id, nil, do not add fiber
Error happens: return 0, err
*/
func (bd *FiberBundle) waitHandshake(fb *fiber) (uint32, error) {
	f, err := fb.read()
	if err != nil {
		return 0, err
	}

	id := uint32(0)
	switch f.msgType {
	case typeRequestAllocation:
		for id == 0 {
			id = DefaultRNG.Uint32()
		}
		bd.id = id

		var bufBack [12]byte
		binary.BigEndian.PutUint32(bufBack[:4], id)
		seqC2s := uint32(0) //DefaultRNG.Uint32()
		seqS2c := uint32(0) //DefaultRNG.Uint32()
		bd.seqs[upload] = seqS2c
		bd.seqs[download] = seqC2s
		binary.BigEndian.PutUint32(bufBack[4:8], seqS2c)
		binary.BigEndian.PutUint32(bufBack[8:12], seqC2s)
		writeError := fb.write(fiberFrame{bufBack[:], typeAllocationConfirm, 0})

		if writeError != nil {
			return id, writeError
		}

		return id, nil
	case typeAddNewFiber:
		id = binary.BigEndian.Uint32(f.message[:4])
		bufBack := make([]byte, 4)
		binary.BigEndian.PutUint32(bufBack, id)

		writeError := fb.write(fiberFrame{bufBack[:], typeFiberAdded, 0})
		if writeError != nil {
			return id, writeError
		}

		if bd.id == id {
			return id, nil
		}
		return id, nil
	default:
		return id, errUnexpectedRequest
	}
}

/*
GetFiberToWrite gets a fiber to write on, for sending message.
By design, this function is not thread safe.
*/
func (bd *FiberBundle) GetFiberToWrite() *fiber {
	for {
		bd.fibersLock.Lock()
		x := len(bd.fibers)
		//log.Println("bundle id:", bd.id, "size:", x)
		if x == 0 {
			bd.fibersLock.Unlock()
			time.Sleep(time.Millisecond * 1000) //0.1s, wait until there is one connection
		} else {
			break
		}
	}

	defer bd.fibersLock.Unlock()
	bd.next = (bd.next + 1) % uint32(len(bd.fibers))

	return bd.fibers[bd.next]
}

func (bd *FiberBundle) SendMessage(msg []byte) error {
	//TODO: token is needed here
	ff := fiberFrame{msg, typeSendData, 0}
	bd.sendTokens <- empty{}
	nxt := atomic.AddUint32(&(bd.seqs[upload]), 1) - 1
	ff.id = nxt

	//log.Println("[Step  2] ff.id", ff.id)
	go bd.keepSending(ff)

	return nil
}

/*
keepSending would be called once for every message (in goroutine).
It ends until message is sent and confirmed.
*/
func (bd *FiberBundle) keepSending(ff fiberFrame) {
	bd.confirmGotLock.Lock()
	bd.confirmGotSignal[ff.id] = make(chan empty, 1)
	thisChannel := bd.confirmGotSignal[ff.id]
	bd.confirmGotLock.Unlock()

	for {
		//log.Println("Geting fiber", ff.id, len(ff.message), ff.msgType)
		fb := bd.GetFiberToWrite()

		//log.Println("sending frame:", ff.id, len(ff.message), ff.msgType)
		fb.write(ff)
		//log.Println("[Step  3] ff.id, len(ff.msg), ff.msgType", ff.id, len(ff.message), ff.msgType)

		select {
		case <-time.After(globalResend):
			break
		case <-thisChannel:
			//log.Println("[Step  8] keepSending is closing: id", ff.id)
			goto ended
		case <-bd.closeChan:
			bd.closeChan <- empty{}
			goto ended
		}
	}

ended:
	bd.confirmGotLock.Lock()
	delete(bd.confirmGotSignal, ff.id)
	close(thisChannel)
	bd.confirmGotLock.Unlock()

	<-bd.sendTokens
}

func (bd *FiberBundle) keepConfirming() {
	for {
		fb := bd.GetFiberToWrite()

		bd.confirmLock.Lock()
		info := make([]byte, len(bd.confirmBuffer)*4)
		for i := 0; i < len(info); i += 4 {
			binary.BigEndian.PutUint32(info[i:i+4], bd.confirmBuffer[i/4])
			//log.Println("[Step  6] Sending confirm :id", bd.confirmBuffer[i/4])
		}
		bd.confirmBuffer = bd.confirmBuffer[:0]
		bd.confirmLock.Unlock()

		if len(info) > 0 {
			fb.write(fiberFrame{info, typeDataReceived, 0})
		}

		time.Sleep(globalConfirmWait)
	}
}

func (bd *FiberBundle) keepReceiving(fb *fiber) error {
	for {
		//log.Println("keepReceiving", fb.conn)
		ff, err := fb.read()

		if err != nil {
			//TODO: close
			return err
		}

		if ff.msgType == typeSendData {
			if !bd.inRange(ff.id) {
				continue
			}
			//log.Println("[Step  4] data rec: id, len(msg)", ff.id, len(ff.message))

			bd.receiveLock.Lock()
			bd.receiveBuffer[ff.id] = ff
			bd.receiveLock.Unlock()

			bd.receiveChannel <- empty{}

			bd.confirmLock.Lock()
			bd.confirmBuffer = append(bd.confirmBuffer, ff.id)
			bd.confirmLock.Unlock()
			//log.Println("[Step  5] confirming :id, len(msg)", ff.id, len(ff.message))
		}
		if ff.msgType == typeDataReceived {
			buf := ff.message

			bd.confirmGotLock.Lock()
			for i := 0; i < len(buf); i += 4 {
				id := binary.BigEndian.Uint32(buf[i : i+4])
				//log.Println("[Step  7] confirm got", id)
				chn, ok := bd.confirmGotSignal[id]
				if ok {
					chn <- empty{}
				}
			}
			bd.confirmGotLock.Unlock()
		}
	}
}

func (bd *FiberBundle) keepForwarding() {
	for {
		select {
		case <-bd.receiveChannel:
			bd.receiveLock.Lock()
			for {
				seq := bd.seqs[download]
				ff, ok := bd.receiveBuffer[seq]
				//log.Println("seq, status", seq, ok)
				if ok {
					bd.seqs[download]++
					if bd.onReceived != nil {
						(*bd.onReceived)(bd.id, ff.message)
						//log.Println("[Step  9] call_bd_onrec", ff.id)
					}
				} else {
					break
				}
			}
			bd.receiveLock.Unlock()

		case <-bd.closeChan:
			bd.closeChan <- empty{}
			break
		}
	}
	return
}

func (bd *FiberBundle) inRange(packetId uint32) bool {
	seqA := atomic.LoadUint32(&bd.seqs[download])
	seqB := seqA + bd.bufferLen

	//log.Println("Range: ", seqA, seqB, packetId, bd.id)
	if seqA <= seqB {
		return (seqA <= packetId && packetId < seqB)
	}
	return (packetId >= seqA || packetId < seqB)
}
