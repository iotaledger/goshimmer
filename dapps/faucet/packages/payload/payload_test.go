package faucetpayload

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
)

func ExamplePayload() {
	keyPair := ed25519.GenerateKeyPair()
	local := identity.NewLocalIdentity(keyPair.PublicKey, keyPair.PrivateKey)

	// 1. create faucet payload
	faucetPayload := New(
		// request address
		address.Random(),
	)

	// 2. build actual message
	tx := message.New(
		message.EmptyId,
		message.EmptyId,
		time.Now(),
		local.PublicKey(),
		0,
		faucetPayload,
		0,
		ed25519.EmptySignature,
	)
	fmt.Println(tx.String())
}

func TestPayload(t *testing.T) {
	originalPayload := New(address.Random())

	clonedPayload1, err, _ := FromBytes(originalPayload.Bytes())
	if err != nil {
		panic(err)
	}

	assert.Equal(t, originalPayload.Address(), clonedPayload1.Address())

	clonedPayload2, err, _ := FromBytes(clonedPayload1.Bytes())
	if err != nil {
		panic(err)
	}

	assert.Equal(t, originalPayload.Address(), clonedPayload2.Address())
}
