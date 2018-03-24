package bundle

import (
	"crypto/sha512"
	"encoding/hex"
)

func ShortHash(msg []byte) string {
	h := sha512.New()
	h.Write(msg)
	hval := hex.EncodeToString(h.Sum(nil)[:6])
	return hval
	//log.Println("clt Rec:", hval)
}
