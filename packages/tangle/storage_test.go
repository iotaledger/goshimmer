package tangle

import (
	"math/rand"
	"testing"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/stretchr/testify/assert"
)

func TestStorage_StoreAttachment(t *testing.T) {
	storage := NewStorage(mapdb.NewMapDB())
	defer storage.Shutdown()

	transactionID, err := ledgerstate.TransactionIDFromRandomness()
	assert.NoError(t, err)
	messageID := randomMessageID()
	cachedAttachment, stored := storage.StoreAttachment(transactionID, messageID)
	cachedAttachment.Release()
	assert.True(t, stored)
	cachedAttachment, stored = storage.StoreAttachment(transactionID, randomMessageID())
	assert.True(t, stored)
	cachedAttachment.Release()
	cachedAttachment, stored = storage.StoreAttachment(transactionID, messageID)
	assert.False(t, stored)
	assert.Nil(t, cachedAttachment)
}

func TestStorage_Attachments(t *testing.T) {
	storage := NewStorage(mapdb.NewMapDB())
	defer storage.Shutdown()

	attachments := make(map[ledgerstate.TransactionID]int)
	for i := 0; i < 2; i++ {
		transactionID, err := ledgerstate.TransactionIDFromRandomness()
		assert.NoError(t, err)
		// for every tx, store random number of attachments.
		for j := 0; j < rand.Intn(5)+1; j++ {
			attachments[transactionID]++
			cachedAttachment, _ := storage.StoreAttachment(transactionID, randomMessageID())
			cachedAttachment.Release()
		}
	}

	for transactionID := range attachments {
		cachedAttachments := storage.Attachments(transactionID)
		assert.Equal(t, attachments[transactionID], len(cachedAttachments))
		for _, cachedAttachment := range cachedAttachments {
			cachedAttachment.Release()
		}
	}

}
