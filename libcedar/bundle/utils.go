package bundle

import (
	"sync/atomic"
)

const (
	seqInRange = iota + 1
	seqReceived
	seqOutOfRange
)

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
