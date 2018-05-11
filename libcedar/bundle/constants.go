package bundle

import (
	"errors"
	"time"
)

const (
	typeRequestAllocation = iota + 1
	typeAllocationConfirm
	typeAddNewFiber
	typeFiberAdded
	typeSendData
	typeDataReceived
	typeHeartbeat
)

const (
	upload   = 0
	download = 1
)

var errEmptyBundle = errors.New("empty bundle")

var errAllocationFailed = errors.New("allocation failed")
var errAddingFailed = errors.New("adding failed")

const (
	defaultTimeout     time.Duration = time.Second * 40
	defaultResend      time.Duration = time.Second * 12
	defaultConfirmWait time.Duration = time.Millisecond * 10
)

var globalTimeout = defaultTimeout
var globalResend = defaultResend
var globalConfirmWait = defaultConfirmWait
var globalCleanup = defaultTimeout * 5

const (
	serverBundle uint32 = 1
	clientBundle uint32 = 2
)

func SetGlobalTimeout(duration time.Duration) {
	if duration < 0 {
		duration = 3600 * time.Second //one hour
	}
	globalTimeout = duration
	globalCleanup = globalTimeout * 5
}

func SetGlobalResend(duration time.Duration) {
	if duration < 0 {
		duration = 3600 * time.Second //one hour
	}
	globalResend = duration
}
