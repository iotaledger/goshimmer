package tangle

import (
	"math/rand"
	"testing"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/stretchr/testify/assert"
)

func TestMessageStore_StoreAttachment(t *testing.T) {
	msgStore := NewMessageStore(mapdb.NewMapDB())
	defer msgStore.Shutdown()

	transactionID, err := ledgerstate.TransactionIDFromRandomness()
	assert.NoError(t, err)
	messageID := randomMessageID()
	cachedAttachment, stored := msgStore.StoreAttachment(transactionID, messageID)
	cachedAttachment.Release()
	assert.True(t, stored)
	cachedAttachment, stored = msgStore.StoreAttachment(transactionID, randomMessageID())
	assert.True(t, stored)
	cachedAttachment.Release()
	cachedAttachment, stored = msgStore.StoreAttachment(transactionID, messageID)
	assert.False(t, stored)
	assert.Nil(t, cachedAttachment)
}

func TestMessageStore_Attachments(t *testing.T) {
	msgStore := NewMessageStore(mapdb.NewMapDB())
	defer msgStore.Shutdown()

	attachments := make(map[ledgerstate.TransactionID]int)
	for i := 0; i < 2; i++ {
		transactionID, err := ledgerstate.TransactionIDFromRandomness()
		assert.NoError(t, err)
		// for every tx, store random number of attachments.
		for j := 0; j < rand.Intn(5)+1; j++ {
			attachments[transactionID]++
			cachedAttachment, _ := msgStore.StoreAttachment(transactionID, randomMessageID())
			cachedAttachment.Release()
		}
	}

	for transactionID := range attachments {
		cachedAttachments := msgStore.Attachments(transactionID)
		assert.Equal(t, attachments[transactionID], len(cachedAttachments))
		for _, cachedAttachment := range cachedAttachments {
			cachedAttachment.Release()
		}
	}

}
