/*
Package socks of libcedar provides a minimal implementation of SOCKS tunnel.

It contains two types of endpoint: Server and Client.
Typically, Client works locally, listening for SOCKS requests from applications like browsers.
While Server works on another machine, getting command from Client, connecting to the remote, sending data back, etc.

Server and Client are communicated via commands (see the source for the protocol used).

When one endpoint generates command, it should be fetched to the other endpoint, like this:
	ssServer := NewServer()
	ssClient := NewClient("localhost:1080")

	ssServer.OnCommandGenerated = ssClient.WriteCommand
	ssClient.OnCommandGenerated = ssServer.WriteCommand

Known issues:

Features like authentication are not supported (yet).
*/
package socks

import (
	"encoding/binary"
	"errors"
	"io"
	"log"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
)

/*
CommandGeneratedFunc is type of the callback function.
The function is called when an endpoint generates command.
This would happen right after data received, connection lost, connection created, etc.
*/
type CommandGeneratedFunc (func([]byte) error)

/*
Endpoint is a SOCKS tunnel proxy's endpoint.
*/
type Endpoint struct {
	OnCommandGenerated CommandGeneratedFunc

	idCounter uint32
	conns     map[uint16]net.Conn
	connsLock sync.RWMutex
	config    endpointConfig

	address string
}

/*
endpointConfig is configuration struct to create SOCKS server/client.

BufferLength: the length of data buffer.
EndpointType: either Server or Client
*/
type endpointConfig struct {
	BufferLength int
	EndpointType int
}

const defaultBufferLength = 8192

/*
server and client are types of Endpoint.
*/
const (
	server = 1 + iota
	client
)

/*
NewServer creates a new SOCKS server.
*/
func NewServer() *Endpoint {
	ret := new(Endpoint)

	ret.OnCommandGenerated = nil

	ret.idCounter = 0
	ret.conns = make(map[uint16]net.Conn)
	ret.config = endpointConfig{defaultBufferLength, server}

	ret.address = ""

	return ret
}

/*
NewClient creates a new SOCKS client.
Address addr should be like "host:port" such as "127.0.0.1:8080".
*/
func NewClient(addr string) *Endpoint {
	ret := new(Endpoint)

	ret.OnCommandGenerated = nil

	ret.idCounter = 0
	ret.conns = make(map[uint16]net.Conn)
	ret.config = endpointConfig{defaultBufferLength, client}

	ret.address = addr

	return ret
}

/*
ErrSocksFailure is error indicating general failures in SOCKS.
*/
var ErrSocksFailure = errors.New("socks failed")

/*
ErrSocksNotSupported is error indicating the command is not supported (yet).
*/
var ErrSocksNotSupported = errors.New("socks command not supported")

const (
	cmdConnectTCP = uint8(1 + iota)
	cmdConnectUDP // FIXME: not supported yet
	cmdBindTCP    // FIXME: not supported yet
	cmdSend
	cmdClose
)

/*
Protocol:
    [cmdConnect 1B][id 2B][port 2B][domain name xB]
    [cmdSend    1B][id 2B][data xB]
    [cmdClose   1B][id 2B]
*/

func (edp *Endpoint) yield(msg []byte) error {
	if edp.OnCommandGenerated != nil {
		(edp.OnCommandGenerated)(msg)
	}
	return nil
}

func (edp *Endpoint) keepReading(conn net.Conn, id uint16) {
	for {
		buf := make([]byte, edp.config.BufferLength+3)

		n, err := conn.Read(buf[3:])
		buf[0] = cmdSend
		binary.BigEndian.PutUint16(buf[1:3], id)
		if n > 0 {
			edp.yield(buf[:3+n])
		}
		if err != nil {
			edp.yieldClose(id)
			break
		}
	}
	edp.removeConnection(id)

	return
}

/*
WriteCommand would write command to the endpoint.
Use it as a callback.
*/
func (edp *Endpoint) WriteCommand(msg []byte) error {
	//TODO: Here data is assumed legal. No checking beforehand. Potential security issue.
	cmd := msg[0]
	id := binary.BigEndian.Uint16(msg[1:3])

	switch cmd {
	case cmdConnectTCP:
		if edp.config.EndpointType != server {
			panic("only server can create connections")
		}

		port := int(binary.BigEndian.Uint16(msg[3:5]))
		host := string(msg[5:])

		newConn, err := net.Dial("tcp", host+":"+strconv.Itoa(port))
		if err == nil {
			edp.addConnection(newConn, id)
			go edp.keepReading(newConn, id)
		} else {
			edp.yieldClose(id)
			edp.removeConnection(id)
		}

	case cmdSend:
		edp.connsLock.RLock()
		val, ok := edp.conns[id]
		edp.connsLock.RUnlock()
		if ok {
			val.Write(msg[3:])
		} else {
			//TODO: this is problem
		}

	case cmdClose:
		edp.connsLock.RLock()
		val, ok := edp.conns[id]
		edp.connsLock.RUnlock()
		if ok {
			val.Close()
			edp.removeConnection(id)
		}
	}

	return nil
}

func (edp *Endpoint) addConnection(conn net.Conn, id uint16) {
	edp.connsLock.Lock()
	edp.conns[id] = conn
	edp.connsLock.Unlock()
}

func (edp *Endpoint) removeConnection(id uint16) {
	edp.connsLock.Lock()
	if _, ok := edp.conns[id]; ok {
		delete(edp.conns, id)
	}
	edp.connsLock.Unlock()
}

func (edp *Endpoint) yieldClose(id uint16) {
	closeBuf := make([]byte, 3)
	closeBuf[0] = cmdClose
	binary.BigEndian.PutUint16(closeBuf[1:3], id)
	edp.yield(closeBuf)
}

/*
HandleConnection tries to take over a tcp connection, do the handshake, transfer data and finally exits.
It should be called only from the client's side.
The function is sync, which means it blocks until the connection is finally closed.
Returns ErrSocksFailure or ErrSocksNotSupported when errors occur during handshake.
Returns nil if handshake succeeded but errors (include EOF) occur in underlying socket.
*/
func (edp *Endpoint) HandleConnection(conn net.Conn) error {
	if edp.config.EndpointType != client {
		panic("only client can handle connection")
	}

	id := uint16(0)
	for id == 0 {
		id = uint16(atomic.AddUint32(&edp.idCounter, 1) & 0xffff)
	}
	edp.addConnection(conn, id)
	defer edp.removeConnection(id)
	defer conn.Close()

	//Part 1: Start authentication
	//0x05, numOfAuthentications, [Auth1, Auth2, ...]
	handshakeBuf := make([]byte, 2048)
	n, _ := io.ReadFull(conn, handshakeBuf[:2])
	if (n != 2) || (handshakeBuf[0] != 0x5) {
		conn.Close()
		return ErrSocksFailure
	}
	numAuth := int(uint8(handshakeBuf[1]))
	n, _ = io.ReadFull(conn, handshakeBuf[2:2+numAuth])
	conn.Write([]byte("\x05\x00")) //version 5, no auth

	//Part 2: Addr
	n, _ = conn.Read(handshakeBuf)
	//Here we assume info could be read in one shot
	if (n < 5) || handshakeBuf[0] != 0x5 || handshakeBuf[2] != 0x0 {
		conn.Write([]byte("\x05\x01\x00\x01\xff\xff\xff\xff\x00\x00"))
		conn.Close()
		return ErrSocksFailure
	}
	//0x05(SOCKS5), 0x01(TCP connect), 0x00(reserve), 0x0?(type)

	switch handshakeBuf[1] { //Command
	case 0x1: //TCP connection
		var addrStr string
		var portBuf []byte

		switch handshakeBuf[3] { //Addr type
		case 0x1: //IPv4
			addr := make(net.IP, 4)
			copy(addr, handshakeBuf[4:8])
			addrStr = addr.String()
			portBuf = handshakeBuf[8:10]
		case 0x4: //IPv6
			addr := make(net.IP, 16)
			copy(addr, handshakeBuf[4:20])
			addrStr = addr.String()
			portBuf = handshakeBuf[20:22]
		case 0x3: //String domain name
			strLen := uint8(handshakeBuf[4])
			addrStr = string(handshakeBuf[5 : 5+strLen])
			portBuf = handshakeBuf[5+strLen : 7+strLen]
		default:
			conn.Write([]byte("\x05\x07\x00\x01\xff\xff\xff\xff\x00\x00")) //not support
			conn.Close()
			return ErrSocksNotSupported
		}

		msgBuf := make([]byte, 1+2+2+len(addrStr))
		msgBuf[0] = cmdConnectTCP
		binary.BigEndian.PutUint16(msgBuf[1:3], id)
		copy(msgBuf[3:5], portBuf)
		copy(msgBuf[5:], []byte(addrStr))
		edp.yield(msgBuf)

	default:
		conn.Write([]byte("\x05\x07\x00\x01\xff\xff\xff\xff\x00\x00"))
		conn.Close()
		return ErrSocksNotSupported
	}

	//FIXME: reply here not indicating the right addr and port
	conn.Write([]byte("\x05\x00\x00\x01\x00\x00\x00\x00\xff\xff")) //granted

	edp.keepReading(conn, id)

	return nil
}

/*
StartAsync starts the client. It opens the socket and start accepting connections.
Returns nil if service started, error otherwise.
*/
func (edp *Endpoint) StartAsync() error {
	lst, err := net.Listen("tcp", edp.address)
	if err != nil {
		log.Fatalln("StartAsync failed,", err)
		return err
	}

	go func() {
		for {
			conn, err := lst.Accept()
			log.Println("Accepted", conn.LocalAddr(), conn.RemoteAddr())

			if err == nil {
				go edp.HandleConnection(conn)
			} else {
				log.Println("Error happens while accepting:", err)
			}
		}
	}()

	return nil
}
