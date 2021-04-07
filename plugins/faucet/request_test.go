package faucet

import (
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

func ExampleRequest() {
	keyPair := ed25519.GenerateKeyPair()
	address := ledgerstate.NewED25519Address(keyPair.PublicKey)
	local := identity.NewLocalIdentity(keyPair.PublicKey, keyPair.PrivateKey)

	// 1. create faucet payload
	faucetRequest, err := NewRequest(address, 4)
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
	keyPair := ed25519.GenerateKeyPair()
	address := ledgerstate.NewED25519Address(keyPair.PublicKey)
	originalRequest, err := NewRequest(address, 4)
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

func TestIsFaucetReq(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()
	address := ledgerstate.NewED25519Address(keyPair.PublicKey)
	local := identity.NewLocalIdentity(keyPair.PublicKey, keyPair.PrivateKey)

	faucetRequest, err := NewRequest(address, 4)
	if err != nil {
		require.NoError(t, err)
		return
	}
	faucetMsg := tangle.NewMessage(
		[]tangle.MessageID{tangle.EmptyMessageID},
		[]tangle.MessageID{},
		time.Now(),
		local.PublicKey(),
		0,
		faucetRequest,
		0,
		ed25519.EmptySignature,
	)

	dataMsg := tangle.NewMessage(
		[]tangle.MessageID{tangle.EmptyMessageID},
		[]tangle.MessageID{},
		time.Now(),
		local.PublicKey(),
		0,
		payload.NewGenericDataPayload([]byte("data")),
		0,
		ed25519.EmptySignature,
	)

	assert.Equal(t, true, IsFaucetReq(faucetMsg))
	assert.Equal(t, false, IsFaucetReq(dataMsg))
}
