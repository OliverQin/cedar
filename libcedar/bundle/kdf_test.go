package bundle

import (
	"encoding/hex"
	"log"
	"testing"
)

func TestCedarKdf(t *testing.T) {
	var k kdf

	k = cedarKdf{}
	token := k.generate("MyPassword", "Cedar_Session", 256)

	res := hex.EncodeToString(token)

	log.Println(res)
	if res != "a332512bca33c1087513a3e026d38a4d9319e27f419f814440a142b4dad40d48" {
		panic("kdf did not get expected result")
	}
}
