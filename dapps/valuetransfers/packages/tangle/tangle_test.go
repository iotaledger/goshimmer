package tangle

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/branchmanager"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
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
