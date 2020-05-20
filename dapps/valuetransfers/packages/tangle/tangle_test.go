package tangle

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/hive.go/crypto/ed25519"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/branchmanager"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/packages/binary/testutil"
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

func TestTangle_ValueTransfer(t *testing.T) {
	// initialize tangle + ledgerstate
	valueTangle := New(testutil.DB(t))
	if err := valueTangle.Prune(); err != nil {
		t.Error(err)

		return
	}
	ledgerState := NewLedgerState(valueTangle)

	//
	addressKeyPair1 := ed25519.GenerateKeyPair()
	addressKeyPair2 := ed25519.GenerateKeyPair()
	address1 := address.FromED25519PubKey(addressKeyPair1.PublicKey)
	address2 := address.FromED25519PubKey(addressKeyPair2.PublicKey)

	// check if ledger empty first
	assert.Equal(t, map[balance.Color]int64{}, ledgerState.Balances(address1))
	assert.Equal(t, map[balance.Color]int64{}, ledgerState.Balances(address2))

	// load snapshot
	valueTangle.LoadSnapshot(map[transaction.ID]map[address.Address][]*balance.Balance{
		transaction.GenesisID: {
			address1: []*balance.Balance{
				balance.New(balance.ColorIOTA, 337),
			},

			address2: []*balance.Balance{
				balance.New(balance.ColorIOTA, 1000),
			},
		},
	})

	// check if balance exists after loading snapshot
	assert.Equal(t, map[balance.Color]int64{balance.ColorIOTA: 337}, ledgerState.Balances(address1))
	assert.Equal(t, map[balance.Color]int64{balance.ColorIOTA: 1000}, ledgerState.Balances(address2))

	// attach first spend
	outputAddress1 := address.Random()
	valueTangle.storePayloadWorker(payload.New(payload.GenesisID, payload.GenesisID, transaction.New(
		transaction.NewInputs(
			transaction.NewOutputID(address1, transaction.GenesisID),
			transaction.NewOutputID(address2, transaction.GenesisID),
		),

		transaction.NewOutputs(map[address.Address][]*balance.Balance{
			outputAddress1: {
				balance.New(balance.ColorIOTA, 1337),
			},
		}),
	)))

	// wait for async task to run (TODO: REPLACE TIME BASED APPROACH WITH A WG)
	time.Sleep(500 * time.Millisecond)

	// check if old addresses are empty
	assert.Equal(t, map[balance.Color]int64{}, ledgerState.Balances(address1))
	assert.Equal(t, map[balance.Color]int64{}, ledgerState.Balances(address2))

	// check if new addresses are filled
	assert.Equal(t, map[balance.Color]int64{balance.ColorIOTA: 1337}, ledgerState.Balances(outputAddress1))

	// attach double spend
	outputAddress2 := address.Random()
	valueTangle.storePayloadWorker(payload.New(payload.GenesisID, payload.GenesisID, transaction.New(
		transaction.NewInputs(
			transaction.NewOutputID(address1, transaction.GenesisID),
			transaction.NewOutputID(address2, transaction.GenesisID),
		),

		transaction.NewOutputs(map[address.Address][]*balance.Balance{
			outputAddress2: {
				balance.New(balance.ColorNew, 1337),
			},
		}),
	)))

	// shutdown tangle
	valueTangle.Shutdown()
}
