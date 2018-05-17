package bundle

import (
	"log"
	"sync/atomic"
)

const (
	seqInRange = iota + 1
	seqReceived
	seqOutOfRange
)

var debugVerbose = 0

func innerLog(level int, a ...interface{}) {
	if debugVerbose >= level {
		log.Println(a)
	}
}

//LogDebug prints info
func LogDebug(a ...interface{}) {
	innerLog(10, a)
}

//LogInfo prints info
func LogInfo(a ...interface{}) {
	innerLog(0, a)
}

func inRange(seq, start, end uint32) bool {
	if start < end {
		return (start <= seq && seq < end)
	}
	//start > end, overflowed case
	return (seq >= start) || (seq < end)
}

func (bd *FiberBundle) seqCheck(packetId uint32) int {
	seqA := atomic.LoadUint32(&bd.seqs[download])
	seqB := seqA + bd.bufferLen

	//log.Println("Range: ", seqA, seqB, packetId)
	if inRange(packetId, seqA, seqB) {
		return seqInRange
	}

	DeltaReceived := uint32(100000)
	if inRange(packetId, seqA-DeltaReceived, seqA) {
		return seqReceived
	}

	return seqOutOfRange
}
