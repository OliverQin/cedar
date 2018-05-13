package bundle

import (
	"errors"
	"log"
	"net"
	"sync"
)

var ErrDuplicatedBundle = errors.New("duplicated bundle")

type BundleCollection struct {
	sync.RWMutex
	data map[uint32]*FiberBundle
	main *FiberBundle
}

func NewBundleCollection() *BundleCollection {
	ret := new(BundleCollection)
	ret.data = make(map[uint32]*FiberBundle)
	ret.main = nil
	return ret
}

func (bc *BundleCollection) AddBundle(bd *FiberBundle) error {
	bc.Lock()
	defer bc.Unlock()

	id := bd.id
	_, ok := bc.data[id]
	if ok {
		return ErrDuplicatedBundle
	}
	bc.data[id] = bd
	bc.main = bd

	return nil
}

func (bc *BundleCollection) HasID(id uint32) bool {
	bc.RLock()
	defer bc.RUnlock()
	_, ok := bc.data[id]
	return ok
}

func (bc *BundleCollection) GetBundle(id uint32) *FiberBundle {
	bc.RLock()
	defer bc.RUnlock()
	if id == 0 {
		return bc.main
	}
	v, ok := bc.data[id]
	if ok {
		return v
	}
	return nil
}

func (bc *BundleCollection) HasMain() bool {
	bc.RLock()
	defer bc.RUnlock()
	return bc.main != nil
}

type Endpoint struct {
	bundles   *BundleCollection
	bufferLen uint32

	addr         string
	endpointType string
	encryptor    CryptoIO
	handshaker   *Handshaker

	onReceived FuncDataReceived
}

func NewEndpoint(bufferLen uint32, endpointType string, addr string, password string) *Endpoint {
	n := new(Endpoint)

	n.bufferLen = bufferLen
	n.onReceived = nil
	n.bundles = NewBundleCollection()
	n.endpointType = endpointType
	n.addr = addr
	n.encryptor = NewCedarCryptoIO(password)
	n.handshaker = NewHandshaker(n.encryptor, n.bundles)

	//n.bundles[0] = NewFiberBundle(bufferLen, endpointType, password)
	//n.mbd = nil

	//type FuncDataReceived func(id uint32, message []byte)
	//n.onReceived = (*FuncDataReceived)(&callback)

	//go n.keepCleaning()
	return n
}

/*
ServerStart is a endless loop, keep accepting connections
*/
func (ep *Endpoint) ServerStart() {
	if ep.endpointType != "server" {
		panic("only server can call ServerStart")
	}

	lst, err := net.Listen("tcp", ep.addr)
	if err != nil {
		panic(err)
	}
	for i := 0; true; i++ {
		conn, err := lst.Accept()
		if err != nil {
			continue
		}

		//TODO: parallel here
		hsr, err := ep.handshaker.ConfirmHandshake(conn)
		log.Println("[Endpoint.handshaked]", hsr.id)
		if ep.bundles.HasID(hsr.id) {
			bd := ep.bundles.GetBundle(hsr.id)
			NewFiber(hsr.conn, ep.encryptor, bd)
		} else {
			bd := NewFiberBundle(ep.bufferLen, "server", &hsr)
			bd.SetOnReceived(ep.onReceived)
			ep.bundles.AddBundle(bd)
			NewFiber(hsr.conn, ep.encryptor, bd)
		}

		/*if err == nil {
			bd := ep.bundles[0]
			if bd.id != 0 {
				panic("only new bundle should be used for handshaking")
			}

			id, fb, err := bd.addConnection(conn)
			if err != nil {
				continue
			}
			if id != bd.id {
				panic("after handshake id != bd.id")
			}

			nbd, ok := ep.bundles[id]
			if ok {
				nbd.addAndReceive(fb)
				ep.bundles[0].id = 0
				ep.bundles[0].seqs[download] = 0
				ep.bundles[0].seqs[upload] = 0

			} else {
				ep.mbdLock.Lock()
				ep.bundles[id] = bd
				ep.mbd = bd
				ep.mbdLock.Unlock()
				ep.mbd.SetOnReceived(ep.onReceived)

				bd.addAndReceive(fb)
				ep.bundles[0] = NewFiberBundle(ep.bufferLen, ep.endpointType, ep.password)

				if ep.bundles[0].id != 0 {
					panic("?")
				}
			}
		}*/
	}
}

/*func (ep *Endpoint) keepCleaning() {
	if ep.endpointType != "server" {
		return
	}

	for {
		ep.mbdLock.Lock()
		for i, v := range ep.bundles {
			if i == 0 {
				continue
			}
			if v.CloseIfAllFibersClosed() {
				delete(ep.bundles, i)
			}
		}
		ep.mbdLock.Unlock()

		time.Sleep(60 * time.Second)
	}
}*/

func (ep *Endpoint) CreateConnection(numberOfConnections int) {
	if ep.endpointType != "client" {
		panic("only client can call CreateConnection")
	}

	if numberOfConnections < 1 {
		return
	}

	conn, err := net.Dial("tcp", ep.addr)
	if err != nil {
		return
	}
	log.Println("Connected", ep.addr, conn)

	hsr, err := ep.handshaker.RequestNewBundle(conn)
	log.Println("request", hsr, err)
	bd := NewFiberBundle(ep.bufferLen, "client", &hsr)
	bd.SetOnReceived(ep.onReceived)
	NewFiber(hsr.conn, ep.encryptor, bd)
	err = ep.bundles.AddBundle(bd)
	if err != nil {
		return
	}

	for i := 1; i < numberOfConnections; i++ {
		conn, err = net.Dial("tcp", ep.addr)
		if err != nil {
			return
		}
		NewFiber(conn, ep.encryptor, bd)
	}
}

func (ep *Endpoint) Write(id uint32, message []byte) {
	log.Println("[Endpoint.Write]", ShortHash(message))

	var x *FiberBundle

	x = ep.bundles.GetBundle(id)
	if x == nil {
		panic("write failed because no bundle exists")
	}
	x.SendMessage(message)
	return
}

func (ep *Endpoint) SetOnReceived(f FuncDataReceived) {
	/*if id == 0 {
		ep.mbd.SetOnReceived(f)
	} else {
		p, ok := ep.bundles[id]
		if ok {
			p.SetOnReceived(f)
		} else {
			panic("id does not exist")
		}
	}
	return*/
	ep.onReceived = f
}
