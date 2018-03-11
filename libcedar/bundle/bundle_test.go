package bundle

import (
	"hash"
	"sync"
	"testing"
	"time"
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

//const numOfClients = 2

var hashLock sync.Mutex
var hashers = make(map[uint32]hash.Hash)

func StartServer() {
	sv := NewEndpoint(500, "server", serverAddr, "test")
	sv.ServerStart()
}

func StartClient() {
	clt := NewEndpoint(500, "client", serverAddr, "test")
	clt.CreateConnection(10)

	time.Sleep(time.Millisecond * 500)

	fi := newPseudoRandomFile()
	for i := 0; i < 2048; i++ {
		buf := make([]byte, 8192)
		fi.Read(buf)
		clt.Write(buf)
	}

	time.Sleep(time.Millisecond * 500)
}

func TestMultiClientLoopback(t *testing.T) {
	SetGlobalResend(1000 * time.Millisecond)

	go StartServer()
	time.Sleep(500 * time.Millisecond)

	go StartClient()
	time.Sleep(10 * time.Second)

	//if len(hashers) != numOfClients {
	//	log.Panic("num of hashers ", len(hashers), " should be ", numOfClients)
	//}
	//for id, val := range hashers {
	//hashStr := fmt.Sprintf("%x", val.Sum(nil))
	//log.Printf("%d = %x", id, val.Sum(nil))
	//if hashStr != "bedf74b44350af67a4a570195f9ae860cd719ac98b0619a7cfe9fdf1248fa528" {
	//	log.Panic("hash wrong")
	//}
	//}
}
