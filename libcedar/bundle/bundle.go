package bundle

import (
	"encoding/binary"
	"errors"
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
		//LogDebug("bundle id:", bd.id, "size:", x)
		if x == 0 {
			bd.fibersLock.Unlock()

			select {
			case err := <-bd.closeChan:
				bd.closeChan <- err
				LogDebug("[Bundle.GetFiberToWrite] closeChan got")
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

	LogDebug("[Bundle.SendMessage] ", pkt.id, ShortHash(msg))
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
		//LogDebug("Geting Fiber", pkt.id, len(pkt.message), pkt.ms+gType)
		fb := bd.GetFiberToWrite()
		if fb == nil {
			goto ended
		}

		LogDebug("[Bundle.keepSending.Got]", pkt.id)
		fb.write(pkt)
		LogDebug("[Bundle.keepSending.Wrote]", pkt.id)
		//LogDebug("[Step  3] pkt.id, len(pkt.msg), pkt.msgType", pkt.id, len(pkt.message), pkt.msgType)

		select {
		case <-time.After(globalResend):
			break
		case <-thisChannel:
			//LogDebug("[Step  8] keepSending is closing: id", pkt.id)
			goto ended
		case err := <-bd.closeChan:
			bd.closeChan <- err
			LogDebug("[Bundle.keepSending] closeChan got")
			goto ended
		}
	}

ended:
	bd.confirmGotLock.Lock()
	//LogDebug("pkt.id sent successfully", pkt.id)
	delete(bd.confirmGotSignal, pkt.id)
	close(thisChannel)
	bd.confirmGotLock.Unlock()

	<-bd.sendTokens
}

func (bd *FiberBundle) keepConfirming() {
	for {
		fb := bd.GetFiberToWrite()
		if fb == nil {
			return
		}

		info := bd.received.getMessage()
		if info != nil && len(info) > 0 {
			fb.write(FiberPacket{0, typeDataReceived, info})
			LogDebug(time.Now(), "=============", len(info))
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
			LogDebug("[Bundle.keepReceiving.outOfRange]", pkt.id)
			return
		}
		if seqStatus == seqReceived {
			//Only send confirm back, not add this buffer
			LogDebug("[Bundle.keepReceiving.dupSeqReceived]", pkt.id)
			bd.received.addID(pkt.id)
			return
		}
		//seqStatus == seqInRange
		LogDebug("[Bundle.keepReceiving]", pkt.id)

		bd.receiveLock.Lock()
		bd.receiveBuffer[pkt.id] = pkt
		bd.receiveLock.Unlock()

		LogDebug("[Bundle.keepReceiving.receiveBufferAdded]", pkt.id)
		bd.receiveChannel <- empty{}

		bd.received.addID(pkt.id)
		LogDebug("[Bundle.keepReceiving.confirmBufferAdded]", pkt.id)
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
			LogDebug("[Bundle.keepReceiving.confirmReceived]", id)
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

	LogDebug("[FiberCreated]", fb)

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
	var closedOneFiber = false
	bd.fibersLock.Lock()
	for i, v := range bd.fibers {
		if v == fb {
			bd.fibers = append(bd.fibers[:i], bd.fibers[i+1:]...)
			closedOneFiber = true
			break
		}
	}
	lenFibers := len(bd.fibers)
	bd.fibersLock.Unlock()

	if !closedOneFiber {
		return
	}

	bd.callbackLock.RLock()
	if bd.onFiberLost != nil {
		go bd.onFiberLost(bd.id)
	}
	bd.callbackLock.RUnlock()

	if lenFibers == 0 {
		//Wait three minutes, if no new fibers created, close this channel
		go func() {
			time.Sleep(3 * time.Minute) //TODO: make a constant here
			bd.fibersLock.Lock()
			lenNow := len(bd.fibers)
			bd.fibersLock.Unlock()
			if lenNow == 0 {
				bd.Close(ErrAllFibersLost)
			}
		}()
	}
}

func (bd *FiberBundle) IsClosed() bool {
	return atomic.LoadUint32(&bd.cleaned) > 0
}

func (bd *FiberBundle) Close(err error) {
	if 1 != atomic.AddUint32(&bd.cleaned, 1) {
		return
	}

	bd.fibersLock.Lock()
	for _, v := range bd.fibers {
		v.Close(err)
	}
	bd.fibers = bd.fibers[:0]
	bd.fibersLock.Unlock() //NOTE: cannot defer, otherwise FiberClosed gets stuck

	for i := 0; i < 5; i++ {
		bd.closeChan <- err
	}

	bd.callbackLock.RLock()
	if bd.onBundleLost != nil {
		go bd.onBundleLost(bd.id)
	}
	bd.callbackLock.RUnlock()
}

func (bd *FiberBundle) keepForwarding() {
	for {
		select {
		case <-bd.receiveChannel:
			bd.receiveLock.Lock()
			for {
				seq := bd.seqs[download]
				pkt, ok := bd.receiveBuffer[seq]
				//LogDebug("seq, status", seq, ok)
				if ok {
					atomic.AddUint32(&bd.seqs[download], 1)
					bd.callbackLock.RLock()
					if bd.onReceived != nil {
						bd.onReceived(bd.id, pkt.message)
						LogDebug("[keepForwarding]", pkt.id)
					}
					bd.callbackLock.RUnlock()
				} else {
					//LogDebug("[Step  9--] waiting for ", seq)
					break
				}
			}
			bd.receiveLock.Unlock()

		case err := <-bd.closeChan:
			bd.closeChan <- err
			LogDebug("[Bundle.keepForwarding] closeChan got")
			return
		}
	}
}
