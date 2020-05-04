package utxodag

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/branchmanager"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tangle"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/plugins/config"
)

func TestNewOutput(t *testing.T) {
	randomAddress := address.Random()
	randomTransactionID := transaction.RandomID()

	output := NewOutput(randomAddress, randomTransactionID, branchmanager.MasterBranchID, []*balance.Balance{
		balance.New(balance.ColorIOTA, 1337),
	})

	assert.Equal(t, randomAddress, output.Address())
	assert.Equal(t, randomTransactionID, output.TransactionID())
	assert.Equal(t, false, output.Solid())
	assert.Equal(t, time.Time{}, output.SolidificationTime())
	assert.Equal(t, []*balance.Balance{
		balance.New(balance.ColorIOTA, 1337),
	}, output.Balances())

	assert.Equal(t, true, output.SetSolid(true))
	assert.Equal(t, false, output.SetSolid(true))
	assert.Equal(t, true, output.Solid())
	assert.NotEqual(t, time.Time{}, output.SolidificationTime())

	clonedOutput, _, err := OutputFromBytes(output.Bytes())
	if err != nil {
		panic(err)
	}

	assert.Equal(t, output.Address(), clonedOutput.Address())
	assert.Equal(t, output.TransactionID(), clonedOutput.TransactionID())
	assert.Equal(t, output.Solid(), clonedOutput.Solid())
	assert.Equal(t, output.SolidificationTime().Round(time.Second), clonedOutput.SolidificationTime().Round(time.Second))
	assert.Equal(t, output.Balances(), clonedOutput.Balances())
}

func TestAttachment(t *testing.T) {
	transactionID := transaction.RandomID()
	payloadID := payload.RandomID()

	attachment := NewAttachment(transactionID, payloadID)

	assert.Equal(t, transactionID, attachment.TransactionID())
	assert.Equal(t, payloadID, attachment.PayloadID())

	clonedAttachment, consumedBytes, err := AttachmentFromBytes(attachment.Bytes())
	if err != nil {
		panic(err)
	}

	assert.Equal(t, AttachmentLength, consumedBytes)
	assert.Equal(t, transactionID, clonedAttachment.TransactionID())
	assert.Equal(t, payloadID, clonedAttachment.PayloadID())
}

func TestTangle_AttachPayload(t *testing.T) {
	dir, err := ioutil.TempDir("", t.Name())
	require.NoError(t, err)
	defer os.Remove(dir)

	config.Node.Set(database.CFG_DIRECTORY, dir)

	valueTangle := tangle.New(database.GetBadgerInstance())
	if err := valueTangle.Prune(); err != nil {
		t.Error(err)

		return
	}

	utxoDAG := New(database.GetBadgerInstance(), valueTangle)

	addressKeyPair1 := ed25519.GenerateKeyPair()
	addressKeyPair2 := ed25519.GenerateKeyPair()

	transferID1, _ := transaction.IDFromBase58("8opHzTAnfzRpPEx21XtnrVTX28YQuCpAjcn1PczScKh")
	transferID2, _ := transaction.IDFromBase58("4uQeVj5tqViQh7yWWGStvkEG1Zmhx6uasJtWCJziofM")

	input1 := NewOutput(address.FromED25519PubKey(addressKeyPair1.PublicKey), transferID1, branchmanager.MasterBranchID, []*balance.Balance{
		balance.New(balance.ColorIOTA, 337),
	})
	input1.SetSolid(true)
	input2 := NewOutput(address.FromED25519PubKey(addressKeyPair2.PublicKey), transferID2, branchmanager.MasterBranchID, []*balance.Balance{
		balance.New(balance.ColorIOTA, 1000),
	})
	input2.SetSolid(true)

	utxoDAG.outputStorage.Store(input1).Release()
	utxoDAG.outputStorage.Store(input2).Release()

	outputAddress1 := address.Random()
	outputAddress2 := address.Random()

	// attach first spend
	valueTangle.AttachPayload(payload.New(payload.GenesisID, payload.GenesisID, transaction.New(
		transaction.NewInputs(
			input1.ID(),
			input2.ID(),
		),

		transaction.NewOutputs(map[address.Address][]*balance.Balance{
			outputAddress1: {
				balance.New(balance.ColorNew, 1337),
			},
		}),
	)))

	// attach double spend
	valueTangle.AttachPayload(payload.New(payload.GenesisID, payload.GenesisID, transaction.New(
		transaction.NewInputs(
			input1.ID(),
			input2.ID(),
		),

		transaction.NewOutputs(map[address.Address][]*balance.Balance{
			outputAddress2: {
				balance.New(balance.ColorNew, 1337),
			},
		}),
	)))

	valueTangle.Shutdown()
	utxoDAG.Shutdown()
}
