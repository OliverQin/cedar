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
	typeHeartBeat
)

const (
	upload   = 0
	download = 1
)

var errEmptyBundle = errors.New("empty bundle")

var errAllocationFailed = errors.New("allocation failed")
var errAddingFailed = errors.New("adding failed")

const (
	defaultTimeout     time.Duration = time.Second * 20
	defaultResend      time.Duration = time.Second * 1
	defaultConfirmWait time.Duration = time.Millisecond * 10
)

var globalTimeout time.Duration = defaultTimeout
var globalResend time.Duration = defaultResend
var globalConfirmWait time.Duration = defaultConfirmWait

const (
	serverBundle uint32 = 1
	clientBundle uint32 = 2
)

func SetGlobalTimeout(duration time.Duration) {
	if duration < 0 {
		duration = 3600 * time.Second //one hour
	}
	globalTimeout = duration
}

func SetGlobalResend(duration time.Duration) {
	if duration < 0 {
		duration = 3600 * time.Second //one hour
	}
	globalResend = duration
}
