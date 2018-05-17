package bundle

import (
	"log"
	"net"
)

type Endpoint struct {
	bundles   *BundleCollection
	bufferLen uint32

	addr         string
	endpointType string
	encryptor    CryptoIO
	handshaker   *Handshaker

	onReceived   FuncDataReceived
	onBundleLost FuncBundleLost
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

		go func() {
			hsr, err := ep.handshaker.ConfirmHandshake(conn)
			if err != nil {
				log.Println("Confirm failed:", err)
				return
			}
			log.Println("[Endpoint.handshaked]", hsr.id)
			bd := ep.bundles.GetBundle(hsr.id)

			if bd == nil {
				bd = NewFiberBundle(ep.bufferLen, "server", &hsr)
				bd.SetOnReceived(ep.onReceived)
				bd.SetOnBundleLost(ep.onBundleLost)
				ep.bundles.AddBundle(bd)
			}
			NewFiber(hsr.conn, ep.encryptor, bd)
		}()
	}
}

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
		bd.Close(ErrDuplicatedBundle)
		return
	}

	for i := 1; i < numberOfConnections; i++ {
		conn, err = net.Dial("tcp", ep.addr)
		if err != nil {
			continue
		}
		_, err := ep.handshaker.RequestAddToBundle(conn, hsr.id)
		if err != nil {
			continue
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
	ep.onReceived = f
}

func (ep *Endpoint) SetOnBundleLost(f FuncBundleLost) {
	ep.onBundleLost = f
}
