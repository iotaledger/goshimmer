package tangle

import (
	"testing"

	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
)

// TestStorePayload checks whether a value object is correctly stored.
func TestStorePayload(t *testing.T) {
	tangle := New(mapdb.NewMapDB())

	tx := createDummyTransaction()
	valueObject := payload.New(payload.GenesisID, payload.GenesisID, tx)
	{
		cachedPayload, cachedMetadata, stored := tangle.storePayload(valueObject)
		cachedPayload.Consume(func(payload *payload.Payload) {
			assert.True(t, assert.ObjectsAreEqual(valueObject, payload))
		})
		cachedMetadata.Consume(func(payloadMetadata *PayloadMetadata) {
			assert.Equal(t, valueObject.ID(), payloadMetadata.PayloadID())
		})
		assert.True(t, stored)
	}

	// store same value object again -> should return false
	{
		cachedPayload, cachedMetadata, stored := tangle.storePayload(valueObject)
		assert.Nil(t, cachedPayload)
		assert.Nil(t, cachedMetadata)
		assert.False(t, stored)
	}

	// retrieve from tangle
	{
		cachedPayload := tangle.Payload(valueObject.ID())
		cachedPayload.Consume(func(payload *payload.Payload) {
			assert.True(t, assert.ObjectsAreEqual(valueObject, payload))
		})
		cachedMetadata := tangle.PayloadMetadata(valueObject.ID())
		cachedMetadata.Consume(func(payloadMetadata *PayloadMetadata) {
			assert.Equal(t, valueObject.ID(), payloadMetadata.PayloadID())
		})

	}
}

// TestStoreTransactionModels checks whether all models corresponding to a transaction are correctly created.
func TestStoreTransactionModels(t *testing.T) {
	tangle := New(mapdb.NewMapDB())

	tx := createDummyTransaction()
	valueObject := payload.New(payload.GenesisID, payload.GenesisID, tx)
	{
		cachedTransaction, cachedTransactionMetadata, cachedAttachment, transactionIsNew := tangle.storeTransactionModels(valueObject)
		cachedTransaction.Consume(func(transaction *transaction.Transaction) {
			assert.True(t, assert.ObjectsAreEqual(tx, transaction))
		})
		cachedTransactionMetadata.Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, tx.ID(), transactionMetadata.ID())
		})
		expectedAttachment := NewAttachment(tx.ID(), valueObject.ID())
		cachedAttachment.Consume(func(attachment *Attachment) {
			assert.Equal(t, expectedAttachment.TransactionID(), attachment.TransactionID())
			assert.Equal(t, expectedAttachment.PayloadID(), attachment.PayloadID())
		})
		assert.True(t, transactionIsNew)
	}

	// add same value object with same tx again -> should return false
	{
		cachedTransaction, cachedTransactionMetadata, cachedAttachment, transactionIsNew := tangle.storeTransactionModels(valueObject)
		cachedTransaction.Consume(func(transaction *transaction.Transaction) {
			assert.True(t, assert.ObjectsAreEqual(tx, transaction))
		})
		cachedTransactionMetadata.Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, tx.ID(), transactionMetadata.ID())
		})
		assert.Nil(t, cachedAttachment)
		assert.False(t, transactionIsNew)
	}

	// store same tx with different value object -> new attachment, same tx, transactionIsNew=false
	valueObject2 := payload.New(payload.RandomID(), payload.RandomID(), tx)
	{
		cachedTransaction, cachedTransactionMetadata, cachedAttachment, transactionIsNew := tangle.storeTransactionModels(valueObject2)
		cachedTransaction.Consume(func(transaction *transaction.Transaction) {
			assert.True(t, assert.ObjectsAreEqual(tx, transaction))
		})
		cachedTransactionMetadata.Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, tx.ID(), transactionMetadata.ID())
		})
		expectedAttachment := NewAttachment(tx.ID(), valueObject2.ID())
		cachedAttachment.Consume(func(attachment *Attachment) {
			assert.Equal(t, expectedAttachment.TransactionID(), attachment.TransactionID())
			assert.Equal(t, expectedAttachment.PayloadID(), attachment.PayloadID())
		})
		assert.False(t, transactionIsNew)
	}

	// retrieve from tangle
	{
		cachedTransaction := tangle.Transaction(tx.ID())
		cachedTransaction.Consume(func(transaction *transaction.Transaction) {
			assert.True(t, assert.ObjectsAreEqual(tx, transaction))
		})
		cachedTransactionMetadata := tangle.TransactionMetadata(tx.ID())
		cachedTransactionMetadata.Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Equal(t, tx.ID(), transactionMetadata.ID())
		})

		// check created consumers
		// TODO: only reason that there could be multiple consumers = conflict, e.g. 2 tx use same inputs?
		tx.Inputs().ForEach(func(inputId transaction.OutputID) bool {
			expectedConsumer := NewConsumer(inputId, tx.ID())
			tangle.Consumers(inputId).Consume(func(consumer *Consumer) {
				assert.Equal(t, expectedConsumer.ConsumedInput(), consumer.ConsumedInput())
				assert.Equal(t, expectedConsumer.TransactionID(), consumer.TransactionID())
			})
			return true
		})

		cachedAttachments := tangle.Attachments(tx.ID())
		assert.Len(t, cachedAttachments, 2)
		attachmentPayloads := []payload.ID{valueObject.ID(), valueObject2.ID()}
		cachedAttachments.Consume(func(attachment *Attachment) {
			assert.Equal(t, tx.ID(), attachment.TransactionID())
			assert.Contains(t, attachmentPayloads, attachment.PayloadID())
		})
	}
}

// TestStorePayloadReferences checks whether approvers are correctly created.
func TestStorePayloadReferences(t *testing.T) {
	tangle := New(mapdb.NewMapDB())

	tx := createDummyTransaction()
	parent1 := payload.RandomID()
	parent2 := payload.RandomID()
	valueObject1 := payload.New(parent1, parent2, tx)

	{
		tangle.storePayloadReferences(valueObject1)

		// check for approvers
		approversParent1 := tangle.Approvers(parent1)
		assert.Len(t, approversParent1, 1)
		approversParent1.Consume(func(approver *PayloadApprover) {
			assert.Equal(t, parent1, approver.referencedPayloadID)
			assert.Equal(t, valueObject1.ID(), approver.ApprovingPayloadID())
		})

		approversParent2 := tangle.Approvers(parent2)
		assert.Len(t, approversParent2, 1)
		approversParent2.Consume(func(approver *PayloadApprover) {
			assert.Equal(t, parent2, approver.referencedPayloadID)
			assert.Equal(t, valueObject1.ID(), approver.ApprovingPayloadID())
		})
	}

	valueObject2 := payload.New(parent1, parent2, createDummyTransaction())
	{
		tangle.storePayloadReferences(valueObject2)

		// check for approvers
		approversParent1 := tangle.Approvers(parent1)
		assert.Len(t, approversParent1, 2)
		valueObjectIDs := []payload.ID{valueObject1.ID(), valueObject2.ID()}
		approversParent1.Consume(func(approver *PayloadApprover) {
			assert.Equal(t, parent1, approver.referencedPayloadID)
			assert.Contains(t, valueObjectIDs, approver.ApprovingPayloadID())
		})

		approversParent2 := tangle.Approvers(parent2)
		assert.Len(t, approversParent2, 2)
		approversParent2.Consume(func(approver *PayloadApprover) {
			assert.Equal(t, parent2, approver.referencedPayloadID)
			assert.Contains(t, valueObjectIDs, approver.ApprovingPayloadID())
		})
	}
}

func createDummyTransaction() *transaction.Transaction {
	return transaction.New(
		// inputs
		transaction.NewInputs(
			transaction.NewOutputID(address.Random(), transaction.RandomID()),
			transaction.NewOutputID(address.Random(), transaction.RandomID()),
		),

		// outputs
		transaction.NewOutputs(map[address.Address][]*balance.Balance{
			address.Random(): {
				balance.New(balance.ColorIOTA, 1337),
			},
		}),
	)
}
