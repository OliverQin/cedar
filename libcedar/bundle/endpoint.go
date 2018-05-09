package bundle

import (
	"log"
	"net"
	"sync"
	"time"
)

type Endpoint struct {
	bundles      map[uint32]*FiberBundle
	bufferLen    uint32
	password     string
	addr         string
	endpointType string

	mbd     *FiberBundle
	mbdLock sync.RWMutex

	onReceived FuncDataReceived
}

func NewEndpoint(bufferLen uint32, endpointType string, addr string, password string) *Endpoint {
	n := new(Endpoint)

	n.bufferLen = bufferLen
	n.onReceived = nil
	n.bundles = make(map[uint32]*FiberBundle)
	n.endpointType = endpointType
	n.addr = addr
	n.password = password

	n.bundles[0] = NewFiberBundle(bufferLen, endpointType, password)
	n.mbd = nil

	//type FuncDataReceived func(id uint32, message []byte)
	//n.onReceived = (*FuncDataReceived)(&callback)

	go n.keepCleaning()
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
		if err == nil {
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
				ep.bundles[id] = bd
				ep.mbdLock.Lock()
				ep.mbd = bd
				ep.mbdLock.Unlock()
				ep.mbd.SetOnReceived(ep.onReceived)

				bd.addAndReceive(fb)
				ep.bundles[0] = NewFiberBundle(ep.bufferLen, ep.endpointType, ep.password)

				if ep.bundles[0].id != 0 {
					panic("?")
				}
			}
		}
	}
}

func (ep *Endpoint) keepCleaning() {
	if ep.endpointType != "server" {
		return
	}

	for {
		ep.mbdLock.Lock()
		for i, v := range ep.bundles {
			if v.CloseIfAllFibersClosed() {
				delete(ep.bundles, i)
			}
		}
		ep.mbdLock.Unlock()

		time.Sleep(60 * time.Second)
	}
}

func (ep *Endpoint) CreateConnection(numberOfConnections int) {
	if ep.endpointType != "client" {
		panic("only client can call CreateConnection")
	}

	for i := 0; i < numberOfConnections; i++ {
		conn, err := net.Dial("tcp", ep.addr)
		//log.Println("client new fiber...", conn, err)
		if err != nil {
			continue
		}
		var bd *FiberBundle
		if ep.mbd == nil {
			bd = ep.bundles[0]
		} else {
			bd = ep.mbd
		}
		//log.Println("bundle add..", bd.id)
		_, fb, err := bd.addConnection(conn)
		//log.Println("bundle add", err, bd.id)
		bd.addAndReceive(fb)

		ep.bundles[bd.id] = bd
		ep.mbdLock.Lock()
		ep.mbd = ep.bundles[bd.id]
		ep.mbdLock.Unlock()
		ep.mbd.SetOnReceived(ep.onReceived)
		if ep.bundles[0] == bd {
			ep.bundles[0] = NewFiberBundle(ep.bufferLen, ep.endpointType, ep.password)
			//log.Println("client replaced")
		}
	}
}

func (ep *Endpoint) Write(id uint32, message []byte) {
	log.Println("[Endpoint.Write]", ShortHash(message))
	//nmessage := make([]byte, lonReceiveden(message))
	//copy(nmessage, message)
	if id == 0 {
		//TODO: bug when mbd is not prepared
		ep.mbdLock.RLock()
		x := ep.mbd
		ep.mbdLock.RUnlock()
		x.SendMessage(message)
	} else {
		p, ok := ep.bundles[id]
		if ok {
			p.SendMessage(message)
		} else {
			panic("id does not exist")
		}
	}
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
