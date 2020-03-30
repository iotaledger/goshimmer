package faucet

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	faucet "github.com/iotaledger/goshimmer/packages/binary/faucet/payload"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message/payload/data"
	"github.com/iotaledger/goshimmer/packages/binary/signature/ed25119"
)

func TestIsFaucetReq(t *testing.T) {
	faucetMsg := message.New(
		message.EmptyId,
		message.EmptyId,
		ed25119.GenerateKeyPair(),
		time.Now(),
		0,
		faucet.New([]byte("address")),
	)

	dataMsg := message.New(
		message.EmptyId,
		message.EmptyId,
		ed25119.GenerateKeyPair(),
		time.Now(),
		0,
		data.New([]byte("data")),
	)

	assert.Equal(t, true, IsFaucetReq(faucetMsg))
	assert.Equal(t, false, IsFaucetReq(dataMsg))
}
