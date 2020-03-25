package tangle

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/iotaledger/hive.go/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/binary/signature/ed25119"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/address"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/balance"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/transaction"
	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/plugins/config"
)

func TestAttachment(t *testing.T) {
	transactionId := transaction.RandomId()
	payloadId := payload.RandomId()

	attachment := NewAttachment(transactionId, payloadId)

	assert.Equal(t, transactionId, attachment.TransactionId())
	assert.Equal(t, payloadId, attachment.PayloadId())

	clonedAttachment, err, consumedBytes := AttachmentFromBytes(attachment.Bytes())
	if err != nil {
		panic(err)
	}

	assert.Equal(t, AttachmentLength, consumedBytes)
	assert.Equal(t, transactionId, clonedAttachment.TransactionId())
	assert.Equal(t, payloadId, clonedAttachment.PayloadId())
}

func TestTangle_AttachPayload(t *testing.T) {
	dir, err := ioutil.TempDir("", t.Name())
	require.NoError(t, err)
	defer os.Remove(dir)

	config.Node.Set(database.CFG_DIRECTORY, dir)

	tangle := New(database.GetBadgerInstance(), []byte("TEST_BINARY_TANGLE"))
	if err := tangle.Prune(); err != nil {
		t.Error(err)

		return
	}

	tangle.Events.PayloadSolid.Attach(events.NewClosure(func(payload *payload.CachedPayload, metadata *CachedPayloadMetadata) {
		fmt.Println(payload.Unwrap())

		payload.Release()
		metadata.Release()
	}))

	addressKeyPair1 := ed25119.GenerateKeyPair()
	addressKeyPair2 := ed25119.GenerateKeyPair()

	transferId1, _ := transaction.IdFromBase58("8opHzTAnfzRpPEx21XtnrVTX28YQuCpAjcn1PczScKh")
	transferId2, _ := transaction.IdFromBase58("4uQeVj5tqViQh7yWWGStvkEG1Zmhx6uasJtWCJziofM")

	tangle.AttachPayload(payload.New(payload.GenesisId, payload.GenesisId, transaction.New(
		transaction.NewInputs(
			transaction.NewOutputId(address.FromED25519PubKey(addressKeyPair1.PublicKey), transferId1),
			transaction.NewOutputId(address.FromED25519PubKey(addressKeyPair2.PublicKey), transferId2),
		),

		transaction.NewOutputs(map[address.Address][]*balance.Balance{
			address.Random(): {
				balance.New(balance.COLOR_IOTA, 1337),
			},
		}),
	)))

	tangle.Shutdown()
}
