// +build !race

package bundle

import (
	"testing"
	"time"
)

func TestBundleIntense(t *testing.T) {
	//TODO: fix this RACE and make the test more intensive
	globalResend = time.Millisecond * 3000
	globalMinHeartbeat = time.Millisecond * 800
	globalMaxHeartbeat = time.Millisecond * 1000
	globalConfirmWait = time.Millisecond * 1

	testOne("127.0.0.1:20001", 20, 50, 10000)
}
