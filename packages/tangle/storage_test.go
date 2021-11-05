package tangle

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

func TestStorage_StoreAttachment(t *testing.T) {
	tangle := NewTestTangle()
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
	tangle := NewTestTangle()
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
