package bundle

import (
	"fmt"
	"net"
	"testing"
	"time"
)

const (
	magicID  = 42
	msgCount = 100
)

var msgBundleBasic = make(chan []byte, 100)

func dataReceivedBundleBasic(id uint32, message []byte) {
	if id != magicID {
		panic("id should be 42")
	}
	msgBundleBasic <- message
}

func TestBundleBasic(t *testing.T) {
	addr := "127.0.0.1:22350"
	message := "Cooool! Awesome!"

	lst, _ := net.Listen("tcp", addr)
	ch := make(chan net.Conn, 10)
	go func() {
		c, _ := lst.Accept()
		ch <- c
	}()
	connClt, _ := net.Dial("tcp", addr)
	connSvr := <-ch

	hsrS := HandshakeResult{magicID, 1000000, 4000000, connSvr}
	hsrC := HandshakeResult{magicID, 1000000, 4000000, connClt}
	encryptor := NewCedarCryptoIO("12345")

	bdS := NewFiberBundle(20, "server", &hsrS)
	bdC := NewFiberBundle(20, "client", &hsrC)

	NewFiber(hsrS.conn, encryptor, bdS)
	NewFiber(hsrC.conn, encryptor, bdC)

	bdC.SetOnReceived(dataReceivedBundleBasic)
	bdS.SetOnReceived(dataReceivedBundleBasic)

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
		case <-time.After(90 * time.Second):
			panic("no message got and test failed")
		}
	}
}
