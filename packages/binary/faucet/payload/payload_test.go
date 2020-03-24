package faucetpayload

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/binary/signature/ed25119"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message"
)

func ExampleFaucetPayload() {
	// 1. create faucet payload
	faucetPayload := New(
		// request address
		[]byte("address"),
	)

	// 2. build actual message
	tx := message.New(
		// trunk in "network tangle" ontology (filled by tipSelector)
		message.EmptyId,

		// branch in "network tangle" ontology (filled by tipSelector)
		message.EmptyId,

		// issuer of the message (signs automatically)
		ed25119.GenerateKeyPair(),

		// issuing time
		time.Now(),

		// sequence number
		0,

		// payload
		faucetPayload,
	)
	fmt.Println(tx.String())
}

func TestPayload(t *testing.T) {
	originalPayload := New([]byte("address"))

	clonedPayload1, err, _ := FromBytes(originalPayload.Bytes())
	if err != nil {
		panic(err)
	}

	assert.Equal(t, originalPayload.GetAddress(), clonedPayload1.GetAddress())

	clonedPayload2, err, _ := FromBytes(clonedPayload1.Bytes())
	if err != nil {
		panic(err)
	}

	assert.Equal(t, originalPayload.GetAddress(), clonedPayload2.GetAddress())
}
