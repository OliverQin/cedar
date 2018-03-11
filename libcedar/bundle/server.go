package bundle

import (
	"fmt"
	"net"
	"os"
)

type Endpoint struct {
	bundles      map[uint32]*FiberBundle
	bufferLen    uint32
	password     string
	addr         string
	endpointType string

	onReceived *FuncDataReceived
	mbd        *FiberBundle
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

	callback := func(id uint32, message []byte) {
		fn := fmt.Sprint(id) + "_rec.bin"
		//log.Println("printing...")
		f, _ := os.OpenFile(fn, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
		f.Write([]byte(message))
		f.Close()
	}
	//type FuncDataReceived func(id uint32, message []byte)
	n.onReceived = (*FuncDataReceived)(&callback)

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
			//log.Println("server new fiber...", bd.id)
			id, err := bd.addConnection(conn)
			if err != nil {
				continue
			}

			nbd, ok := ep.bundles[id]
			//log.Println(id, "adding...")
			if ok {
				nbd.addAndReceive(conn)
			} else {
				ep.bundles[id] = bd
				ep.mbd = ep.bundles[bd.id]
				ep.mbd.SetOnReceived(ep.onReceived)
				ep.bundles[bd.id].addAndReceive(conn)
				ep.bundles[0] = NewFiberBundle(ep.bufferLen, ep.endpointType, ep.password)
			}
		}
		//log.Println("accepted,", i)
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
		_, err = bd.addConnection(conn)
		//log.Println("bundle add", err, bd.id)
		bd.addAndReceive(conn)

		ep.bundles[bd.id] = bd
		ep.mbd = ep.bundles[bd.id]
		ep.mbd.SetOnReceived(ep.onReceived)
		if ep.bundles[0] == bd {
			ep.bundles[0] = NewFiberBundle(ep.bufferLen, ep.endpointType, ep.password)
			//log.Println("client replaced")
		}
	}
}

func (ep *Endpoint) Write(message []byte) {
	//log.Println("[Step  1]", len(message))
	//nmessage := make([]byte, len(message))
	//copy(nmessage, message)
	ep.mbd.SendMessage(message)
}
