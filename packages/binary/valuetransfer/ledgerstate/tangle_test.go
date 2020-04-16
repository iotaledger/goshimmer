package ledgerstate

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

	tangle := New(database.GetBadgerInstance())
	if err := tangle.Prune(); err != nil {
		t.Error(err)

		return
	}

	addressKeyPair1 := ed25519.GenerateKeyPair()
	addressKeyPair2 := ed25519.GenerateKeyPair()

	transferId1, _ := transaction.IdFromBase58("8opHzTAnfzRpPEx21XtnrVTX28YQuCpAjcn1PczScKh")
	transferId2, _ := transaction.IdFromBase58("4uQeVj5tqViQh7yWWGStvkEG1Zmhx6uasJtWCJziofM")

	input1 := NewOutput(address.FromED25519PubKey(addressKeyPair1.PublicKey), transferId1, MasterBranchId, []*balance.Balance{
		balance.New(balance.COLOR_IOTA, 337),
	})
	input1.SetSolid(true)
	input2 := NewOutput(address.FromED25519PubKey(addressKeyPair2.PublicKey), transferId2, MasterBranchId, []*balance.Balance{
		balance.New(balance.COLOR_IOTA, 1000),
	})
	input2.SetSolid(true)

	tangle.outputStorage.Store(input1)
	tangle.outputStorage.Store(input2)

	outputAddress := address.Random()

	tx := transaction.New(
		transaction.NewInputs(
			input1.Id(),
			input2.Id(),
		),

		transaction.NewOutputs(map[address.Address][]*balance.Balance{
			outputAddress: {
				balance.New(balance.COLOR_NEW, 1337),
			},
		}),
	)

	tangle.AttachPayload(payload.New(payload.GenesisId, payload.GenesisId, tx))

	tangle.Shutdown()
}
