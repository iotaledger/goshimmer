package tangle

import (
	"github.com/iotaledger/hive.go/types"
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
	transactionIDs := make(ledgerstate.TransactionIDs, 2)
	dependency1TxID, err := ledgerstate.TransactionIDFromRandomness()
	assert.NoError(t, err)
	dependency2TxID, err := ledgerstate.TransactionIDFromRandomness()
	assert.NoError(t, err)
	transactionIDs[dependency1TxID] = types.Void
	transactionIDs[dependency2TxID] = types.Void

	dependencies := NewUnconfirmedTxDependency(&transactionID)
	dependencies.AddDependency(&dependency1TxID)
	dependencies.AddDependency(&dependency2TxID)

	cachedDependencies := tangle.Storage.UnconfirmedTransactionDependencies(&transactionID)
	assert.NotNil(t, cachedDependencies)

	tangle.Storage.StoreUnconfirmedTransactionDependencies(dependencies)
	cachedDependencies = tangle.Storage.UnconfirmedTransactionDependencies(&transactionID)
	assert.NotNil(t, cachedDependencies)

	assert.Equal(t, transactionID, cachedDependencies.Unwrap().dependencyTxID)
	assert.Equal(t, transactionIDs, cachedDependencies.Unwrap().dependentTxIDs)
}
