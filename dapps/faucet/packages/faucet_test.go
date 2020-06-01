package faucet

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"

	faucet "github.com/iotaledger/goshimmer/dapps/faucet/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	data "github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
)

func TestIsFaucetReq(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()
	local := identity.NewLocalIdentity(keyPair.PublicKey, keyPair.PrivateKey)

	faucetMsg := message.New(
		message.EmptyId,
		message.EmptyId,
		local,
		time.Now(),
		0,
		faucet.New(address.Random()),
	)

	dataMsg := message.New(
		message.EmptyId,
		message.EmptyId,
		local,
		time.Now(),
		0,
		data.NewData([]byte("data")),
	)

	assert.Equal(t, true, IsFaucetReq(faucetMsg))
	assert.Equal(t, false, IsFaucetReq(dataMsg))
}
