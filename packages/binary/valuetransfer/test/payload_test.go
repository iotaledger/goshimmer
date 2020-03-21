package test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/binary/signature/ed25119"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/address"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/coloredbalance"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/coloredbalance/color"
	valuepayload "github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload"
	payloadid "github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload/id"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload/transfer/signatures"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/transaction"
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
		transaction.NewOutputs(map[address.Address][]*coloredbalance.ColoredBalance{
			address.Random(): {
				coloredbalance.New(color.IOTA, 1337),
			},
		}),
	)

	// 2. create value payload (the ontology creates this and wraps the user provided transfer accordingly)
	valuePayload := valuepayload.New(
		// trunk in "value transfer ontology" (filled by ontology tipSelector)
		payloadid.GenesisId,

		// branch in "value transfer ontology"  (filled by ontology tipSelector)
		payloadid.GenesisId,

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

	originalPayload := valuepayload.New(
		payloadid.GenesisId,
		payloadid.GenesisId,
		transaction.New(
			transaction.NewInputs(
				transaction.NewOutputId(address.FromED25519PubKey(addressKeyPair1.PublicKey), transaction.RandomId()),
				transaction.NewOutputId(address.FromED25519PubKey(addressKeyPair2.PublicKey), transaction.RandomId()),
			),

			transaction.NewOutputs(map[address.Address][]*coloredbalance.ColoredBalance{
				address.Random(): {
					coloredbalance.New(color.IOTA, 1337),
				},
			}),
		).Sign(
			signatures.ED25519(addressKeyPair1),
		),
	)

	assert.Equal(t, false, originalPayload.GetTransfer().SignaturesValid())

	originalPayload.GetTransfer().Sign(
		signatures.ED25519(addressKeyPair2),
	)

	assert.Equal(t, true, originalPayload.GetTransfer().SignaturesValid())

	clonedPayload1, err, _ := valuepayload.FromBytes(originalPayload.Bytes())
	if err != nil {
		panic(err)
	}

	assert.Equal(t, originalPayload.GetId(), clonedPayload1.GetId())
	assert.Equal(t, true, clonedPayload1.GetTransfer().SignaturesValid())

	clonedPayload2, err, _ := valuepayload.FromBytes(clonedPayload1.Bytes())
	if err != nil {
		panic(err)
	}

	assert.Equal(t, originalPayload.GetId(), clonedPayload2.GetId())
	assert.Equal(t, true, clonedPayload2.GetTransfer().SignaturesValid())
}
