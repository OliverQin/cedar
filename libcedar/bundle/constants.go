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
	defaultTimeout     time.Duration = time.Second * 60
	defaultResend      time.Duration = time.Second * 15
	defaultConfirmWait time.Duration = time.Millisecond * 1
)

/*
GlobalConnectionTimeout is timeout for a connection.
This connection would be closed if any read/write operation did not return after this timeout, or if connection did not get any packet in this period of time.
*/
var GlobalConnectionTimeout = defaultTimeout

var globalResend = defaultResend
var globalMinHeartbeat = time.Second * 10
var globalMaxHeartbeat = defaultResend
var globalConfirmWait = defaultConfirmWait

func SetGlobalTimeout(duration time.Duration) {
	if duration < 0 {
		duration = 3600 * time.Second //one hour
	}
	GlobalConnectionTimeout = duration
}

func SetGlobalResend(duration time.Duration) {
	if duration < 0 {
		duration = 3600 * time.Second //one hour
	}
	globalResend = duration
}
