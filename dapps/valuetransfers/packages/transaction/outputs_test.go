package transaction

import (
	"bytes"

	"github.com/stretchr/testify/assert"

	"testing"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"golang.org/x/crypto/blake2b"
)

func TestOutputs(t *testing.T) {
	rndAddrs := make([]address.Address, 15)
	for i := range rndAddrs {
		rndAddrs[i] = address.RandomOfType(address.VersionED25519)
	}

	theMap1 := make(map[address.Address][]*balance.Balance)
	for i := 0; i < len(rndAddrs); i++ {
		theMap1[rndAddrs[i]] = []*balance.Balance{balance.New(balance.ColorIOTA, int64(i))}
	}
	out1 := NewOutputs(theMap1)

	theMap2 := make(map[address.Address][]*balance.Balance)
	for i := len(rndAddrs) - 1; i >= 0; i-- {
		theMap2[rndAddrs[i]] = []*balance.Balance{balance.New(balance.ColorIOTA, int64(i))}
	}
	out2 := NewOutputs(theMap2)

	h1 := hashOutputs(t, out1)
	h2 := hashOutputs(t, out2)

	assert.Equal(t, bytes.Equal(h1, h2), true)
}

func hashOutputs(t *testing.T, out *Outputs) []byte {
	h, err := blake2b.New256(nil)
	assert.NoError(t, err)

	h.Write(out.Bytes())
	return h.Sum(nil)
}
