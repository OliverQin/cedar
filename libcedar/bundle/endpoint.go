package bundle

import (
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
	onFiberLost  FuncFiberLost
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
				LogDebug("Confirm failed:", err)
				return
			}
			LogDebug("[Endpoint.handshaked]", hsr.id)
			bd := ep.bundles.GetBundle(hsr.id)

			if bd == nil {
				bd = NewFiberBundle(ep.bufferLen, "server", &hsr)
				bd.SetOnReceived(ep.onReceived)
				bd.SetOnBundleLost(ep.onBundleLost)
				bd.SetOnFiberLost(ep.onFiberLost)
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
	LogDebug("Connected", ep.addr, conn)

	hsr, err := ep.handshaker.RequestNewBundle(conn)
	LogDebug("request", hsr, err)

	bd := NewFiberBundle(ep.bufferLen, "client", &hsr)
	bd.SetOnReceived(ep.onReceived)
	bd.SetOnBundleLost(ep.onBundleLost)
	bd.SetOnFiberLost(ep.onFiberLost)
	NewFiber(hsr.conn, ep.encryptor, bd)

	err = ep.bundles.AddBundle(bd)
	if err != nil {
		bd.Close(ErrDuplicatedBundle)
		return
	}

	for i := 1; i < numberOfConnections; i++ {
		ep.AddConnection()
	}
}

func (ep *Endpoint) AddConnection() {
	conn, err := net.Dial("tcp", ep.addr)
	if err != nil {
		return
	}
	id := ep.bundles.GetMain().id
	_, err = ep.handshaker.RequestAddToBundle(conn, id)
	if err != nil {
		return
	}
	NewFiber(conn, ep.encryptor, ep.bundles.GetMain())
}

func (ep *Endpoint) Write(id uint32, message []byte) {
	LogDebug("[Endpoint.Write]", ShortHash(message))

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

func (ep *Endpoint) SetOnFiberLost(f FuncFiberLost) {
	ep.onFiberLost = f
}
