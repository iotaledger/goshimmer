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

func ExampleObject() {
	keyPair := ed25519.GenerateKeyPair()
	local := identity.NewLocalIdentity(keyPair.PublicKey, keyPair.PrivateKey)

	// 1. create faucet payload
	faucetPayload, err := NewObject(address.Random(), 4)
	if err != nil {
		panic(err)
	}

	// 2. build actual message
	tx := tangle.NewMessage(
		tangle.EmptyMessageID,
		tangle.EmptyMessageID,
		time.Now(),
		local.PublicKey(),
		0,
		faucetPayload,
		0,
		ed25519.EmptySignature,
	)
	fmt.Println(tx.String())
}

func TestObject(t *testing.T) {
	originalObject, err := NewObject(address.Random(), 4)
	if err != nil {
		panic(err)
	}

	clonedObject1, _, err := FromBytes(originalObject.Bytes())
	if err != nil {
		panic(err)
	}

	assert.Equal(t, originalObject.Address(), clonedObject1.Address())

	clonedObject2, _, err := FromBytes(clonedObject1.Bytes())
	if err != nil {
		panic(err)
	}

	assert.Equal(t, originalObject.Address(), clonedObject2.Address())
}
