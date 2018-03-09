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

	res := make([]byte, 64)
	hex.Encode(res, token)

	log.Println(string(res))
	if string(res) != "95203e29f9a30f2561c4076b579cc02c3f0bf73788deca6d45c3a939ec2fe6b9" {
		panic("kdf did not get expected result")
	}
}
