package bundle

import (
	"encoding/binary"
	"errors"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type FuncBundleCreated func(id uint32)
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

	closeChan chan empty

	confirmBuffer []uint32 //store ids to confirm
	confirmLock   sync.RWMutex

	confirmGotSignal map[uint32]chan empty
	confirmGotLock   sync.RWMutex

	callbackLock    sync.RWMutex
	onBundleCreated FuncBundleCreated
	onReceived      FuncDataReceived
	onFiberLost     FuncFiberLost
	onBundleLost    FuncBundleLost
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

	ret.closeChan = make(chan empty, 4)

	ret.confirmBuffer = make([]uint32, 0, bufferLen+1)

	ret.onReceived = nil
	ret.onFiberLost = nil
	ret.onBundleLost = nil

	go ret.keepConfirming()
	go ret.keepForwarding()
	go ret.keepDebugging() //TODO: remove this

	return ret
}

func (bd *FiberBundle) SetOnReceived(f FuncDataReceived) {
	bd.callbackLock.Lock()
	bd.onReceived = f
	bd.callbackLock.Unlock()
}

func (bd *FiberBundle) SetOnFiberLost(f FuncFiberLost) {
	bd.callbackLock.Lock()
	bd.onFiberLost = f
	bd.callbackLock.Unlock()
}

func (bd *FiberBundle) SetOnBundleLost(f FuncBundleLost) {
	bd.callbackLock.Lock()
	bd.onBundleLost = f
	bd.callbackLock.Unlock()
}

func (bd *FiberBundle) SetOnBundleCreated(f FuncBundleCreated) {
	bd.callbackLock.Lock()
	bd.onBundleCreated = f
	bd.callbackLock.Unlock()
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
func (bd *FiberBundle) addConnection(conn rwcDeadliner) (uint32, *fiber, error) {
	//FIXME: this ugly signature is a work around
	fb := newFiber(conn, bd.keys)

	var err error
	var id, c2s, s2c uint32
	id, err = 0, nil
	if bd.bundleType == clientBundle {
		id, c2s, s2c, err = fb.handshake(bd.id)
	} else {
		id, c2s, s2c, err = fb.waitHandshake()
	}
	if err != nil {
		return 0, nil, err
	}

	if bd.id == 0 { //A new bundle, not set yet
		bd.id = id

		if bd.bundleType == clientBundle {
			bd.seqs[download] = s2c
			bd.seqs[upload] = c2s
		} else {
			bd.seqs[upload] = s2c
			bd.seqs[download] = c2s
		}
	}

	return id, fb, nil
}

func (bd *FiberBundle) addAndReceive(fb *fiber) {
	//fb := newFiber(conn, bd.keys)
	bd.fibersLock.Lock()
	bd.fibers = append(bd.fibers, fb)
	bd.fibersLock.Unlock()

	fb.activate()
	go bd.keepReceiving(fb)
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

	log.Println("[Bundle.SendMessage] ", ff.id, ShortHash(msg))
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
		fb := bd.GetFiberToWrite()

		log.Println("[Bundle.keepSending.Got]", ff.id)
		fb.write(ff)
		log.Println("[Bundle.keepSending.Wrote]", ff.id)
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
	//log.Println("ff.id sent successfully", ff.id)
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
			log.Println("[keepConfirming.confirmSent]", bd.confirmBuffer[i/4])
		}
		bd.confirmBuffer = bd.confirmBuffer[:0]
		bd.confirmLock.Unlock()

		if len(info) > 0 {
			fb.write(fiberFrame{info, typeDataReceived, 0})
			//log.Println(time.Now(), &fb)
		}

		time.Sleep(globalConfirmWait)
	}
}

func (bd *FiberBundle) keepReceiving(fb *fiber) error {
	for {
		//log.Println("keepReceiving", fb.conn)
		ff, err := fb.read()

		if err != nil {
			panic("keepReceiving failed") //for debug
		}

		if ff.msgType == typeSendData {
			seqStatus := bd.seqCheck(ff.id)
			if seqStatus == seqOutOfRange {
				log.Println("[Bundle.keepReceiving.outOfRange]", ff.id)
				continue
			}
			if seqStatus == seqReceived {
				//Only send confirm back, not add this buffer
				log.Println("[Bundle.keepReceiving.dupSeqReceived]", ff.id)
				bd.confirmLock.Lock()
				bd.confirmBuffer = append(bd.confirmBuffer, ff.id)
				bd.confirmLock.Unlock()
				continue
			}
			//seqStatus == seqInRange
			log.Println("[Bundle.keepReceiving]", ff.id)

			bd.receiveLock.Lock()
			bd.receiveBuffer[ff.id] = ff
			bd.receiveLock.Unlock()

			log.Println("[Bundle.keepReceiving.receiveBufferAdded]", ff.id)
			bd.receiveChannel <- empty{}

			bd.confirmLock.Lock()
			bd.confirmBuffer = append(bd.confirmBuffer, ff.id)
			bd.confirmLock.Unlock()
			log.Println("[Bundle.keepReceiving.confirmBufferAdded]", ff.id)
		}
		if ff.msgType == typeDataReceived {
			buf := ff.message

			bd.confirmGotLock.Lock()
			for i := 0; i < len(buf); i += 4 {
				id := binary.BigEndian.Uint32(buf[i : i+4])
				log.Println("[Bundle.keepReceiving.confirmReceived]", id)
				chn, ok := bd.confirmGotSignal[id]
				if ok {
					chn <- empty{}
				}
			}
			bd.confirmGotLock.Unlock()
		}
		if ff.msgType == typeHeartbeat {
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
					atomic.AddUint32(&bd.seqs[download], 1)
					bd.callbackLock.RLock()
					if bd.onReceived != nil {
						bd.onReceived(bd.id, ff.message)
						log.Println("[keepForwarding]", ff.id)
					}
					bd.callbackLock.RUnlock()
				} else {
					//log.Println("[Step  9--] waiting for ", seq)
					break
				}
			}
			bd.receiveLock.Unlock()

		case <-bd.closeChan:
			bd.closeChan <- empty{}
			break
		}
	}
}

func (bd *FiberBundle) keepDebugging() {
	/*for {
		select {
		case <-time.After(time.Second * 1):
			log.Println("bd", bd.id, "len:", bd.GetSize(), len(bd.sendTokens))
			//bd.confirmLock

		case <-bd.closeChan:
			bd.closeChan <- empty{}
			break
		}
	}*/
}
