package bundle

import (
	"crypto/sha512"
	"encoding/hex"
)

/*
ShortHash returns a short hash string of message.
It starts with "sh" and hex form of hash.
*/
func ShortHash(msg []byte) string {
	h := sha512.New()
	h.Write(msg)
	hval := "sh" + hex.EncodeToString(h.Sum(nil)[:6])
	return hval
	//LogDebug("clt Rec:", hval)
}
