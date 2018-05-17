package bundle

import (
	"encoding/binary"
	"sync"
)

type bundleReceivedIDs struct {
	confirmBuffer []uint32 //store ids to confirm
	confirmLock   sync.RWMutex
}

func newBundleReceivedIDs() *bundleReceivedIDs {
	ret := new(bundleReceivedIDs)
	ret.confirmBuffer = make([]uint32, 0, 128)
	return ret
}

func (lst *bundleReceivedIDs) addID(id uint32) {
	lst.confirmLock.Lock()
	for _, v := range lst.confirmBuffer {
		if v == id {
			return
		}
	}
	lst.confirmBuffer = append(lst.confirmBuffer, id)
	lst.confirmLock.Unlock()
	return
}

/*
generateConfirmInfo returns nil when no IDs in list
*/
func (lst *bundleReceivedIDs) getMessage() []byte {
	lst.confirmLock.Lock()
	defer lst.confirmLock.Unlock()

	ret := make([]byte, len(lst.confirmBuffer)*4)
	for i := 0; i < len(ret); i += 4 {
		binary.BigEndian.PutUint32(ret[i:i+4], lst.confirmBuffer[i/4])
		LogDebug("[keepConfirming.confirmSent]", lst.confirmBuffer[i/4])
	}
	lst.confirmBuffer = lst.confirmBuffer[:0]

	return ret
}
