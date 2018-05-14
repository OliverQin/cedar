package bundle

import (
	"fmt"
	"net"
	"testing"
	"time"
)

const (
	magicID = 42
)

var msgBundleBasic = make(chan []byte, 100)

func dataReceivedBundleBasic(id uint32, message []byte) {
	if id != magicID {
		panic("id should be 42")
	}
	msgBundleBasic <- message
}

func localConnPairs(addr string, num int) []net.Conn {
	ret := make([]net.Conn, num*2)

	lst, _ := net.Listen("tcp", addr)
	ch := make(chan net.Conn, 10)

	for i := 0; i < num; i++ {
		go func() {
			c, _ := lst.Accept()
			ch <- c
		}()
		connClt, _ := net.Dial("tcp", addr)
		connSvr := <-ch

		ret[i] = connSvr
		ret[i+num] = connClt
	}
	return ret
}

func testOne(addr string, num int, bufSize uint32, msgCount int) {
	message := "Cooool! Awesome!"

	conns := localConnPairs(addr, num)

	hsrS := HandshakeResult{magicID, 1000000, 4000000, conns[0]}
	hsrC := HandshakeResult{magicID, 1000000, 4000000, conns[num]}
	encryptor := NewCedarCryptoIO("12345")

	bdS := NewFiberBundle(bufSize, "server", &hsrS)
	bdC := NewFiberBundle(bufSize, "client", &hsrC)

	for i := 0; i < num; i++ {
		NewFiber(conns[i], encryptor, bdS)
		NewFiber(conns[i+num], encryptor, bdC)

		bdS.SetOnReceived(dataReceivedBundleBasic)
		bdC.SetOnReceived(dataReceivedBundleBasic)
	}
	go func() {
		for i := 0; i < msgCount; i++ {
			bdC.SendMessage([]byte(message))
		}
	}()

	go func() {
		for i := 0; i < msgCount; i++ {
			bdS.SendMessage([]byte(message))
		}
	}()

	for i := 0; i < msgCount*2; i++ {
		select {
		case x := <-msgBundleBasic:
			fmt.Println("Message:", x)
			if string(x) != message {
				panic("data error")
			}
		case <-time.After(10 * time.Second):
			panic("no message got and test failed")
		}
	}

	select {
	case <-msgBundleBasic:
		panic("too much message in channel")
	default:
		//pass
	}
}

func TestBundleMini(t *testing.T) {
	testOne("127.0.0.1:20001", 1, 1, 20)
}

func TestBundleBasic(t *testing.T) {
	testOne("127.0.0.1:20001", 1, 30, 100)
}

func TestBundleParallel(t *testing.T) {
	testOne("127.0.0.1:20001", 3, 1, 100)
}
