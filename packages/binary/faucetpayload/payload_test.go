package faucetpayload

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/binary/signature/ed25119"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction"
)

func ExampleFaucetPayload() {
	// 1. create faucet payload
	faucetPayload := New(
		// request address
		[]byte("address"),
	)

	// 2. build actual transaction
	tx := transaction.New(
		// trunk in "network tangle" ontology (filled by tipSelector)
		transaction.EmptyId,

		// branch in "network tangle" ontology (filled by tipSelector)
		transaction.EmptyId,

		// issuer of the transaction (signs automatically)
		ed25119.GenerateKeyPair(),

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
