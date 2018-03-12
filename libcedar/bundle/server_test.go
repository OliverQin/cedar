package bundle

import (
	"crypto/sha512"
	"encoding/hex"
	"hash"
	"log"
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
const numOfClients = 5
const testFileLength = 2 * 1024 * 1024
const bufSize = 2048
const fileSha512 = "f8228ab81fa60c2db4bc7f1ad9b5c8f96de4df2a2c2498772d223e1a84c1836e14637f2487536d24bd2fc3bd838121c50fe5d95c5360b337b7a309601cb94188"

var finishChannel = make(chan int, numOfClients)

var hashLock sync.Mutex
var hashers = make(map[uint32]hash.Hash)
var lengthMap = make(map[uint32]int)

func callback(id uint32, message []byte) {
	hashLock.Lock()
	defer hashLock.Unlock()

	_, ok := hashers[id]
	if !ok {
		hashers[id] = sha512.New()
		lengthMap[id] = 0
	}

	hashers[id].Write(message)
	lengthMap[id] += len(message)
	//log.Println(len(message), id, lengthMap[id])
	if lengthMap[id] == testFileLength {
		hashRet := hex.EncodeToString(hashers[id].Sum(nil))
		log.Println("file", id, "=", hashRet)
		if hashRet != fileSha512 {
			panic("hash does not match, something implemented goes wrong")
		}
		finishChannel <- 0
	}
}

func StartServer() {
	sv := NewEndpoint(500, "server", serverAddr, "test")
	sv.SetOnReceived(callback)
	sv.ServerStart()
}

func StartClient() {
	clts := make([]*Endpoint, 0)

	for i := 0; i < numOfClients; i++ {
		clt := NewEndpoint(500, "client", serverAddr, "test")
		//clt.SetOnReceived(callback)
		clt.CreateConnection(10)
		clts = append(clts, clt)
	}

	fi := newPseudoRandomFile()
	for j := 0; j < testFileLength/bufSize; j++ {
		buf := make([]byte, bufSize)
		fi.Read(buf)

		for i := 0; i < numOfClients; i++ {
			clts[i].Write(0, buf)
		}
	}

	time.Sleep(time.Millisecond * 500)
}

func TestMultiClientLoopback(t *testing.T) {
	SetGlobalResend(2000 * time.Millisecond)

	go StartServer()
	time.Sleep(500 * time.Millisecond)

	go StartClient()

	for i := 0; i < numOfClients; i++ {
		<-finishChannel
	}
}
