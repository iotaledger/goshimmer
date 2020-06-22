package tangle

import (
	"container/list"
	"math"
	"testing"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/branchmanager"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSetTransactionPreferred(t *testing.T) {
	tangle := New(mapdb.NewMapDB())
	event := newEventTangle(t, tangle)
	defer event.DetachAll()

	tx := createDummyTransaction()
	valueObject := payload.New(payload.GenesisID, payload.GenesisID, tx)
	tangle.storeTransactionModels(valueObject)

	event.Expect("TransactionPreferred", tx, mock.Anything)

	modified, err := tangle.SetTransactionPreferred(tx.ID(), true)
	require.NoError(t, err)
	assert.True(t, modified)

	event.AssertExpectations(t)
}

// TestBookTransaction tests the following cases:
// - missing output
// - transaction already booked by another process
// - booking first spend
// - booking double spend
func TestBookTransaction(t *testing.T) {

	// CASE: missing output
	t.Run("CASE: missing output", func(t *testing.T) {
		tangle := New(mapdb.NewMapDB())
		event := newEventTangle(t, tangle)
		defer event.DetachAll()

		tx := createDummyTransaction()
		valueObject := payload.New(payload.GenesisID, payload.GenesisID, tx)

		cachedTransaction, cachedTransactionMetadata, _, transactionIsNew := tangle.storeTransactionModels(valueObject)
		assert.True(t, transactionIsNew)

		event.Expect("TransactionSolid", tx, mock.Anything)

		// manually trigger a booking: tx will be marked solid, but it cannot be book as its inputs are unavailable
		transactionBooked, decisionPending, err := tangle.bookTransaction(cachedTransaction, cachedTransactionMetadata)
		assert.False(t, transactionBooked)
		assert.False(t, decisionPending)
		assert.Error(t, err)

		event.AssertExpectations(t)
	})

	// CASE: transaction already booked by another process
	t.Run("CASE: transaction already booked by another process", func(t *testing.T) {
		tangle := New(mapdb.NewMapDB())
		event := newEventTangle(t, tangle)
		defer event.DetachAll()

		tx := createDummyTransaction()
		valueObject := payload.New(payload.GenesisID, payload.GenesisID, tx)
		cachedTransaction, cachedTransactionMetadata, _, _ := tangle.storeTransactionModels(valueObject)

		transactionMetadata := cachedTransactionMetadata.Unwrap()
		transactionMetadata.setSolid(true)

		transactionBooked, decisionPending, err := tangle.bookTransaction(cachedTransaction, cachedTransactionMetadata)
		require.NoError(t, err)
		assert.False(t, transactionBooked)
		assert.False(t, decisionPending)

		event.AssertExpectations(t)
	})

	// CASE: booking first spend
	t.Run("CASE: booking first spend", func(t *testing.T) {
		tangle := New(mapdb.NewMapDB())
		event := newEventTangle(t, tangle)
		defer event.DetachAll()

		// prepare snapshot
		color1 := [32]byte{1}
		outputs := map[address.Address][]*balance.Balance{
			address.Random(): {
				balance.New(balance.ColorIOTA, 1),
			},
			address.Random(): {
				balance.New(balance.ColorIOTA, 2),
				balance.New(color1, 3),
			},
		}
		inputIDs := loadSnapshotFromOutputs(tangle, outputs)

		// build first spending
		tx1 := transaction.New(
			transaction.NewInputs(inputIDs...),
			// outputs
			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				address.Random(): {
					balance.New(balance.ColorIOTA, 3),
					balance.New(color1, 3),
				},
			}),
		)

		valueObject := payload.New(payload.GenesisID, payload.GenesisID, tx1)
		cachedTransaction, cachedTransactionMetadata, _, _ := tangle.storeTransactionModels(valueObject)
		txMetadata := cachedTransactionMetadata.Unwrap()

		// assert that branchID is undefined before being booked
		assert.Equal(t, branchmanager.UndefinedBranchID, txMetadata.BranchID())

		event.Expect("TransactionSolid", tx1, mock.Anything)
		// TransactionBooked is triggered outside of bookTransaction

		transactionBooked, decisionPending, err := tangle.bookTransaction(cachedTransaction, cachedTransactionMetadata)
		require.NoError(t, err)
		assert.True(t, transactionBooked, "transactionBooked")
		assert.True(t, decisionPending, "decisionPending")

		// assert that branchID is the same as the MasterBranchID
		assert.Equal(t, branchmanager.MasterBranchID, txMetadata.BranchID())

		// CASE: booking double spend
		t.Run("CASE: booking double spend", func(t *testing.T) {
			// build second spending
			tx2 := transaction.New(
				transaction.NewInputs(inputIDs...),
				// outputs
				transaction.NewOutputs(map[address.Address][]*balance.Balance{
					address.Random(): {
						balance.New(balance.ColorIOTA, 3),
						balance.New(color1, 3),
					},
				}),
			)

			valueObject := payload.New(payload.GenesisID, payload.GenesisID, tx2)
			cachedTransaction, cachedTransactionMetadata, _, _ = tangle.storeTransactionModels(valueObject)
			txMetadata := cachedTransactionMetadata.Unwrap()

			// assert that branchID is undefined before being booked
			assert.Equal(t, branchmanager.UndefinedBranchID, txMetadata.BranchID())

			// manually book the double spending tx2, this will mark it as solid and trigger a fork
			event.Expect("TransactionSolid", tx2, mock.Anything)
			event.Expect("Fork", tx1, mock.Anything, mock.Anything, inputIDs)

			transactionBooked, decisionPending, err := tangle.bookTransaction(cachedTransaction, cachedTransactionMetadata)
			require.NoError(t, err)
			assert.True(t, transactionBooked, "transactionBooked")
			assert.True(t, decisionPending, "decisionPending")

			// assert that first spend and double spend have different BranchIDs
			assert.NotEqual(t, branchmanager.MasterBranchID, txMetadata.BranchID(), "BranchID")
		})

		event.AssertExpectations(t)
	})
}

func TestCalculateBranchOfTransaction(t *testing.T) {

	// CASE: missing output
	t.Run("CASE: missing output", func(t *testing.T) {
		tangle := New(mapdb.NewMapDB())
		tx := createDummyTransaction()
		cachedBranch, err := tangle.calculateBranchOfTransaction(tx)
		assert.Error(t, err)
		assert.Nil(t, cachedBranch)
	})

	// CASE: same as master branch
	t.Run("CASE: same as master branch", func(t *testing.T) {
		tangle := New(mapdb.NewMapDB())

		// prepare snapshot
		color1 := [32]byte{1}
		outputs := map[address.Address][]*balance.Balance{
			address.Random(): {
				balance.New(balance.ColorIOTA, 1),
			},
			address.Random(): {
				balance.New(balance.ColorIOTA, 2),
				balance.New(color1, 3),
			},
		}
		inputIDs := loadSnapshotFromOutputs(tangle, outputs)

		tx := transaction.New(
			transaction.NewInputs(inputIDs...),
			// outputs
			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				address.Random(): {
					balance.New(balance.ColorIOTA, 3),
					balance.New(color1, 3),
				},
			}),
		)

		cachedBranch, err := tangle.calculateBranchOfTransaction(tx)
		require.NoError(t, err)
		assert.Equal(t, branchmanager.MasterBranchID, cachedBranch.Unwrap().ID())
	})
}

func TestMoveTransactionToBranch(t *testing.T) {
	tangle := New(mapdb.NewMapDB())
	// prepare snapshot
	color1 := [32]byte{1}
	outputs := map[address.Address][]*balance.Balance{
		address.Random(): {
			balance.New(balance.ColorIOTA, 1),
		},
		address.Random(): {
			balance.New(balance.ColorIOTA, 2),
			balance.New(color1, 3),
		},
	}
	inputIDs := loadSnapshotFromOutputs(tangle, outputs)

	tx := transaction.New(
		transaction.NewInputs(inputIDs...),
		// outputs
		transaction.NewOutputs(map[address.Address][]*balance.Balance{
			address.Random(): {
				balance.New(balance.ColorIOTA, 3),
				balance.New(color1, 3),
			},
		}),
	)
	valueObject := payload.New(payload.GenesisID, payload.GenesisID, tx)
	cachedTransaction, cachedTransactionMetadata, _, _ := tangle.storeTransactionModels(valueObject)
	txMetadata := cachedTransactionMetadata.Unwrap()

	// create conflicting branche
	cachedBranch2, _ := tangle.BranchManager().Fork(branchmanager.BranchID{2}, []branchmanager.BranchID{branchmanager.MasterBranchID}, []branchmanager.ConflictID{{0}})
	defer cachedBranch2.Release()

	err := tangle.moveTransactionToBranch(cachedTransaction.Retain(), cachedTransactionMetadata.Retain(), cachedBranch2.Retain())
	require.NoError(t, err)
	assert.Equal(t, branchmanager.BranchID{2}, txMetadata.BranchID())
}

func TestFork(t *testing.T) {
	// CASE: already finalized
	t.Run("CASE: already finalized", func(t *testing.T) {
		tangle := New(mapdb.NewMapDB())
		event := newEventTangle(t, tangle)
		defer event.DetachAll()

		// prepare snapshot
		color1 := [32]byte{1}
		outputs := map[address.Address][]*balance.Balance{
			address.Random(): {
				balance.New(balance.ColorIOTA, 1),
			},
			address.Random(): {
				balance.New(balance.ColorIOTA, 2),
				balance.New(color1, 3),
			},
		}
		inputIDs := loadSnapshotFromOutputs(tangle, outputs)

		tx := transaction.New(
			transaction.NewInputs(inputIDs...),
			// outputs
			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				address.Random(): {
					balance.New(balance.ColorIOTA, 3),
					balance.New(color1, 3),
				},
			}),
		)
		valueObject := payload.New(payload.GenesisID, payload.GenesisID, tx)
		_, cachedTransactionMetadata, _, _ := tangle.storeTransactionModels(valueObject)
		txMetadata := cachedTransactionMetadata.Unwrap()

		txMetadata.setFinalized(true)

		// no fork created so no event should be triggered
		forked, finalized, err := tangle.Fork(tx.ID(), []transaction.OutputID{})
		require.NoError(t, err)
		assert.False(t, forked)
		assert.True(t, finalized)

		event.AssertExpectations(t)
	})

	t.Run("CASE: normal fork", func(t *testing.T) {
		tangle := New(mapdb.NewMapDB())
		event := newEventTangle(t, tangle)
		defer event.DetachAll()

		// prepare snapshot
		color1 := [32]byte{1}
		outputs := map[address.Address][]*balance.Balance{
			address.Random(): {
				balance.New(balance.ColorIOTA, 1),
			},
			address.Random(): {
				balance.New(balance.ColorIOTA, 2),
				balance.New(color1, 3),
			},
		}
		inputIDs := loadSnapshotFromOutputs(tangle, outputs)

		tx := transaction.New(
			transaction.NewInputs(inputIDs...),
			// outputs
			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				address.Random(): {
					balance.New(balance.ColorIOTA, 3),
					balance.New(color1, 3),
				},
			}),
		)
		valueObject := payload.New(payload.GenesisID, payload.GenesisID, tx)
		tangle.storeTransactionModels(valueObject)

		event.Expect("Fork", tx, mock.Anything, mock.Anything, []transaction.OutputID{})

		forked, finalized, err := tangle.Fork(tx.ID(), []transaction.OutputID{})
		require.NoError(t, err)
		assert.True(t, forked, "forked")
		assert.False(t, finalized, "finalized")

		t.Run("CASE: branch existed already", func(t *testing.T) {
			// no fork created so no event should be triggered
			forked, finalized, err = tangle.Fork(tx.ID(), []transaction.OutputID{})
			require.NoError(t, err)
			assert.False(t, forked, "forked")
			assert.False(t, finalized, "finalized")
		})

		event.AssertExpectations(t)
	})
}

func TestBookPayload(t *testing.T) {
	t.Run("CASE: undefined branchID", func(t *testing.T) {
		tangle := New(mapdb.NewMapDB())
		event := newEventTangle(t, tangle)
		defer event.DetachAll()

		valueObject := payload.New(payload.GenesisID, payload.GenesisID, createDummyTransaction())
		cachedPayload, cachedMetadata, _ := tangle.storePayload(valueObject)
		_, cachedTransactionMetadata, _, _ := tangle.storeTransactionModels(valueObject)

		payloadBooked, err := tangle.bookPayload(cachedPayload.Retain(), cachedMetadata.Retain(), cachedTransactionMetadata.Retain())
		defer func() {
			cachedPayload.Release()
			cachedMetadata.Release()
			cachedTransactionMetadata.Release()
		}()

		require.NoError(t, err)
		assert.False(t, payloadBooked, "payloadBooked")

		event.AssertExpectations(t)
	})

	t.Run("CASE: successfully book", func(t *testing.T) {
		tangle := New(mapdb.NewMapDB())
		event := newEventTangle(t, tangle)
		defer event.DetachAll()

		valueObject := payload.New(payload.GenesisID, payload.GenesisID, createDummyTransaction())
		cachedPayload, cachedMetadata, _ := tangle.storePayload(valueObject)
		metadata := cachedMetadata.Unwrap()

		metadata.setBranchID(branchmanager.BranchID{1})
		metadata.setBranchID(branchmanager.BranchID{2})

		_, cachedTransactionMetadata, _, _ := tangle.storeTransactionModels(valueObject)
		txMetadata := cachedTransactionMetadata.Unwrap()
		txMetadata.setBranchID(branchmanager.BranchID{1})

		event.Expect("PayloadSolid", valueObject, mock.Anything)
		payloadBooked, err := tangle.bookPayload(cachedPayload.Retain(), cachedMetadata.Retain(), cachedTransactionMetadata.Retain())
		defer func() {
			cachedPayload.Release()
			cachedMetadata.Release()
			cachedTransactionMetadata.Release()
		}()

		require.NoError(t, err)
		assert.True(t, payloadBooked, "payloadBooked")

		event.AssertExpectations(t)
	})

	t.Run("CASE: not booked", func(t *testing.T) {
		tangle := New(mapdb.NewMapDB())
		event := newEventTangle(t, tangle)
		defer event.DetachAll()

		valueObject := payload.New(payload.GenesisID, payload.GenesisID, createDummyTransaction())
		cachedPayload, cachedMetadata, _ := tangle.storePayload(valueObject)
		metadata := cachedMetadata.Unwrap()

		metadata.setBranchID(branchmanager.BranchID{1})
		metadata.setBranchID(branchmanager.BranchID{1})

		_, cachedTransactionMetadata, _, _ := tangle.storeTransactionModels(valueObject)
		txMetadata := cachedTransactionMetadata.Unwrap()
		txMetadata.setBranchID(branchmanager.BranchID{1})

		event.Expect("PayloadSolid", valueObject, mock.Anything)
		payloadBooked, err := tangle.bookPayload(cachedPayload.Retain(), cachedMetadata.Retain(), cachedTransactionMetadata.Retain())
		defer func() {
			cachedPayload.Release()
			cachedMetadata.Release()
			cachedTransactionMetadata.Release()
		}()

		require.NoError(t, err)
		assert.False(t, payloadBooked, "payloadBooked")

		event.AssertExpectations(t)
	})

}

// TestStorePayload checks whether a value object is correctly stored.
func TestStorePayload(t *testing.T) {
	tangle := New(mapdb.NewMapDB())
	event := newEventTangle(t, tangle)
	defer event.DetachAll()

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

	event.AssertExpectations(t)
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
		// only reason that there could be multiple consumers = conflict, e.g. 2 tx use same inputs?
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

// TestCheckTransactionOutputs checks whether inputs and outputs are correctly reconciled.
func TestCheckTransactionOutputs(t *testing.T) {
	tangle := New(mapdb.NewMapDB())

	// test happy cases with ColorIOTA
	{
		consumedBalances := make(map[balance.Color]int64)
		consumedBalances[balance.ColorIOTA] = 1000

		outputs := transaction.NewOutputs(map[address.Address][]*balance.Balance{
			address.Random(): {
				balance.New(balance.ColorIOTA, 1000),
			},
		})
		assert.True(t, tangle.checkTransactionOutputs(consumedBalances, outputs))
	}
	{
		consumedBalances := make(map[balance.Color]int64)
		consumedBalances[balance.ColorIOTA] = math.MaxInt64

		outputs := transaction.NewOutputs(map[address.Address][]*balance.Balance{
			address.Random(): {
				balance.New(balance.ColorIOTA, math.MaxInt64),
			},
		})
		assert.True(t, tangle.checkTransactionOutputs(consumedBalances, outputs))
	}
	{
		consumedBalances := make(map[balance.Color]int64)
		consumedBalances[balance.ColorIOTA] = 25123

		outputs := transaction.NewOutputs(map[address.Address][]*balance.Balance{
			address.Random(): {
				balance.New(balance.ColorIOTA, 122),
			},
			address.Random(): {
				balance.New(balance.ColorIOTA, 1),
			},
			address.Random(): {
				balance.New(balance.ColorIOTA, 5000),
			},
			address.Random(): {
				balance.New(balance.ColorIOTA, 20000),
			},
		})
		assert.True(t, tangle.checkTransactionOutputs(consumedBalances, outputs))
	}

	// test wrong balances
	{
		consumedBalances := make(map[balance.Color]int64)
		consumedBalances[balance.ColorIOTA] = 1000

		outputs := transaction.NewOutputs(map[address.Address][]*balance.Balance{
			address.Random(): {
				balance.New(balance.ColorIOTA, 122),
			},
			address.Random(): {
				balance.New(balance.ColorIOTA, 1),
			},
			address.Random(): {
				balance.New(balance.ColorIOTA, 5000),
			},
			address.Random(): {
				balance.New(balance.ColorIOTA, 20000),
			},
		})
		assert.False(t, tangle.checkTransactionOutputs(consumedBalances, outputs))
	}

	// test input overflow
	{
		consumedBalances := make(map[balance.Color]int64)
		consumedBalances[balance.ColorIOTA] = math.MaxInt64
		consumedBalances[[32]byte{1}] = 1

		outputs := transaction.NewOutputs(map[address.Address][]*balance.Balance{
			address.Random(): {
				balance.New(balance.ColorIOTA, 1000),
			},
		})
		assert.False(t, tangle.checkTransactionOutputs(consumedBalances, outputs))
	}

	// 0, negative outputs and overflows
	{
		consumedBalances := make(map[balance.Color]int64)
		//consumedBalances[balance.ColorIOTA] = 1000

		outputs := transaction.NewOutputs(map[address.Address][]*balance.Balance{
			address.Random(): {
				balance.New(balance.ColorIOTA, -1),
			},
		})
		assert.False(t, tangle.checkTransactionOutputs(consumedBalances, outputs))

		outputs = transaction.NewOutputs(map[address.Address][]*balance.Balance{
			address.Random(): {
				balance.New(balance.ColorIOTA, 0),
			},
		})
		assert.False(t, tangle.checkTransactionOutputs(consumedBalances, outputs))

		outputs = transaction.NewOutputs(map[address.Address][]*balance.Balance{
			address.Random(): {
				balance.New(balance.ColorIOTA, 1),
			},
			address.Random(): {
				balance.New(balance.ColorIOTA, math.MaxInt64),
			},
		})
		assert.False(t, tangle.checkTransactionOutputs(consumedBalances, outputs))
	}

	// test happy cases with ColorNew
	{
		consumedBalances := make(map[balance.Color]int64)
		consumedBalances[balance.ColorIOTA] = 1000

		outputs := transaction.NewOutputs(map[address.Address][]*balance.Balance{
			address.Random(): {
				balance.New(balance.ColorNew, 333),
			},
			address.Random(): {
				balance.New(balance.ColorNew, 333),
			},
			address.Random(): {
				balance.New(balance.ColorNew, 334),
			},
		})
		assert.True(t, tangle.checkTransactionOutputs(consumedBalances, outputs))
	}

	// test wrong balances
	{
		consumedBalances := make(map[balance.Color]int64)
		consumedBalances[balance.ColorIOTA] = 1000

		outputs := transaction.NewOutputs(map[address.Address][]*balance.Balance{
			address.Random(): {
				balance.New(balance.ColorNew, 122),
			},
			address.Random(): {
				balance.New(balance.ColorNew, 1),
			},
		})
		assert.False(t, tangle.checkTransactionOutputs(consumedBalances, outputs))
	}

	// 0, negative outputs and overflows
	{
		consumedBalances := make(map[balance.Color]int64)
		//consumedBalances[balance.ColorIOTA] = 1000

		outputs := transaction.NewOutputs(map[address.Address][]*balance.Balance{
			address.Random(): {
				balance.New(balance.ColorNew, -1),
			},
		})
		assert.False(t, tangle.checkTransactionOutputs(consumedBalances, outputs))

		outputs = transaction.NewOutputs(map[address.Address][]*balance.Balance{
			address.Random(): {
				balance.New(balance.ColorNew, 0),
			},
		})
		assert.False(t, tangle.checkTransactionOutputs(consumedBalances, outputs))

		outputs = transaction.NewOutputs(map[address.Address][]*balance.Balance{
			address.Random(): {
				balance.New(balance.ColorNew, 1),
			},
			address.Random(): {
				balance.New(balance.ColorNew, math.MaxInt64),
			},
		})
		assert.False(t, tangle.checkTransactionOutputs(consumedBalances, outputs))
	}

	// test happy case with colors
	{
		color1 := [32]byte{1}
		color2 := [32]byte{2}

		consumedBalances := make(map[balance.Color]int64)
		consumedBalances[color1] = 1000
		consumedBalances[color2] = 25123

		outputs := transaction.NewOutputs(map[address.Address][]*balance.Balance{
			address.Random(): {
				balance.New(color1, 333),
			},
			address.Random(): {
				balance.New(color1, 333),
			},
			address.Random(): {
				balance.New(color1, 334),
			},
			address.Random(): {
				balance.New(color2, 25000),
			},
			address.Random(): {
				balance.New(color2, 123),
			},
		})
		assert.True(t, tangle.checkTransactionOutputs(consumedBalances, outputs))
	}

	// try to spend color that is not in inputs
	{
		color1 := [32]byte{1}
		color2 := [32]byte{2}

		consumedBalances := make(map[balance.Color]int64)
		consumedBalances[color1] = 1000

		outputs := transaction.NewOutputs(map[address.Address][]*balance.Balance{
			address.Random(): {
				balance.New(color1, 333),
			},
			address.Random(): {
				balance.New(color1, 333),
			},
			address.Random(): {
				balance.New(color1, 334),
			},
			address.Random(): {
				balance.New(color2, 25000),
			},
		})
		assert.False(t, tangle.checkTransactionOutputs(consumedBalances, outputs))
	}

	// try to spend more than in inputs of color
	{
		color1 := [32]byte{1}

		consumedBalances := make(map[balance.Color]int64)
		consumedBalances[color1] = 1000

		outputs := transaction.NewOutputs(map[address.Address][]*balance.Balance{
			address.Random(): {
				balance.New(color1, math.MaxInt64),
			},
			address.Random(): {
				balance.New(color1, math.MaxInt64),
			},
		})
		assert.False(t, tangle.checkTransactionOutputs(consumedBalances, outputs))
	}

	// combine unspent colors and colors
	{
		color1 := [32]byte{1}
		color2 := [32]byte{2}

		consumedBalances := make(map[balance.Color]int64)
		consumedBalances[color1] = 1000
		consumedBalances[color2] = 1000
		consumedBalances[balance.ColorIOTA] = 1000

		outputs := transaction.NewOutputs(map[address.Address][]*balance.Balance{
			address.Random(): {
				balance.New(color1, 1000),
				balance.New(color2, 500),
				balance.New(balance.ColorNew, 500),
			},
			address.Random(): {
				balance.New(balance.ColorNew, 1000),
			},
		})
		assert.True(t, tangle.checkTransactionOutputs(consumedBalances, outputs))
	}
}

func TestGetCachedOutputsFromTransactionInputs(t *testing.T) {
	tangle := New(mapdb.NewMapDB())

	color1 := [32]byte{1}

	// prepare inputs for tx that we want to retrieve from tangle
	outputs := map[address.Address][]*balance.Balance{
		address.Random(): {
			balance.New(balance.ColorIOTA, 1),
		},
		address.Random(): {
			balance.New(balance.ColorIOTA, 2),
			balance.New(color1, 3),
		},
	}
	inputIDs := loadSnapshotFromOutputs(tangle, outputs)

	// build tx2 spending "outputs"
	tx2 := transaction.New(
		transaction.NewInputs(inputIDs...),
		// outputs
		transaction.NewOutputs(map[address.Address][]*balance.Balance{
			address.Random(): {
				balance.New(balance.ColorIOTA, 1337),
			},
		}),
	)

	// verify that outputs are retrieved correctly
	{
		cachedOutputs := tangle.getCachedOutputsFromTransactionInputs(tx2)
		assert.Len(t, cachedOutputs, len(outputs))
		cachedOutputs.Consume(func(output *Output) {
			assert.ElementsMatch(t, outputs[output.Address()], output.Balances())
		})
	}
}

func TestRetrieveConsumedInputDetails(t *testing.T) {
	// test simple happy case
	{
		tangle := New(mapdb.NewMapDB())

		color1 := [32]byte{1}

		// prepare inputs for tx that we want to retrieve from tangle
		outputs := map[address.Address][]*balance.Balance{
			address.Random(): {
				balance.New(balance.ColorIOTA, 1),
			},
			address.Random(): {
				balance.New(balance.ColorIOTA, 2),
				balance.New(color1, 3),
			},
		}
		inputIDs := loadSnapshotFromOutputs(tangle, outputs)
		tx := transaction.New(
			transaction.NewInputs(inputIDs...),
			// outputs
			transaction.NewOutputs(map[address.Address][]*balance.Balance{}),
		)

		inputsSolid, cachedInputs, consumedBalances, consumedBranches, err := tangle.retrieveConsumedInputDetails(tx)
		require.NoError(t, err)
		assert.True(t, inputsSolid)
		assert.Len(t, cachedInputs, len(outputs))
		cachedInputs.Consume(func(input *Output) {
			assert.ElementsMatch(t, outputs[input.Address()], input.Balances())
		})
		assert.Equal(t, sumOutputsByColor(outputs), consumedBalances)
		assert.Len(t, consumedBranches, 1)
		assert.Contains(t, consumedBranches, branchmanager.MasterBranchID)
	}

	// test happy case with more colors
	{
		tangle := New(mapdb.NewMapDB())

		color1 := [32]byte{1}
		color2 := [32]byte{2}
		color3 := [32]byte{3}

		// prepare inputs for tx that we want to retrieve from tangle
		outputs := map[address.Address][]*balance.Balance{
			address.Random(): {
				balance.New(color1, 1000),
			},
			address.Random(): {
				balance.New(color2, 210),
				balance.New(color1, 3),
			},
			address.Random(): {
				balance.New(color3, 5621),
				balance.New(color1, 3),
			},
		}
		// build tx spending "outputs"
		inputIDs := loadSnapshotFromOutputs(tangle, outputs)
		tx := transaction.New(
			transaction.NewInputs(inputIDs...),
			// outputs
			transaction.NewOutputs(map[address.Address][]*balance.Balance{}),
		)

		inputsSolid, cachedInputs, consumedBalances, consumedBranches, err := tangle.retrieveConsumedInputDetails(tx)
		require.NoError(t, err)
		assert.True(t, inputsSolid)
		assert.Len(t, cachedInputs, len(outputs))
		cachedInputs.Consume(func(input *Output) {
			assert.ElementsMatch(t, outputs[input.Address()], input.Balances())
		})
		assert.Equal(t, sumOutputsByColor(outputs), consumedBalances)
		assert.Len(t, consumedBranches, 1)
		assert.Contains(t, consumedBranches, branchmanager.MasterBranchID)
	}

	// test int overflow
	{
		tangle := New(mapdb.NewMapDB())

		// prepare inputs for tx that we want to retrieve from tangle
		outputs := map[address.Address][]*balance.Balance{
			address.Random(): {
				balance.New(balance.ColorIOTA, 1),
			},
			address.Random(): {
				balance.New(balance.ColorIOTA, math.MaxInt64),
			},
		}
		inputIDs := loadSnapshotFromOutputs(tangle, outputs)
		tx := transaction.New(
			transaction.NewInputs(inputIDs...),
			// outputs
			transaction.NewOutputs(map[address.Address][]*balance.Balance{}),
		)

		inputsSolid, cachedInputs, _, _, err := tangle.retrieveConsumedInputDetails(tx)
		assert.Error(t, err)
		assert.False(t, inputsSolid)
		assert.Len(t, cachedInputs, len(outputs))
	}

	// test multiple consumed branches
	{
		tangle := New(mapdb.NewMapDB())

		// prepare inputs for tx that we want to retrieve from tangle
		outputs := map[address.Address][]*balance.Balance{
			address.Random(): {
				balance.New(balance.ColorIOTA, 1),
			},
			address.Random(): {
				balance.New(balance.ColorIOTA, 2),
			},
		}
		inputIDs := loadSnapshotFromOutputs(tangle, outputs)
		tx := transaction.New(
			transaction.NewInputs(inputIDs...),
			// outputs
			transaction.NewOutputs(map[address.Address][]*balance.Balance{}),
		)

		// modify branch of 1 output
		newBranch := branchmanager.NewBranchID(transaction.RandomID())
		output := tangle.TransactionOutput(inputIDs[0])
		output.Consume(func(output *Output) {
			output.branchID = newBranch
		})

		inputsSolid, cachedInputs, consumedBalances, consumedBranches, err := tangle.retrieveConsumedInputDetails(tx)
		require.NoError(t, err)
		assert.True(t, inputsSolid)
		assert.Len(t, cachedInputs, len(outputs))
		cachedInputs.Consume(func(input *Output) {
			assert.ElementsMatch(t, outputs[input.Address()], input.Balances())
		})
		assert.Equal(t, sumOutputsByColor(outputs), consumedBalances)
		assert.Len(t, consumedBranches, 2)
		assert.Contains(t, consumedBranches, branchmanager.MasterBranchID)
		assert.Contains(t, consumedBranches, newBranch)
	}
}

func TestCheckTransactionSolidity(t *testing.T) {
	// already solid tx
	{
		tangle := New(mapdb.NewMapDB())
		tx := createDummyTransaction()
		txMetadata := NewTransactionMetadata(tx.ID())
		txMetadata.setSolid(true)
		txMetadata.setBranchID(branchmanager.MasterBranchID)

		solid, consumedBranches, err := tangle.checkTransactionSolidity(tx, txMetadata)
		assert.True(t, solid)
		assert.Len(t, consumedBranches, 1)
		assert.Contains(t, consumedBranches, branchmanager.MasterBranchID)
		assert.NoError(t, err)
	}

	// deleted tx
	{
		tangle := New(mapdb.NewMapDB())
		tx := createDummyTransaction()
		txMetadata := NewTransactionMetadata(tx.ID())
		tx.Delete()
		txMetadata.Delete()

		solid, consumedBranches, _ := tangle.checkTransactionSolidity(tx, txMetadata)
		assert.False(t, solid)
		assert.Len(t, consumedBranches, 0)
		//assert.Error(t, err)
	}

	// invalid tx: inputs not solid/non-existing
	{
		tangle := New(mapdb.NewMapDB())
		tx := createDummyTransaction()
		txMetadata := NewTransactionMetadata(tx.ID())

		solid, consumedBranches, err := tangle.checkTransactionSolidity(tx, txMetadata)
		assert.False(t, solid)
		assert.Len(t, consumedBranches, 0)
		assert.NoError(t, err)
	}

	// invalid tx: inputs do not match outputs
	{
		tangle := New(mapdb.NewMapDB())

		// prepare snapshot
		color1 := [32]byte{1}
		outputs := map[address.Address][]*balance.Balance{
			address.Random(): {
				balance.New(balance.ColorIOTA, 1),
			},
			address.Random(): {
				balance.New(balance.ColorIOTA, 2),
				balance.New(color1, 3),
			},
		}
		inputIDs := loadSnapshotFromOutputs(tangle, outputs)

		// build tx spending wrong "outputs"
		tx := transaction.New(
			transaction.NewInputs(inputIDs...),
			// outputs
			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				address.Random(): {
					balance.New(balance.ColorIOTA, 11337),
					balance.New(color1, 1000),
				},
			}),
		)
		txMetadata := NewTransactionMetadata(tx.ID())

		solid, consumedBranches, err := tangle.checkTransactionSolidity(tx, txMetadata)
		assert.False(t, solid)
		assert.Len(t, consumedBranches, 0)
		assert.Error(t, err)
	}

	// spent outputs from master branch (non-conflicting branches)
	{
		tangle := New(mapdb.NewMapDB())

		// prepare snapshot
		color1 := [32]byte{1}
		outputs := map[address.Address][]*balance.Balance{
			address.Random(): {
				balance.New(balance.ColorIOTA, 1),
			},
			address.Random(): {
				balance.New(balance.ColorIOTA, 2),
				balance.New(color1, 3),
			},
		}
		inputIDs := loadSnapshotFromOutputs(tangle, outputs)

		// build tx spending "outputs"
		tx := transaction.New(
			transaction.NewInputs(inputIDs...),
			// outputs
			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				address.Random(): {
					balance.New(balance.ColorIOTA, 3),
					balance.New(color1, 3),
				},
			}),
		)
		txMetadata := NewTransactionMetadata(tx.ID())

		solid, consumedBranches, err := tangle.checkTransactionSolidity(tx, txMetadata)
		assert.True(t, solid)
		assert.Len(t, consumedBranches, 1)
		assert.Contains(t, consumedBranches, branchmanager.MasterBranchID)
		assert.NoError(t, err)
	}

	// spent outputs from conflicting branches
	{
		tangle := New(mapdb.NewMapDB())

		// create conflicting branches
		cachedBranch2, _ := tangle.BranchManager().Fork(branchmanager.BranchID{2}, []branchmanager.BranchID{branchmanager.MasterBranchID}, []branchmanager.ConflictID{{0}})
		branch2 := cachedBranch2.Unwrap()
		defer cachedBranch2.Release()
		cachedBranch3, _ := tangle.BranchManager().Fork(branchmanager.BranchID{3}, []branchmanager.BranchID{branchmanager.MasterBranchID}, []branchmanager.ConflictID{{0}})
		branch3 := cachedBranch3.Unwrap()
		defer cachedBranch3.Release()
		// create outputs for conflicting branches
		inputIDs := make([]transaction.OutputID, 0)
		for _, branch := range []*branchmanager.Branch{branch2, branch3} {
			input := NewOutput(address.Random(), transaction.GenesisID, branch.ID(), []*balance.Balance{balance.New(balance.ColorIOTA, 1)})
			input.setSolid(true)
			cachedObject, _ := tangle.outputStorage.StoreIfAbsent(input)
			cachedOutput := &CachedOutput{CachedObject: cachedObject}
			cachedOutput.Consume(func(output *Output) {
				inputIDs = append(inputIDs, transaction.NewOutputID(output.Address(), transaction.GenesisID))
			})
		}

		// build tx spending "outputs" from conflicting branches
		tx := transaction.New(
			transaction.NewInputs(inputIDs...),
			// outputs
			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				address.Random(): {
					balance.New(balance.ColorIOTA, 2),
				},
			}),
		)
		txMetadata := NewTransactionMetadata(tx.ID())

		solid, consumedBranches, err := tangle.checkTransactionSolidity(tx, txMetadata)
		assert.False(t, solid)
		assert.Len(t, consumedBranches, 2)
		assert.Contains(t, consumedBranches, branch2.ID())
		assert.Contains(t, consumedBranches, branch3.ID())
		assert.Error(t, err)
	}

}

func TestPayloadBranchID(t *testing.T) {
	tangle := New(mapdb.NewMapDB())
	event := newEventTangle(t, tangle)
	defer event.DetachAll()

	{
		branchID := tangle.payloadBranchID(payload.GenesisID)
		assert.Equal(t, branchmanager.MasterBranchID, branchID)
	}

	// test with stored payload
	{
		valueObject := payload.New(payload.GenesisID, payload.GenesisID, createDummyTransaction())
		cachedPayload, cachedMetadata, stored := tangle.storePayload(valueObject)
		assert.True(t, stored)
		cachedPayload.Release()
		expectedBranchID := branchmanager.BranchID{1}
		cachedMetadata.Consume(func(metadata *PayloadMetadata) {
			metadata.setSolid(true)
			metadata.setBranchID(expectedBranchID)
		})

		branchID := tangle.payloadBranchID(valueObject.ID())
		assert.Equal(t, expectedBranchID, branchID)
	}

	// test missing value object
	{
		valueObject := payload.New(payload.GenesisID, payload.GenesisID, createDummyTransaction())

		event.Expect("PayloadMissing", valueObject.ID())
		branchID := tangle.payloadBranchID(valueObject.ID())
		assert.Equal(t, branchmanager.UndefinedBranchID, branchID)
	}

	event.AssertExpectations(t)
}

func TestCheckPayloadSolidity(t *testing.T) {
	tangle := New(mapdb.NewMapDB())
	event := newEventTangle(t, tangle)
	defer event.DetachAll()

	// check with already solid payload
	{
		valueObject := payload.New(payload.GenesisID, payload.GenesisID, createDummyTransaction())
		metadata := NewPayloadMetadata(valueObject.ID())
		metadata.setSolid(true)
		metadata.setBranchID(branchmanager.MasterBranchID)

		transactionBranches := []branchmanager.BranchID{branchmanager.MasterBranchID}
		solid, err := tangle.payloadBecameNewlySolid(valueObject, metadata, transactionBranches)
		assert.False(t, solid)
		assert.NoError(t, err)
	}

	// check with parents=genesis
	{
		valueObject := payload.New(payload.GenesisID, payload.GenesisID, createDummyTransaction())
		metadata := NewPayloadMetadata(valueObject.ID())

		transactionBranches := []branchmanager.BranchID{branchmanager.MasterBranchID}
		solid, err := tangle.payloadBecameNewlySolid(valueObject, metadata, transactionBranches)
		assert.True(t, solid)
		assert.NoError(t, err)
	}

	// check with solid parents and branch set
	{
		setParent := func(payloadMetadata *PayloadMetadata) {
			payloadMetadata.setSolid(true)
			payloadMetadata.setBranchID(branchmanager.MasterBranchID)
		}

		valueObject := payload.New(storeParentPayloadWithMetadataFunc(t, tangle, setParent), storeParentPayloadWithMetadataFunc(t, tangle, setParent), createDummyTransaction())
		metadata := NewPayloadMetadata(valueObject.ID())

		transactionBranches := []branchmanager.BranchID{branchmanager.MasterBranchID}
		solid, err := tangle.payloadBecameNewlySolid(valueObject, metadata, transactionBranches)
		assert.True(t, solid)
		assert.NoError(t, err)
	}

	// check with solid parents but no branch set -> should not be solid
	{
		setParent := func(payloadMetadata *PayloadMetadata) {
			payloadMetadata.setSolid(true)
		}

		valueObject := payload.New(storeParentPayloadWithMetadataFunc(t, tangle, setParent), storeParentPayloadWithMetadataFunc(t, tangle, setParent), createDummyTransaction())
		metadata := NewPayloadMetadata(valueObject.ID())

		transactionBranches := []branchmanager.BranchID{branchmanager.MasterBranchID}
		solid, err := tangle.payloadBecameNewlySolid(valueObject, metadata, transactionBranches)
		assert.False(t, solid)
		assert.NoError(t, err)
	}

	// conflicting branches of parents
	{
		// create conflicting branches
		cachedBranch2, _ := tangle.BranchManager().Fork(branchmanager.BranchID{2}, []branchmanager.BranchID{branchmanager.MasterBranchID}, []branchmanager.ConflictID{{0}})
		defer cachedBranch2.Release()
		cachedBranch3, _ := tangle.BranchManager().Fork(branchmanager.BranchID{3}, []branchmanager.BranchID{branchmanager.MasterBranchID}, []branchmanager.ConflictID{{0}})
		defer cachedBranch3.Release()
		setParent1 := func(payloadMetadata *PayloadMetadata) {
			payloadMetadata.setSolid(true)
			payloadMetadata.setBranchID(branchmanager.BranchID{2})
		}
		setParent2 := func(payloadMetadata *PayloadMetadata) {
			payloadMetadata.setSolid(true)
			payloadMetadata.setBranchID(branchmanager.BranchID{3})
		}

		valueObject := payload.New(storeParentPayloadWithMetadataFunc(t, tangle, setParent1), storeParentPayloadWithMetadataFunc(t, tangle, setParent2), createDummyTransaction())
		metadata := NewPayloadMetadata(valueObject.ID())

		transactionBranches := []branchmanager.BranchID{branchmanager.MasterBranchID}
		solid, err := tangle.payloadBecameNewlySolid(valueObject, metadata, transactionBranches)
		assert.False(t, solid)
		assert.Error(t, err)
	}

	// conflicting branches with transactions
	{
		// create conflicting branches
		cachedBranch2, _ := tangle.BranchManager().Fork(branchmanager.BranchID{2}, []branchmanager.BranchID{branchmanager.MasterBranchID}, []branchmanager.ConflictID{{0}})
		defer cachedBranch2.Release()
		cachedBranch3, _ := tangle.BranchManager().Fork(branchmanager.BranchID{3}, []branchmanager.BranchID{branchmanager.MasterBranchID}, []branchmanager.ConflictID{{0}})
		defer cachedBranch3.Release()
		setParent1 := func(payloadMetadata *PayloadMetadata) {
			payloadMetadata.setSolid(true)
			payloadMetadata.setBranchID(branchmanager.MasterBranchID)
		}
		setParent2 := func(payloadMetadata *PayloadMetadata) {
			payloadMetadata.setSolid(true)
			payloadMetadata.setBranchID(branchmanager.BranchID{3})
		}

		valueObject := payload.New(storeParentPayloadWithMetadataFunc(t, tangle, setParent1), storeParentPayloadWithMetadataFunc(t, tangle, setParent2), createDummyTransaction())
		metadata := NewPayloadMetadata(valueObject.ID())

		transactionBranches := []branchmanager.BranchID{{2}}
		solid, err := tangle.payloadBecameNewlySolid(valueObject, metadata, transactionBranches)
		assert.False(t, solid)
		assert.Error(t, err)
	}

	event.AssertExpectations(t)
}

func TestCreateValuePayloadFutureConeIterator(t *testing.T) {
	// check with new payload -> should be added to stack
	{
		tangle := New(mapdb.NewMapDB())
		event := newEventTangle(t, tangle)
		defer event.DetachAll()

		solidificationStack := list.New()
		processedPayloads := make(map[payload.ID]types.Empty)
		iterator := tangle.createValuePayloadFutureConeIterator(solidificationStack, processedPayloads)

		// create cached objects
		tx := createDummyTransaction()
		valueObject := payload.New(payload.GenesisID, payload.GenesisID, tx)
		cachedPayload, cachedMetadata, stored := tangle.storePayload(valueObject)
		assert.True(t, stored)
		cachedTransaction, cachedTransactionMetadata, _, transactionIsNew := tangle.storeTransactionModels(valueObject)
		assert.True(t, transactionIsNew)

		iterator(cachedPayload, cachedMetadata, cachedTransaction, cachedTransactionMetadata)
		assert.Equal(t, 1, solidificationStack.Len())
		currentSolidificationEntry := solidificationStack.Front().Value.(*valuePayloadPropagationStackEntry)
		assert.Equal(t, cachedPayload, currentSolidificationEntry.CachedPayload)
		currentSolidificationEntry.CachedPayload.Consume(func(payload *payload.Payload) {
			assert.Equal(t, valueObject.ID(), payload.ID())
		})
		currentSolidificationEntry.CachedPayloadMetadata.Consume(func(metadata *PayloadMetadata) {
			assert.Equal(t, valueObject.ID(), metadata.PayloadID())
		})
		currentSolidificationEntry.CachedTransaction.Consume(func(transaction *transaction.Transaction) {
			assert.Equal(t, tx.ID(), transaction.ID())
		})
		currentSolidificationEntry.CachedTransactionMetadata.Consume(func(metadata *TransactionMetadata) {
			assert.Equal(t, tx.ID(), metadata.ID())
		})

		event.AssertExpectations(t)
	}

	// check with already processed payload -> should not be added to stack
	{
		tangle := New(mapdb.NewMapDB())
		event := newEventTangle(t, tangle)
		defer event.DetachAll()

		solidificationStack := list.New()
		processedPayloads := make(map[payload.ID]types.Empty)
		iterator := tangle.createValuePayloadFutureConeIterator(solidificationStack, processedPayloads)

		// create cached objects
		tx := createDummyTransaction()
		valueObject := payload.New(payload.GenesisID, payload.GenesisID, tx)
		cachedPayload, cachedMetadata, stored := tangle.storePayload(valueObject)
		assert.True(t, stored)
		cachedTransaction, cachedTransactionMetadata, _, transactionIsNew := tangle.storeTransactionModels(valueObject)
		assert.True(t, transactionIsNew)

		// payload was already processed
		processedPayloads[valueObject.ID()] = types.Void

		iterator(cachedPayload, cachedMetadata, cachedTransaction, cachedTransactionMetadata)
		assert.Equal(t, 0, solidificationStack.Len())

		event.AssertExpectations(t)
	}
}

func TestForEachConsumers(t *testing.T) {
	tangle := New(mapdb.NewMapDB())

	// prepare inputs for tx
	outputs := map[address.Address][]*balance.Balance{
		address.Random(): {
			balance.New(balance.ColorIOTA, 1),
		},
		address.Random(): {
			balance.New(balance.ColorIOTA, 2),
		},
	}
	genesisTx := transaction.New(transaction.NewInputs(), transaction.NewOutputs(outputs))

	// store tx that uses outputs from genesisTx
	outputIDs := make([]transaction.OutputID, 0)
	for addr := range outputs {
		outputIDs = append(outputIDs, transaction.NewOutputID(addr, genesisTx.ID()))
	}
	tx := transaction.New(
		transaction.NewInputs(outputIDs...),
		transaction.NewOutputs(map[address.Address][]*balance.Balance{}),
	)
	valueObject := payload.New(payload.GenesisID, payload.GenesisID, tx)
	_, _, stored := tangle.storePayload(valueObject)
	assert.True(t, stored)
	_, _, _, transactionIsNew := tangle.storeTransactionModels(valueObject)
	assert.True(t, transactionIsNew)

	counter := 0
	consume := func(cachedPayload *payload.CachedPayload, cachedPayloadMetadata *CachedPayloadMetadata, cachedTransaction *transaction.CachedTransaction, cachedTransactionMetadata *CachedTransactionMetadata) {
		cachedPayload.Consume(func(payload *payload.Payload) {
			assert.Equal(t, valueObject.ID(), payload.ID())
		})
		cachedPayloadMetadata.Consume(func(metadata *PayloadMetadata) {
			assert.Equal(t, valueObject.ID(), metadata.PayloadID())
		})
		cachedTransaction.Consume(func(transaction *transaction.Transaction) {
			assert.Equal(t, tx.ID(), transaction.ID())
		})
		cachedTransactionMetadata.Consume(func(metadata *TransactionMetadata) {
			assert.Equal(t, tx.ID(), metadata.ID())
		})
		counter++
	}

	tangle.ForEachConsumers(genesisTx, consume)
	// even though we have 2 outputs it should only be triggered once because the outputs are within 1 transaction
	assert.Equal(t, 1, counter)
}

func TestForeachApprovers(t *testing.T) {
	tangle := New(mapdb.NewMapDB())

	valueObject := payload.New(payload.GenesisID, payload.GenesisID, createDummyTransaction())

	// create approver 1
	tx1 := createDummyTransaction()
	approver1 := payload.New(valueObject.ID(), payload.GenesisID, tx1)
	_, _, stored := tangle.storePayload(approver1)
	assert.True(t, stored)
	_, _, _, transactionIsNew := tangle.storeTransactionModels(approver1)
	tangle.storePayloadReferences(approver1)
	assert.True(t, transactionIsNew)

	// create approver 2
	tx2 := createDummyTransaction()
	approver2 := payload.New(valueObject.ID(), payload.GenesisID, tx2)
	_, _, stored = tangle.storePayload(approver2)
	assert.True(t, stored)
	_, _, _, transactionIsNew = tangle.storeTransactionModels(approver2)
	tangle.storePayloadReferences(approver2)
	assert.True(t, transactionIsNew)

	counter := 0
	consume := func(cachedPayload *payload.CachedPayload, cachedPayloadMetadata *CachedPayloadMetadata, cachedTransaction *transaction.CachedTransaction, cachedTransactionMetadata *CachedTransactionMetadata) {
		cachedPayload.Consume(func(p *payload.Payload) {
			assert.Contains(t, []payload.ID{approver1.ID(), approver2.ID()}, p.ID())
		})
		cachedPayloadMetadata.Consume(func(metadata *PayloadMetadata) {
			assert.Contains(t, []payload.ID{approver1.ID(), approver2.ID()}, metadata.PayloadID())
		})
		cachedTransaction.Consume(func(tx *transaction.Transaction) {
			assert.Contains(t, []transaction.ID{tx1.ID(), tx2.ID()}, tx.ID())
		})
		cachedTransactionMetadata.Consume(func(metadata *TransactionMetadata) {
			assert.Contains(t, []transaction.ID{tx1.ID(), tx2.ID()}, metadata.ID())
		})
		counter++
	}

	tangle.ForeachApprovers(valueObject.ID(), consume)
	assert.Equal(t, 2, counter)
}

func TestMissingPayloadReceived(t *testing.T) {
	tangle := New(mapdb.NewMapDB())
	event := newEventTangle(t, tangle)
	defer event.DetachAll()

	// prepare snapshot
	unspentOutputs := loadSnapshotFromOutputs(
		tangle,
		map[address.Address][]*balance.Balance{
			address.Random(): {
				balance.New(balance.ColorIOTA, 3),
			},
		},
	)

	// create transaction spending those snapshot outputs
	tx := transaction.New(
		transaction.NewInputs(unspentOutputs...),
		transaction.NewOutputs(map[address.Address][]*balance.Balance{
			address.Random(): {
				balance.New(balance.ColorIOTA, 3),
			},
		}),
	)

	// create two value objects for this transaction referencing each other
	parent := payload.New(payload.GenesisID, payload.GenesisID, tx)
	child := payload.New(parent.ID(), parent.ID(), tx)

	event.Expect("PayloadAttached", child, mock.Anything)
	event.Expect("PayloadMissing", parent.ID(), mock.Anything)
	event.Expect("TransactionReceived", tx, mock.Anything, mock.Anything)

	// submit the child first; it cannot be solidified
	tangle.AttachPayloadSync(child)

	event.Expect("PayloadAttached", parent, mock.Anything)
	event.Expect("PayloadSolid", parent, mock.Anything)
	event.Expect("MissingPayloadReceived", parent, mock.Anything)
	event.Expect("PayloadSolid", child, mock.Anything)
	event.Expect("TransactionSolid", tx, mock.Anything, mock.Anything)
	event.Expect("TransactionBooked", tx, mock.Anything, true)

	// submitting the parent makes everything solid
	tangle.AttachPayloadSync(parent)

	event.AssertExpectations(t)
}

func storeParentPayloadWithMetadataFunc(t *testing.T, tangle *Tangle, consume func(*PayloadMetadata)) payload.ID {
	parent1 := payload.New(payload.GenesisID, payload.GenesisID, createDummyTransaction())
	cachedPayload, cachedMetadata, stored := tangle.storePayload(parent1)
	defer cachedPayload.Release()

	cachedMetadata.Consume(consume)
	assert.True(t, stored)

	return parent1.ID()
}

func loadSnapshotFromOutputs(tangle *Tangle, outputs map[address.Address][]*balance.Balance) []transaction.OutputID {
	snapshot := map[transaction.ID]map[address.Address][]*balance.Balance{transaction.GenesisID: outputs}
	tangle.LoadSnapshot(snapshot)

	outputIDs := make([]transaction.OutputID, 0)
	for addr := range outputs {
		outputIDs = append(outputIDs, transaction.NewOutputID(addr, transaction.GenesisID))
	}
	return outputIDs
}

func sumOutputsByColor(outputs map[address.Address][]*balance.Balance) map[balance.Color]int64 {
	totals := make(map[balance.Color]int64)

	for _, balances := range outputs {
		for _, bal := range balances {
			totals[bal.Color] += bal.Value
		}
	}

	return totals
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
