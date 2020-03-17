package tipselector

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/binary/signature/ed25119"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction/payload/data"
)

func Test(t *testing.T) {
	// create tip selector
	tipSelector := New()

	// check if first tips point to genesis
	trunk1, branch1 := tipSelector.GetTips()
	assert.Equal(t, transaction.EmptyId, trunk1)
	assert.Equal(t, transaction.EmptyId, branch1)

	// create a transaction and attach it
	transaction1 := transaction.New(trunk1, branch1, ed25119.GenerateKeyPair(), time.Now(), 0, data.New([]byte("testtransaction")))
	tipSelector.AddTip(transaction1)

	// check if the tip shows up in the tip count
	assert.Equal(t, 1, tipSelector.GetTipCount())

	// check if next tips point to our first transaction
	trunk2, branch2 := tipSelector.GetTips()
	assert.Equal(t, transaction1.GetId(), trunk2)
	assert.Equal(t, transaction1.GetId(), branch2)

	// create a 2nd transaction and attach it
	transaction2 := transaction.New(transaction.EmptyId, transaction.EmptyId, ed25119.GenerateKeyPair(), time.Now(), 0, data.New([]byte("testtransaction")))
	tipSelector.AddTip(transaction2)

	// check if the tip shows up in the tip count
	assert.Equal(t, 2, tipSelector.GetTipCount())

	// attach a transaction to our two tips
	trunk3, branch3 := tipSelector.GetTips()
	transaction3 := transaction.New(trunk3, branch3, ed25119.GenerateKeyPair(), time.Now(), 0, data.New([]byte("testtransaction")))
	tipSelector.AddTip(transaction3)

	// check if the tip shows replaces the current tips
	trunk4, branch4 := tipSelector.GetTips()
	assert.Equal(t, 1, tipSelector.GetTipCount())
	assert.Equal(t, transaction3.GetId(), trunk4)
	assert.Equal(t, transaction3.GetId(), branch4)
}
