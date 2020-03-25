package faucet

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	faucet "github.com/iotaledger/goshimmer/packages/binary/faucet/payload"
	"github.com/iotaledger/goshimmer/packages/binary/signature/ed25119"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message/payload/data"
)

func TestIsFaucetReq(t *testing.T) {
	faucetTxn := message.New(
		message.EmptyId,
		message.EmptyId,
		ed25119.GenerateKeyPair(),
		time.Now(),
		0,
		faucet.New([]byte("address")),
	)

	dataTxn := message.New(
		message.EmptyId,
		message.EmptyId,
		ed25119.GenerateKeyPair(),
		time.Now(),
		0,
		data.New([]byte("data")),
	)

	assert.Equal(t, true, IsFaucetReq(faucetTxn))
	assert.Equal(t, false, IsFaucetReq(dataTxn))
}
