package main

import (
	"fmt"
	"log"
	"time"

	"github.com/OliverQin/cedar/libcedar/bundle"
	"github.com/OliverQin/cedar/libcedar/socks"
)

type pseudoRandomFile struct {
	status uint64
}

func newPseudoRandomFile() pseudoRandomFile {
	return pseudoRandomFile{1}
}

func (f *pseudoRandomFile) Read(buf []byte) {
	for i := 0; i < len(buf); i++ {
		f.status = f.status * 15854921139867792121
		f.status ^= (f.status >> 41)
		f.status ^= (f.status >> 63)

		buf[i] = byte(f.status & 0xff)
	}
}

const serverAddr = "127.0.0.1:64338"
const proxyAddr = "127.0.0.1:1082"

var ssServer = socks.NewServer()
var ssClient = socks.NewClient(proxyAddr)
var bdServer = bundle.NewEndpoint(500, "server", serverAddr, "test_password")
var bdClient = bundle.NewEndpoint(500, "client", serverAddr, "test_password")

func callbackSvr(id uint32, message []byte) {
	log.Println("svr Rec:", bundle.ShortHash(message))
	ssServer.WriteCommand(message)
}

func callbackClt(id uint32, message []byte) {
	log.Println("clt Rec:", bundle.ShortHash(message))
	ssClient.WriteCommand(message)
}

func cmdGenSvr(message []byte) error {
	log.Println("svr Gen:", bundle.ShortHash(message))
	bdServer.Write(0, message)
	return nil
}

func cmdGenClt(message []byte) error {
	log.Println("clt Gen:", bundle.ShortHash(message))
	bdClient.Write(0, message)
	return nil
}

func StartServer() {
	bdServer.SetOnReceived(callbackSvr)
	go bdServer.ServerStart()
}

func StartClient() {
	bdClient.SetOnReceived(callbackClt)
	bdClient.CreateConnection(10)

	time.Sleep(time.Millisecond * 500)
}

type logWriter struct {
}

func (writer logWriter) Write(bytes []byte) (int, error) {
	//return fmt.Print(time.Now().UTC().Format("2006-01-02T15:04:05.999Z") + " [DEBUG] " + string(bytes))
	return fmt.Printf("%.5f [Debug] %s", float64(time.Now().UnixNano())/1e9, bytes)
}

func main() {
	log.SetFlags(0)
	log.SetOutput(new(logWriter))

	ssServer.OnCommandGenerated = cmdGenSvr
	ssClient.OnCommandGenerated = cmdGenClt

	bundle.SetGlobalResend(2000 * time.Millisecond)

	go StartServer()
	time.Sleep(500 * time.Millisecond)

	go StartClient()
	time.Sleep(500 * time.Millisecond)

	ssClient.StartAsync()

	/*for i := 0; i < numOfClients; i++ {
		<-finishChannel
	}
	serverSend <- 1
	for i := 0; i < numOfClients; i++ {
		<-finishChannel
	}*/
	time.Sleep(500 * time.Second)
}
