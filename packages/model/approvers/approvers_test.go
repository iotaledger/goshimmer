package approvers

import (
	"fmt"
	"testing"

	"github.com/iotaledger/iota.go/trinary"
	"github.com/magiconair/properties/assert"
)

func TestApprovers_SettersGetters(t *testing.T) {
	hashA := trinary.Trytes("A9999999999999999999999999999999999999999999999999999999999999999999999999999999F")
	hashB := trinary.Trytes("B9999999999999999999999999999999999999999999999999999999999999999999999999999999F")
	approversTest := New(hashA)
	approversTest.Add(hashB)

	assert.Equal(t, approversTest.GetHash(), hashA, "hash")
	assert.Equal(t, approversTest.GetHashes()[0], hashB, "hashes")
}

func TestApprovers_MarshalUnmarshal(t *testing.T) {
	hashA := trinary.Trytes("A9999999999999999999999999999999999999999999999999999999999999999999999999999999F")
	hashB := trinary.Trytes("B9999999999999999999999999999999999999999999999999999999999999999999999999999999F")
	approversTest := New(hashA)
	approversTest.Add(hashB)

	approversTestBytes := approversTest.Marshal()

	var approversUnmarshaled Approvers
	err := approversUnmarshaled.Unmarshal(approversTestBytes)
	if err != nil {
		fmt.Println(err, len(approversTestBytes))
	}

	assert.Equal(t, approversUnmarshaled.GetHash(), approversTest.GetHash(), "hash")
	assert.Equal(t, approversUnmarshaled.GetHashes(), approversTest.GetHashes(), "hashes")
}
