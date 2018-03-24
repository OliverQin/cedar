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
const numOfClients = 6
const testFileLength = 2 * 1024 * 1024
const bufSize = 1023
const fileSha512 = "f8228ab81fa60c2db4bc7f1ad9b5c8f96de4df2a2c2498772d223e1a84c1836e14637f2487536d24bd2fc3bd838121c50fe5d95c5360b337b7a309601cb94188"

var finishChannel = make(chan int, numOfClients)
var serverSend = make(chan int, 1)

var hashLockSvr sync.Mutex
var hashersSvr = make(map[uint32]hash.Hash)
var hashLockClt sync.Mutex
var hashersClt = make(map[uint32]hash.Hash)
var lengthMap = make(map[uint32]int)

func callbackSvr(id uint32, message []byte) {
	hashLockSvr.Lock()
	defer hashLockSvr.Unlock()

	_, ok := hashersSvr[id]
	if !ok {
		log.Println("file", id, "created.")
		hashersSvr[id] = sha512.New()
		lengthMap[id] = 0
	}

	hashersSvr[id].Write(message)
	lengthMap[id] += len(message)
	//log.Println(len(message), id, lengthMap[id])
	if lengthMap[id] == testFileLength {
		hashRet := hex.EncodeToString(hashersSvr[id].Sum(nil))
		log.Println("file svr", id, "=", hashRet)
		if hashRet != fileSha512 {
			panic("hash does not match, something implemented goes wrong")
		}
		finishChannel <- 0
	}
}

func callbackClt(id uint32, message []byte) {
	hashLockClt.Lock()
	defer hashLockClt.Unlock()

	_, ok := hashersClt[id]
	if !ok {
		log.Println("file clt", id, "created.")
		hashersClt[id] = sha512.New()
		lengthMap[id] = 0
	}

	hashersClt[id].Write(message)
	lengthMap[id] += len(message)
	//log.Println(len(message), id, lengthMap[id])
	if lengthMap[id] == testFileLength {
		hashRet := hex.EncodeToString(hashersClt[id].Sum(nil))
		log.Println("file", id, "=", hashRet)
		if hashRet != fileSha512 {
			panic("hash does not match, something implemented goes wrong")
		}
		finishChannel <- 0
	}
}

func StartServer() {
	sv := NewEndpoint(500, "server", serverAddr, "test")
	sv.SetOnReceived(callbackSvr)
	go sv.ServerStart()

	_ = <-serverSend
	log.Println("Server start sending...")

	fi := newPseudoRandomFile()
	for j := 0; j < testFileLength; j += bufSize {
		rbufSize := bufSize
		if testFileLength-j < rbufSize {
			rbufSize = testFileLength - j
		}
		buf := make([]byte, rbufSize)
		fi.Read(buf)

		for id := range hashersSvr {
			sv.Write(id, buf)
		}
	}
}

func StartClient() {
	clts := make([]*Endpoint, 0)

	for i := 0; i < numOfClients; i++ {
		clt := NewEndpoint(500, "client", serverAddr, "test")
		clt.SetOnReceived(callbackClt)
		clt.CreateConnection(10)
		clts = append(clts, clt)
	}

	log.Println("Client start sending...")
	fi := newPseudoRandomFile()
	for j := 0; j < testFileLength; j += bufSize {
		rbufSize := bufSize
		if testFileLength-j < rbufSize {
			rbufSize = testFileLength - j
		}
		buf := make([]byte, rbufSize)
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
	serverSend <- 1
	for i := 0; i < numOfClients; i++ {
		<-finishChannel
	}
}
