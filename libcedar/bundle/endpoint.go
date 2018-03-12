package bundle

import (
	"net"
)

type Endpoint struct {
	bundles      map[uint32]*FiberBundle
	bufferLen    uint32
	password     string
	addr         string
	endpointType string

	mbd *FiberBundle

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

func (ep *Endpoint) Write(id uint32, message []byte) {
	//log.Println("[Step  1]", len(message))
	//nmessage := make([]byte, lonReceiveden(message))
	//copy(nmessage, message)
	if id == 0 {
		//TODO: bug when mbd is not prepared
		ep.mbd.SendMessage(message)
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
