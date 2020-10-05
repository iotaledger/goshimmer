package faucet

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

func TestIsFaucetReq(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()
	local := identity.NewLocalIdentity(keyPair.PublicKey, keyPair.PrivateKey)

	faucetObject, err := NewObject(address.Random(), 4)
	if err != nil {
		require.NoError(t, err)
		return
	}
	faucetMsg := tangle.NewMessage(
		tangle.EmptyMessageID,
		tangle.EmptyMessageID,
		time.Now(),
		local.PublicKey(),
		0,
		faucetObject,
		0,
		ed25519.EmptySignature,
	)

	dataMsg := tangle.NewMessage(
		tangle.EmptyMessageID,
		tangle.EmptyMessageID,
		time.Now(),
		local.PublicKey(),
		0,
		tangle.NewDataPayload([]byte("data")),
		0,
		ed25519.EmptySignature,
	)

	assert.Equal(t, true, IsFaucetReq(faucetMsg))
	assert.Equal(t, false, IsFaucetReq(dataMsg))
}
