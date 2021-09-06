package tangle

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

func TestStorage_StoreAttachment(t *testing.T) {
	tangle := newTestTangle()
	defer tangle.Shutdown()

	transactionID, err := ledgerstate.TransactionIDFromRandomness()
	assert.NoError(t, err)
	messageID := randomMessageID()
	cachedAttachment, stored := tangle.Storage.StoreAttachment(transactionID, messageID)
	cachedAttachment.Release()
	assert.True(t, stored)
	cachedAttachment, stored = tangle.Storage.StoreAttachment(transactionID, randomMessageID())
	assert.True(t, stored)
	cachedAttachment.Release()
	cachedAttachment, stored = tangle.Storage.StoreAttachment(transactionID, messageID)
	assert.False(t, stored)
	assert.Nil(t, cachedAttachment)
}

func TestStorage_Attachments(t *testing.T) {
	tangle := newTestTangle()
	defer tangle.Shutdown()

	attachments := make(map[ledgerstate.TransactionID]int)
	for i := 0; i < 2; i++ {
		transactionID, err := ledgerstate.TransactionIDFromRandomness()
		assert.NoError(t, err)
		// for every tx, store random number of attachments.
		for j := 0; j < rand.Intn(5)+1; j++ {
			attachments[transactionID]++
			cachedAttachment, _ := tangle.Storage.StoreAttachment(transactionID, randomMessageID())
			cachedAttachment.Release()
		}
	}

	for transactionID := range attachments {
		cachedAttachments := tangle.Storage.Attachments(transactionID)
		assert.Equal(t, attachments[transactionID], len(cachedAttachments))
		for _, cachedAttachment := range cachedAttachments {
			cachedAttachment.Release()
		}
	}
}

func TestStorage_UnconfirmedTransactionDependencies(t *testing.T) {
	tangle := newTestTangle()
	defer tangle.Shutdown()

	transactionID, err := ledgerstate.TransactionIDFromRandomness()
	assert.NoError(t, err)

	dependencies := make([]ledgerstate.TransactionID, 3)
	for i := 0; i < 3; i++ {
		transactionID, err = ledgerstate.TransactionIDFromRandomness()
		assert.NoError(t, err)
		dependencies[i] = transactionID
	}

	// storage is empty
	emptyCachedDependency := tangle.Storage.UnconfirmedTransactionDependencies(transactionID)
	assert.Nil(t, emptyCachedDependency.Unwrap())
	emptyCachedDependency.Release()

	// store dependencies
	tangle.Storage.UnconfirmedTransactionDependencies(transactionID, func() *UnconfirmedTxDependency {
		return NewUnconfirmedTxDependency(transactionID)
	}).Consume(func(unconfirmedTxDependency *UnconfirmedTxDependency) {
		unconfirmedTxDependency.AddDependency(dependencies[0])
		unconfirmedTxDependency.AddDependency(dependencies[1])
		unconfirmedTxDependency.AddDependency(dependencies[2])
	})

	tangle.Storage.UnconfirmedTransactionDependencies(transactionID).Consume(func(unconfirmedTxDependency *UnconfirmedTxDependency) {
		assert.Equal(t, transactionID, unconfirmedTxDependency.transactionID)
		assert.Equal(t, 3, unconfirmedTxDependency.txDependentIDs.Size())
	})
}
