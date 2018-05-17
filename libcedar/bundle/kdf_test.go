package bundle

import (
	"encoding/hex"
	"testing"
)

func TestCedarKDF(t *testing.T) {
	var k KDF

	k = SimpleKDF{}
	token := k.Generate("MyPassword", "Cedar_Session", 256)

	res := hex.EncodeToString(token)

	LogDebug(res)
	if res != "765f3ae4743384dfa6a7b2ceafe4b795f0f4ef9a6cb12f79fda6477ddbf3c418" {
		panic("KDF did not get expected result")
	}
}
