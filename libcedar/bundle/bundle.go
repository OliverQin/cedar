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

//type FuncBundleCreated func(id uint32)

type FuncDataReceived func(id uint32, message []byte)

type FuncBundleLost func(id uint32)

type FuncFiberLost func(id uint32)

type empty struct{}

var errUnexpectedRequest = errors.New("unexpected request")
var ErrAllFibersLost = errors.New("all fibers lost")

const (
	serverBundle uint32 = 1
	clientBundle uint32 = 2
)

type FiberBundle struct {
	id   uint32
	seqs [2]uint32
	next uint32

	fibersLock sync.RWMutex
	fibers     []*Fiber

	bufferLen      uint32
	receiveLock    sync.RWMutex
	receiveBuffer  map[uint32]*FiberPacket
	receiveChannel chan empty

	sendTokens chan empty //token bucket for sending
	sendLock   sync.RWMutex
	//sendBuffer map[uint32]*FiberPacket

	closeChan chan error
	cleaned   uint32

	confirmGotSignal map[uint32]chan empty
	confirmGotLock   sync.RWMutex

	received *bundleReceivedIDs

	//onBundleCreated FuncBundleCreated
	callbackLock sync.RWMutex
	onReceived   FuncDataReceived
	onFiberLost  FuncFiberLost
	onBundleLost FuncBundleLost
}

func NewFiberBundle(bufferLen uint32, bundleType string, hsr *HandshakeResult) *FiberBundle {
	ret := new(FiberBundle)

	if strings.ToLower(bundleType) == "server" {
		ret.seqs[upload] = hsr.idS2C
		ret.seqs[download] = hsr.idC2S
	} else if strings.ToLower(bundleType) == "client" {
		ret.seqs[upload] = hsr.idC2S
		ret.seqs[download] = hsr.idS2C
	} else {
		panic("bundleType should be either `server` or `client`")
	}

	ret.id = hsr.id
	ret.next = 0

	if bufferLen == 0 {
		bufferLen = 1
	}
	ret.fibers = make([]*Fiber, 0)

	ret.bufferLen = bufferLen
	ret.receiveBuffer = make(map[uint32]*FiberPacket)
	ret.receiveChannel = make(chan empty, bufferLen)
	ret.confirmGotSignal = make(map[uint32]chan empty)

	ret.sendTokens = make(chan empty, bufferLen)
	//ret.sendBuffer = make(map[uint32]*FiberPacket)

	ret.closeChan = make(chan error, 0xff)

	ret.received = newBundleReceivedIDs()

	ret.onReceived = nil
	ret.onFiberLost = nil
	ret.onBundleLost = nil

	go ret.keepConfirming()
	go ret.keepForwarding()

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

func (bd *FiberBundle) GetSize() int {
	bd.fibersLock.RLock()
	x := len(bd.fibers)
	bd.fibersLock.RUnlock()
	return x
}

/*
GetFiberToWrite gets a Fiber to write on, for sending message.
By design, this function is not thread safe.
*/
func (bd *FiberBundle) GetFiberToWrite() *Fiber {
	for {
		bd.fibersLock.Lock()
		x := len(bd.fibers)
		//log.Println("bundle id:", bd.id, "size:", x)
		if x == 0 {
			bd.fibersLock.Unlock()

			select {
			case err := <-bd.closeChan:
				bd.closeChan <- err
				log.Println("[Bundle.GetFiberToWrite] closeChan got")
				return nil
			case <-time.After(time.Second * 1): //1s, wait until there is one connection
				continue
			}
		} else {
			break
		}
	}

	defer bd.fibersLock.Unlock()
	bd.next = (bd.next + 1) % uint32(len(bd.fibers))

	return bd.fibers[bd.next]
}

func (bd *FiberBundle) SendMessage(msg []byte) error {
	bd.sendTokens <- empty{}
	pkt := FiberPacket{
		atomic.AddUint32(&(bd.seqs[upload]), 1) - 1,
		typeSendData,
		msg,
	}

	log.Println("[Bundle.SendMessage] ", pkt.id, ShortHash(msg))
	go bd.keepSending(pkt)

	return nil
}

/*
keepSending would be called once for every message (in goroutine).
It ends until message is sent and confirmed.
*/
func (bd *FiberBundle) keepSending(pkt FiberPacket) {
	bd.confirmGotLock.Lock()
	bd.confirmGotSignal[pkt.id] = make(chan empty, 0xff)
	thisChannel := bd.confirmGotSignal[pkt.id]
	bd.confirmGotLock.Unlock()

	for {
		//log.Println("Geting Fiber", pkt.id, len(pkt.message), pkt.ms+gType)
		fb := bd.GetFiberToWrite()
		if fb == nil {
			goto ended
		}

		log.Println("[Bundle.keepSending.Got]", pkt.id)
		fb.write(pkt)
		log.Println("[Bundle.keepSending.Wrote]", pkt.id)
		//log.Println("[Step  3] pkt.id, len(pkt.msg), pkt.msgType", pkt.id, len(pkt.message), pkt.msgType)

		select {
		case <-time.After(globalResend):
			break
		case <-thisChannel:
			//log.Println("[Step  8] keepSending is closing: id", pkt.id)
			goto ended
		case err := <-bd.closeChan:
			bd.closeChan <- err
			log.Println("[Bundle.keepSending] closeChan got")
			goto ended
		}
	}

ended:
	bd.confirmGotLock.Lock()
	//log.Println("pkt.id sent successfully", pkt.id)
	delete(bd.confirmGotSignal, pkt.id)
	close(thisChannel)
	bd.confirmGotLock.Unlock()

	<-bd.sendTokens
}

func (bd *FiberBundle) keepConfirming() {
	defer log.Println("keepConfirming Stoped", bd)
	for {
		fb := bd.GetFiberToWrite()
		if fb == nil {
			return
		}

		info := bd.received.getMessage()
		if info != nil && len(info) > 0 {
			fb.write(FiberPacket{0, typeDataReceived, info})
			log.Println(time.Now(), "=============", len(info))
		}

		time.Sleep(globalConfirmWait)
	}
}

/*
PacketReceived is called to notify the bundle that a new data packet received.
*/
func (bd *FiberBundle) PacketReceived(pkt *FiberPacket) {
	if pkt.msgType == typeSendData {
		seqStatus := bd.seqCheck(pkt.id)
		if seqStatus == seqOutOfRange {
			log.Println("[Bundle.keepReceiving.outOfRange]", pkt.id)
			return
		}
		if seqStatus == seqReceived {
			//Only send confirm back, not add this buffer
			log.Println("[Bundle.keepReceiving.dupSeqReceived]", pkt.id)
			bd.received.addID(pkt.id)
			return
		}
		//seqStatus == seqInRange
		log.Println("[Bundle.keepReceiving]", pkt.id)

		bd.receiveLock.Lock()
		bd.receiveBuffer[pkt.id] = pkt
		bd.receiveLock.Unlock()

		log.Println("[Bundle.keepReceiving.receiveBufferAdded]", pkt.id)
		bd.receiveChannel <- empty{}

		bd.received.addID(pkt.id)
		log.Println("[Bundle.keepReceiving.confirmBufferAdded]", pkt.id)
	}

	if pkt.msgType == typeDataReceived {
		buf := pkt.message

		bd.confirmGotLock.Lock()
		dupIds := make(map[uint32]bool)
		for i := 0; i < len(buf); i += 4 {
			id := binary.BigEndian.Uint32(buf[i : i+4])
			_, found := dupIds[id]
			if found {
				continue
			}
			dupIds[id] = true
			log.Println("[Bundle.keepReceiving.confirmReceived]", id)
			chn, ok := bd.confirmGotSignal[id]
			if ok {
				chn <- empty{}
			}
		}
		bd.confirmGotLock.Unlock()
	}
}

/*
FiberCreated is called to notify the bundle that a new fiber created.
*/
func (bd *FiberBundle) FiberCreated(fb *Fiber) {
	bd.fibersLock.Lock()
	defer bd.fibersLock.Unlock()

	log.Println("[FiberCreated]", fb)

	for _, v := range bd.fibers {
		if v == fb {
			return
		}
	}
	bd.fibers = append(bd.fibers, fb)
}

/*
FiberClosed is called to notify the bundle that a new fiber closed.
*/
func (bd *FiberBundle) FiberClosed(fb *Fiber) {
	bd.fibersLock.Lock()
	defer bd.fibersLock.Unlock()

	for i, v := range bd.fibers {
		if v == fb {
			bd.fibers = append(bd.fibers[:i], bd.fibers[i+1:]...)
			break
		}
	}

	bd.callbackLock.RLock()
	defer bd.callbackLock.RUnlock()
	if bd.onFiberLost != nil {
		go bd.onFiberLost(bd.id)
	}

	if len(bd.fibers) == 0 {
		go bd.Close(ErrAllFibersLost)
	}
}

func (bd *FiberBundle) Close(err error) {
	bd.fibersLock.Lock()
	defer bd.fibersLock.Unlock()

	if 1 != atomic.AddUint32(&bd.cleaned, 1) {
		return
	}

	for _, v := range bd.fibers {
		v.Close(err)
	}
	bd.fibers = bd.fibers[:0]

	for i := 0; i < 5; i++ {
		bd.closeChan <- err
	}

	bd.callbackLock.RLock()
	defer bd.callbackLock.RUnlock()
	if bd.onBundleLost != nil {
		go bd.onBundleLost(bd.id)
	}
}

func (bd *FiberBundle) keepForwarding() {
	defer log.Println("keepForwarding Stoped", bd)
	for {
		select {
		case <-bd.receiveChannel:
			bd.receiveLock.Lock()
			for {
				seq := bd.seqs[download]
				pkt, ok := bd.receiveBuffer[seq]
				//log.Println("seq, status", seq, ok)
				if ok {
					atomic.AddUint32(&bd.seqs[download], 1)
					bd.callbackLock.RLock()
					if bd.onReceived != nil {
						bd.onReceived(bd.id, pkt.message)
						log.Println("[keepForwarding]", pkt.id)
					}
					bd.callbackLock.RUnlock()
				} else {
					//log.Println("[Step  9--] waiting for ", seq)
					break
				}
			}
			bd.receiveLock.Unlock()

		case err := <-bd.closeChan:
			bd.closeChan <- err
			log.Println("[Bundle.keepForwarding] closeChan got")
			return
		}
	}
}
