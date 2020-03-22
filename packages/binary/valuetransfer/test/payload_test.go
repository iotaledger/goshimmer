package test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/binary/signature/ed25119"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/address"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/balance"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/transaction"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/transaction/signaturescheme"
)

func ExamplePayload() {
	// 1. create value transfer (user provides this)
	valueTransfer := transaction.New(
		// inputs
		transaction.NewInputs(
			transaction.NewOutputId(address.Random(), transaction.RandomId()),
			transaction.NewOutputId(address.Random(), transaction.RandomId()),
		),

		// outputs
		transaction.NewOutputs(map[address.Address][]*balance.Balance{
			address.Random(): {
				balance.New(balance.COLOR_IOTA, 1337),
			},
		}),
	)

	// 2. create value payload (the ontology creates this and wraps the user provided transfer accordingly)
	valuePayload := payload.New(
		// trunk in "value transfer ontology" (filled by ontology tipSelector)
		payload.GenesisId,

		// branch in "value transfer ontology"  (filled by ontology tipSelector)
		payload.GenesisId,

		// value transfer
		valueTransfer,
	)

	// 3. build actual transaction (the base layer creates this and wraps the ontology provided payload)
	tx := message.New(
		// trunk in "network tangle" ontology (filled by tipSelector)
		message.EmptyId,

		// branch in "network tangle" ontology (filled by tipSelector)
		message.EmptyId,

		// issuer of the transaction (signs automatically)
		ed25119.GenerateKeyPair(),

		// the time when the transaction was created
		time.Now(),

		// the ever increasing sequence number of this transaction
		0,

		// payload
		valuePayload,
	)

	fmt.Println(tx)
}

func TestPayload(t *testing.T) {
	addressKeyPair1 := ed25119.GenerateKeyPair()
	addressKeyPair2 := ed25119.GenerateKeyPair()

	originalPayload := payload.New(
		payload.GenesisId,
		payload.GenesisId,
		transaction.New(
			transaction.NewInputs(
				transaction.NewOutputId(address.FromED25519PubKey(addressKeyPair1.PublicKey), transaction.RandomId()),
				transaction.NewOutputId(address.FromED25519PubKey(addressKeyPair2.PublicKey), transaction.RandomId()),
			),

			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				address.Random(): {
					balance.New(balance.COLOR_IOTA, 1337),
				},
			}),
		).Sign(
			signaturescheme.ED25519(addressKeyPair1),
		),
	)

	assert.Equal(t, false, originalPayload.Transaction().SignaturesValid())

	originalPayload.Transaction().Sign(
		signaturescheme.ED25519(addressKeyPair2),
	)

	assert.Equal(t, true, originalPayload.Transaction().SignaturesValid())

	clonedPayload1, err, _ := payload.FromBytes(originalPayload.Bytes())
	if err != nil {
		panic(err)
	}

	assert.Equal(t, originalPayload.BranchId(), clonedPayload1.BranchId())
	assert.Equal(t, originalPayload.TrunkId(), clonedPayload1.TrunkId())
	assert.Equal(t, originalPayload.Transaction().Bytes(), clonedPayload1.Transaction().Bytes())
	assert.Equal(t, originalPayload.GetId(), clonedPayload1.GetId())
	assert.Equal(t, true, clonedPayload1.Transaction().SignaturesValid())

	clonedPayload2, err, _ := payload.FromBytes(clonedPayload1.Bytes())
	if err != nil {
		panic(err)
	}

	assert.Equal(t, originalPayload.GetId(), clonedPayload2.GetId())
	assert.Equal(t, true, clonedPayload2.Transaction().SignaturesValid())
}
