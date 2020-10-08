package faucet

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"

	faucet "github.com/iotaledger/goshimmer/dapps/faucet/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

func TestIsFaucetReq(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()
	local := identity.NewLocalIdentity(keyPair.PublicKey, keyPair.PrivateKey)

	faucetPayload, err := faucet.New(address.Random(), 4)
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
		faucetPayload,
		0,
		ed25519.EmptySignature,
	)

	dataMsg := tangle.NewMessage(
		tangle.EmptyMessageID,
		tangle.EmptyMessageID,
		time.Now(),
		local.PublicKey(),
		0,
		payload.NewData([]byte("data")),
		0,
		ed25519.EmptySignature,
	)

	assert.Equal(t, true, faucet.IsFaucetReq(faucetMsg))
	assert.Equal(t, false, faucet.IsFaucetReq(dataMsg))
}
