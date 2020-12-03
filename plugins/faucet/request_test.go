package faucet

import (
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

func ExampleRequest() {
	keyPair := ed25519.GenerateKeyPair()
	local := identity.NewLocalIdentity(keyPair.PublicKey, keyPair.PrivateKey)

	// 1. create faucet payload
	faucetRequest, err := NewRequest(address.Random(), 4)
	if err != nil {
		panic(err)
	}

	// 2. build actual message
	tx := tangle.NewMessage(
		[]tangle.MessageID{tangle.EmptyMessageID},
		[]tangle.MessageID{},
		time.Now(),
		local.PublicKey(),
		0,
		faucetRequest,
		0,
		ed25519.EmptySignature,
	)
	fmt.Println(tx.String())
}

func TestRequest(t *testing.T) {
	originalRequest, err := NewRequest(address.Random(), 4)
	if err != nil {
		panic(err)
	}

	clonedRequest, _, err := FromBytes(originalRequest.Bytes())
	if err != nil {
		panic(err)
	}

	assert.Equal(t, originalRequest.Address(), clonedRequest.Address())

	clonedRequest2, _, err := FromBytes(clonedRequest.Bytes())
	if err != nil {
		panic(err)
	}

	assert.Equal(t, originalRequest.Address(), clonedRequest2.Address())
}
