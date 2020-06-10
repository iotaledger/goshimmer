package tangle

import (
	"container/list"
	"fmt"
	"log"
	"math"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address/signaturescheme"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/branchmanager"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tipmanager"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/wallet"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetTransactionPreferred(t *testing.T) {
	tangle := New(mapdb.NewMapDB())
	tx := createDummyTransaction()
	valueObject := payload.New(payload.GenesisID, payload.GenesisID, tx)
	tangle.storeTransactionModels(valueObject)

	modified, err := tangle.SetTransactionPreferred(tx.ID(), true)
	require.NoError(t, err)
	assert.True(t, modified)
}

func TestPropagateValuePayloadLikeUpdates(t *testing.T) {

}

//TODO: missing propagateValuePayloadConfirmedUpdates (not yet implemented)

func TestSetTransactionFinalized(t *testing.T) {
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
		tx := createDummyTransaction()
		valueObject := payload.New(payload.GenesisID, payload.GenesisID, tx)
		cachedTransaction, cachedTransactionMetadata, _, _ := tangle.storeTransactionModels(valueObject)

		transactionBooked, decisionPending, err := tangle.bookTransaction(cachedTransaction, cachedTransactionMetadata)
		assert.False(t, transactionBooked)
		assert.False(t, decisionPending)
		assert.Error(t, err)
	})

	// CASE: transaction already booked by another process
	t.Run("CASE: transaction already booked by another process", func(t *testing.T) {
		tangle := New(mapdb.NewMapDB())
		tx := createDummyTransaction()
		valueObject := payload.New(payload.GenesisID, payload.GenesisID, tx)
		cachedTransaction, cachedTransactionMetadata, _, _ := tangle.storeTransactionModels(valueObject)

		transactionMetadata := cachedTransactionMetadata.Unwrap()
		transactionMetadata.SetSolid(true)

		transactionBooked, decisionPending, err := tangle.bookTransaction(cachedTransaction, cachedTransactionMetadata)
		require.NoError(t, err)
		assert.False(t, transactionBooked)
		assert.False(t, decisionPending)
	})

	// CASE: booking first spend
	t.Run("CASE: booking first spend", func(t *testing.T) {
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

		// build first spending
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

		// assert that branchID is undefined before being booked
		assert.Equal(t, branchmanager.UndefinedBranchID, txMetadata.BranchID())

		transactionBooked, decisionPending, err := tangle.bookTransaction(cachedTransaction, cachedTransactionMetadata)
		require.NoError(t, err)
		assert.True(t, transactionBooked, "transactionBooked")
		assert.False(t, decisionPending, "decisionPending")

		// assert that branchID is the same as the MasterBranchID
		assert.Equal(t, branchmanager.MasterBranchID, txMetadata.BranchID())

		// CASE: booking double spend
		t.Run("CASE: booking double spend", func(t *testing.T) {
			// build second spending
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
			cachedTransaction, cachedTransactionMetadata, _, _ = tangle.storeTransactionModels(valueObject)
			txMetadata := cachedTransactionMetadata.Unwrap()

			// assert that branchID is undefined before being booked
			assert.Equal(t, branchmanager.UndefinedBranchID, txMetadata.BranchID())

			transactionBooked, decisionPending, err := tangle.bookTransaction(cachedTransaction, cachedTransactionMetadata)
			require.NoError(t, err)
			assert.True(t, transactionBooked, "transactionBooked")
			assert.True(t, decisionPending, "decisionPending")

			// assert that first spend and double spend have different BranchIDs
			assert.NotEqual(t, branchmanager.MasterBranchID, txMetadata.BranchID(), "BranchID")
		})
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

		txMetadata.SetFinalized(true)

		forked, finalized, err := tangle.Fork(tx.ID(), []transaction.OutputID{})
		require.NoError(t, err)
		assert.False(t, forked)
		assert.True(t, finalized)
	})

	t.Run("CASE: normal fork", func(t *testing.T) {
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
		tangle.storeTransactionModels(valueObject)

		forked, finalized, err := tangle.Fork(tx.ID(), []transaction.OutputID{})
		require.NoError(t, err)
		assert.True(t, forked, "forked")
		assert.False(t, finalized, "finalized")

		t.Run("CASE: branch existed already", func(t *testing.T) {
			forked, finalized, err = tangle.Fork(tx.ID(), []transaction.OutputID{})
			require.NoError(t, err)
			assert.False(t, forked, "forked")
			assert.False(t, finalized, "finalized")
		})
	})

}

func TestBookPayload(t *testing.T) {
	t.Run("CASE: undefined branchID", func(t *testing.T) {
		tangle := New(mapdb.NewMapDB())

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
	})

	t.Run("CASE: successfully book", func(t *testing.T) {
		tangle := New(mapdb.NewMapDB())

		valueObject := payload.New(payload.GenesisID, payload.GenesisID, createDummyTransaction())
		cachedPayload, cachedMetadata, _ := tangle.storePayload(valueObject)
		metadata := cachedMetadata.Unwrap()

		metadata.SetBranchID(branchmanager.BranchID{1})
		metadata.SetBranchID(branchmanager.BranchID{2})

		_, cachedTransactionMetadata, _, _ := tangle.storeTransactionModels(valueObject)
		txMetadata := cachedTransactionMetadata.Unwrap()
		txMetadata.SetBranchID(branchmanager.BranchID{1})

		payloadBooked, err := tangle.bookPayload(cachedPayload.Retain(), cachedMetadata.Retain(), cachedTransactionMetadata.Retain())
		defer func() {
			cachedPayload.Release()
			cachedMetadata.Release()
			cachedTransactionMetadata.Release()
		}()

		require.NoError(t, err)
		assert.True(t, payloadBooked, "payloadBooked")
	})

	t.Run("CASE: not booked", func(t *testing.T) {
		tangle := New(mapdb.NewMapDB())

		valueObject := payload.New(payload.GenesisID, payload.GenesisID, createDummyTransaction())
		cachedPayload, cachedMetadata, _ := tangle.storePayload(valueObject)
		metadata := cachedMetadata.Unwrap()

		metadata.SetBranchID(branchmanager.BranchID{1})
		metadata.SetBranchID(branchmanager.BranchID{1})

		_, cachedTransactionMetadata, _, _ := tangle.storeTransactionModels(valueObject)
		txMetadata := cachedTransactionMetadata.Unwrap()
		txMetadata.SetBranchID(branchmanager.BranchID{1})

		payloadBooked, err := tangle.bookPayload(cachedPayload.Retain(), cachedMetadata.Retain(), cachedTransactionMetadata.Retain())
		defer func() {
			cachedPayload.Release()
			cachedMetadata.Release()
			cachedTransactionMetadata.Release()
		}()

		require.NoError(t, err)
		assert.False(t, payloadBooked, "payloadBooked")
	})

}

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

func TestLoadSnapshot(t *testing.T) {
	tangle := New(mapdb.NewMapDB())

	snapshot := map[transaction.ID]map[address.Address][]*balance.Balance{
		transaction.GenesisID: {
			address.Random(): []*balance.Balance{
				balance.New(balance.ColorIOTA, 337),
			},

			address.Random(): []*balance.Balance{
				balance.New(balance.ColorIOTA, 1000),
				balance.New(balance.ColorIOTA, 1000),
			},
		},
	}
	tangle.LoadSnapshot(snapshot)

	// check whether outputs can be retrieved from tangle
	for addr, balances := range snapshot[transaction.GenesisID] {
		cachedOutput := tangle.TransactionOutput(transaction.NewOutputID(addr, transaction.GenesisID))
		cachedOutput.Consume(func(output *Output) {
			assert.Equal(t, addr, output.Address())
			assert.ElementsMatch(t, balances, output.Balances())
			assert.True(t, output.Solid())
			assert.Equal(t, branchmanager.MasterBranchID, output.BranchID())
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
		assert.True(t, cmp.Equal(sumOutputsByColor(outputs), consumedBalances))
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
		assert.True(t, cmp.Equal(sumOutputsByColor(outputs), consumedBalances))
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
		assert.True(t, cmp.Equal(sumOutputsByColor(outputs), consumedBalances))
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
		txMetadata.SetSolid(true)
		txMetadata.SetBranchID(branchmanager.MasterBranchID)

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
			metadata.SetBranchID(expectedBranchID)
		})

		branchID := tangle.payloadBranchID(valueObject.ID())
		assert.Equal(t, expectedBranchID, branchID)
	}

	// test missing value object
	{
		valueObject := payload.New(payload.GenesisID, payload.GenesisID, createDummyTransaction())
		missing := 0
		tangle.Events.PayloadMissing.Attach(events.NewClosure(func(payloadID payload.ID) {
			missing++
		}))

		branchID := tangle.payloadBranchID(valueObject.ID())
		assert.Equal(t, branchmanager.UndefinedBranchID, branchID)
		assert.Equal(t, 1, missing)
	}
}

func TestCheckPayloadSolidity(t *testing.T) {
	tangle := New(mapdb.NewMapDB())

	// check with already solid payload
	{
		valueObject := payload.New(payload.GenesisID, payload.GenesisID, createDummyTransaction())
		metadata := NewPayloadMetadata(valueObject.ID())
		metadata.setSolid(true)
		metadata.SetBranchID(branchmanager.MasterBranchID)

		transactionBranches := []branchmanager.BranchID{branchmanager.MasterBranchID}
		solid, err := tangle.checkPayloadSolidity(valueObject, metadata, transactionBranches)
		assert.True(t, solid)
		assert.NoError(t, err)
	}

	// check with parents=genesis
	{
		valueObject := payload.New(payload.GenesisID, payload.GenesisID, createDummyTransaction())
		metadata := NewPayloadMetadata(valueObject.ID())

		transactionBranches := []branchmanager.BranchID{branchmanager.MasterBranchID}
		solid, err := tangle.checkPayloadSolidity(valueObject, metadata, transactionBranches)
		assert.True(t, solid)
		assert.NoError(t, err)
	}

	// check with solid parents and branch set
	{
		setParent := func(payloadMetadata *PayloadMetadata) {
			payloadMetadata.setSolid(true)
			payloadMetadata.SetBranchID(branchmanager.MasterBranchID)
		}

		valueObject := payload.New(storeParentPayloadWithMetadataFunc(t, tangle, setParent), storeParentPayloadWithMetadataFunc(t, tangle, setParent), createDummyTransaction())
		metadata := NewPayloadMetadata(valueObject.ID())

		transactionBranches := []branchmanager.BranchID{branchmanager.MasterBranchID}
		solid, err := tangle.checkPayloadSolidity(valueObject, metadata, transactionBranches)
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
		solid, err := tangle.checkPayloadSolidity(valueObject, metadata, transactionBranches)
		assert.False(t, solid)
		assert.NoError(t, err)
	}

	// check with non-solid parents but branch set -> should not be solid
	{
		setParent := func(payloadMetadata *PayloadMetadata) {
			payloadMetadata.SetBranchID(branchmanager.MasterBranchID)
		}

		valueObject := payload.New(storeParentPayloadWithMetadataFunc(t, tangle, setParent), storeParentPayloadWithMetadataFunc(t, tangle, setParent), createDummyTransaction())
		metadata := NewPayloadMetadata(valueObject.ID())

		transactionBranches := []branchmanager.BranchID{branchmanager.MasterBranchID}
		solid, err := tangle.checkPayloadSolidity(valueObject, metadata, transactionBranches)
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
			payloadMetadata.SetBranchID(branchmanager.BranchID{2})
		}
		setParent2 := func(payloadMetadata *PayloadMetadata) {
			payloadMetadata.setSolid(true)
			payloadMetadata.SetBranchID(branchmanager.BranchID{3})
		}

		valueObject := payload.New(storeParentPayloadWithMetadataFunc(t, tangle, setParent1), storeParentPayloadWithMetadataFunc(t, tangle, setParent2), createDummyTransaction())
		metadata := NewPayloadMetadata(valueObject.ID())

		transactionBranches := []branchmanager.BranchID{branchmanager.MasterBranchID}
		solid, err := tangle.checkPayloadSolidity(valueObject, metadata, transactionBranches)
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
			payloadMetadata.SetBranchID(branchmanager.MasterBranchID)
		}
		setParent2 := func(payloadMetadata *PayloadMetadata) {
			payloadMetadata.setSolid(true)
			payloadMetadata.SetBranchID(branchmanager.BranchID{3})
		}

		valueObject := payload.New(storeParentPayloadWithMetadataFunc(t, tangle, setParent1), storeParentPayloadWithMetadataFunc(t, tangle, setParent2), createDummyTransaction())
		metadata := NewPayloadMetadata(valueObject.ID())

		transactionBranches := []branchmanager.BranchID{{2}}
		solid, err := tangle.checkPayloadSolidity(valueObject, metadata, transactionBranches)
		assert.False(t, solid)
		assert.Error(t, err)
	}
}

func TestCreateValuePayloadFutureConeIterator(t *testing.T) {
	// check with new payload -> should be added to stack
	{
		tangle := New(mapdb.NewMapDB())
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
	}

	// check with already processed payload -> should not be added to stack
	{
		tangle := New(mapdb.NewMapDB())
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
	}
}

func TestConcurrency(t *testing.T) {
	// img/concurrency.png
	// Builds a simple UTXO-DAG where each transaction spends exactly 1 output from genesis.
	// Tips are concurrently selected (via TipManager) resulting in a moderately wide tangle depending on `threads`.
	tangle := New(mapdb.NewMapDB())
	defer tangle.Shutdown()

	tipManager := tipmanager.New()

	count := 1000
	threads := 10
	countTotal := threads * count

	// initialize tangle with genesis block
	outputs := make(map[address.Address][]*balance.Balance)
	for i := 0; i < countTotal; i++ {
		outputs[address.Random()] = []*balance.Balance{
			balance.New(balance.ColorIOTA, 1),
		}
	}
	inputIDs := loadSnapshotFromOutputs(tangle, outputs)

	transactions := make([]*transaction.Transaction, countTotal)
	valueObjects := make([]*payload.Payload, countTotal)

	// start threads, each working on its chunk of transaction and valueObjects
	var wg sync.WaitGroup
	for thread := 0; thread < threads; thread++ {
		wg.Add(1)
		go func(threadNo int) {
			defer wg.Done()

			start := threadNo * count
			end := start + count

			for i := start; i < end; i++ {
				// issue transaction moving funds from genesis
				tx := transaction.New(
					transaction.NewInputs(inputIDs[i]),
					transaction.NewOutputs(
						map[address.Address][]*balance.Balance{
							address.Random(): {
								balance.New(balance.ColorIOTA, 1),
							},
						}),
				)
				// use random value objects as tips (possibly created in other threads)
				parent1, parent2 := tipManager.Tips()
				valueObject := payload.New(parent1, parent2, tx)

				tangle.AttachPayloadSync(valueObject)

				tipManager.AddTip(valueObject)
				transactions[i] = tx
				valueObjects[i] = valueObject
			}
		}(thread)
	}

	wg.Wait()

	// verify correctness
	for i := 0; i < countTotal; i++ {
		// check if transaction metadata is found in database
		assert.True(t, tangle.TransactionMetadata(transactions[i].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Truef(t, transactionMetadata.Solid(), "the transaction is not solid")
			assert.Equalf(t, branchmanager.MasterBranchID, transactionMetadata.BranchID(), "the transaction was booked into the wrong branch")
		}))

		// check if payload metadata is found in database
		assert.True(t, tangle.PayloadMetadata(valueObjects[i].ID()).Consume(func(payloadMetadata *PayloadMetadata) {
			assert.Truef(t, payloadMetadata.IsSolid(), "the payload is not solid")
			assert.Equalf(t, branchmanager.MasterBranchID, payloadMetadata.BranchID(), "the payload was booked into the wrong branch")
		}))

		// check if outputs are found in database
		transactions[i].Outputs().ForEach(func(address address.Address, balances []*balance.Balance) bool {
			cachedOutput := tangle.TransactionOutput(transaction.NewOutputID(address, transactions[i].ID()))
			assert.True(t, cachedOutput.Consume(func(output *Output) {
				assert.Equalf(t, 0, output.ConsumerCount(), "the output should not be spent")
				assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1)}, output.Balances())
				assert.Equalf(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
				assert.Truef(t, output.Solid(), "the output is not solid")
			}))
			return true
		})

		// check that all inputs are consumed exactly once
		cachedInput := tangle.TransactionOutput(inputIDs[i])
		assert.True(t, cachedInput.Consume(func(output *Output) {
			assert.Equalf(t, 1, output.ConsumerCount(), "the output should be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1)}, output.Balances())
			assert.Equalf(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
			assert.Truef(t, output.Solid(), "the output is not solid")
		}))
	}
}

func TestReverseValueObjectSolidification(t *testing.T) {
	// img/reverse-valueobject-solidification.png
	// Builds a simple UTXO-DAG where each transaction spends exactly 1 output from genesis.
	// All value objects reference the previous value object, effectively creating a chain.
	// The test attaches the prepared value objects concurrently in reverse order.
	tangle := New(mapdb.NewMapDB())
	defer tangle.Shutdown()

	tipManager := tipmanager.New()

	count := 1000
	threads := 5
	countTotal := threads * count

	// initialize tangle with genesis block
	outputs := make(map[address.Address][]*balance.Balance)
	for i := 0; i < countTotal; i++ {
		outputs[address.Random()] = []*balance.Balance{
			balance.New(balance.ColorIOTA, 1),
		}
	}
	inputIDs := loadSnapshotFromOutputs(tangle, outputs)

	transactions := make([]*transaction.Transaction, countTotal)
	valueObjects := make([]*payload.Payload, countTotal)

	// prepare value objects
	for i := 0; i < countTotal; i++ {
		tx := transaction.New(
			// issue transaction moving funds from genesis
			transaction.NewInputs(inputIDs[i]),
			transaction.NewOutputs(
				map[address.Address][]*balance.Balance{
					address.Random(): {
						balance.New(balance.ColorIOTA, 1),
					},
				}),
		)
		parent1, parent2 := tipManager.Tips()
		valueObject := payload.New(parent1, parent2, tx)

		tipManager.AddTip(valueObject)
		transactions[i] = tx
		valueObjects[i] = valueObject
	}

	// attach value objects in reverse order
	var wg sync.WaitGroup
	for thread := 0; thread < threads; thread++ {
		wg.Add(1)
		go func(threadNo int) {
			defer wg.Done()

			for i := countTotal - 1 - threadNo; i >= 0; i -= threads {
				valueObject := valueObjects[i]
				tangle.AttachPayloadSync(valueObject)
			}
		}(thread)
	}
	wg.Wait()

	// verify correctness
	for i := 0; i < countTotal; i++ {
		// check if transaction metadata is found in database
		assert.True(t, tangle.TransactionMetadata(transactions[i].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Truef(t, transactionMetadata.Solid(), "the transaction is not solid")
			assert.Equalf(t, branchmanager.MasterBranchID, transactionMetadata.BranchID(), "the transaction was booked into the wrong branch")
		}))

		// check if payload metadata is found in database
		assert.True(t, tangle.PayloadMetadata(valueObjects[i].ID()).Consume(func(payloadMetadata *PayloadMetadata) {
			assert.Truef(t, payloadMetadata.IsSolid(), "the payload is not solid")
			assert.Equalf(t, branchmanager.MasterBranchID, payloadMetadata.BranchID(), "the payload was booked into the wrong branch")
		}))

		// check if outputs are found in database
		transactions[i].Outputs().ForEach(func(address address.Address, balances []*balance.Balance) bool {
			cachedOutput := tangle.TransactionOutput(transaction.NewOutputID(address, transactions[i].ID()))
			assert.True(t, cachedOutput.Consume(func(output *Output) {
				assert.Equalf(t, 0, output.ConsumerCount(), "the output should not be spent")
				assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1)}, output.Balances())
				assert.Equalf(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
				assert.Truef(t, output.Solid(), "the output is not solid")
			}))
			return true
		})

		// check that all inputs are consumed exactly once
		cachedInput := tangle.TransactionOutput(inputIDs[i])
		assert.True(t, cachedInput.Consume(func(output *Output) {
			assert.Equalf(t, 1, output.ConsumerCount(), "the output should be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1)}, output.Balances())
			assert.Equalf(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
			assert.Truef(t, output.Solid(), "the output is not solid")
		}))
	}
}

func TestReverseTransactionSolidification(t *testing.T) {
	// img/reverse-transaction-solidification.png
	// Builds a UTXO-DAG with `txChains` spending outputs from the corresponding chain.
	// All value objects reference the previous value object, effectively creating a chain.
	// The test attaches the prepared value objects concurrently in reverse order.
	tangle := New(mapdb.NewMapDB())
	defer tangle.Shutdown()

	tipManager := tipmanager.New()

	txChains := 5
	count := 100
	threads := 10
	countTotal := txChains * threads * count

	// initialize tangle with genesis block
	outputs := make(map[address.Address][]*balance.Balance)
	for i := 0; i < txChains; i++ {
		outputs[address.Random()] = []*balance.Balance{
			balance.New(balance.ColorIOTA, 1),
		}
	}
	inputIDs := loadSnapshotFromOutputs(tangle, outputs)

	transactions := make([]*transaction.Transaction, countTotal)
	valueObjects := make([]*payload.Payload, countTotal)

	// create chains of transactions
	for i := 0; i < count*threads; i++ {
		for j := 0; j < txChains; j++ {
			var tx *transaction.Transaction

			// transferring from genesis
			if i == 0 {
				tx = transaction.New(
					transaction.NewInputs(inputIDs[j]),
					transaction.NewOutputs(
						map[address.Address][]*balance.Balance{
							address.Random(): {
								balance.New(balance.ColorIOTA, 1),
							},
						}),
				)
			} else {
				// create chains in UTXO dag
				tx = transaction.New(
					getTxOutputsAsInputs(transactions[i*txChains-txChains+j]),
					transaction.NewOutputs(
						map[address.Address][]*balance.Balance{
							address.Random(): {
								balance.New(balance.ColorIOTA, 1),
							},
						}),
				)
			}

			transactions[i*txChains+j] = tx
		}
	}

	// prepare value objects (simple chain)
	for i := 0; i < countTotal; i++ {
		parent1, parent2 := tipManager.Tips()
		valueObject := payload.New(parent1, parent2, transactions[i])

		tipManager.AddTip(valueObject)
		valueObjects[i] = valueObject
	}

	// attach value objects in reverse order
	var wg sync.WaitGroup
	for thread := 0; thread < threads; thread++ {
		wg.Add(1)
		go func(threadNo int) {
			defer wg.Done()

			for i := countTotal - 1 - threadNo; i >= 0; i -= threads {
				valueObject := valueObjects[i]
				tangle.AttachPayloadSync(valueObject)
			}
		}(thread)
	}
	wg.Wait()

	// verify correctness
	for i := 0; i < countTotal; i++ {
		// check if transaction metadata is found in database
		assert.True(t, tangle.TransactionMetadata(transactions[i].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.Truef(t, transactionMetadata.Solid(), "the transaction %s is not solid", transactions[i].ID().String())
			assert.Equalf(t, branchmanager.MasterBranchID, transactionMetadata.BranchID(), "the transaction was booked into the wrong branch")
		}))

		// check if payload metadata is found in database
		assert.True(t, tangle.PayloadMetadata(valueObjects[i].ID()).Consume(func(payloadMetadata *PayloadMetadata) {
			assert.Truef(t, payloadMetadata.IsSolid(), "the payload is not solid")
			assert.Equalf(t, branchmanager.MasterBranchID, payloadMetadata.BranchID(), "the payload was booked into the wrong branch")
		}))

		// check if outputs are found in database
		transactions[i].Outputs().ForEach(func(address address.Address, balances []*balance.Balance) bool {
			cachedOutput := tangle.TransactionOutput(transaction.NewOutputID(address, transactions[i].ID()))
			assert.True(t, cachedOutput.Consume(func(output *Output) {
				// only the last outputs in chain should not be spent
				if i+txChains >= countTotal {
					assert.Equalf(t, 0, output.ConsumerCount(), "the output should not be spent")
				} else {
					assert.Equalf(t, 1, output.ConsumerCount(), "the output should be spent")
				}
				assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1)}, output.Balances())
				assert.Equalf(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
				assert.Truef(t, output.Solid(), "the output is not solid")
			}))
			return true
		})
	}
}

func getTxOutputsAsInputs(tx *transaction.Transaction) *transaction.Inputs {
	outputIDs := make([]transaction.OutputID, 0)
	tx.Outputs().ForEach(func(address address.Address, balances []*balance.Balance) bool {
		outputIDs = append(outputIDs, transaction.NewOutputID(address, tx.ID()))
		return true
	})

	return transaction.NewInputs(outputIDs...)
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

const (
	GENESIS uint64 = iota
	A
	B
	C
	D
	E
	F
	G
	H
	I
	J
	Y
)

func preparePropagationScenario1() (*Tangle, map[string]*transaction.Transaction, map[string]*payload.Payload, *wallet.Seed) {
	// create tangle
	tangle := New(mapdb.NewMapDB())

	// create seed for testing
	seed := wallet.NewSeed()

	// initialize tangle with genesis block (+GENESIS)
	tangle.LoadSnapshot(map[transaction.ID]map[address.Address][]*balance.Balance{
		transaction.GenesisID: {
			seed.Address(GENESIS): {
				balance.New(balance.ColorIOTA, 3333),
			},
		},
	})

	// create dictionaries so we can address the created entities by their aliases from the picture
	transactions := make(map[string]*transaction.Transaction)
	valueObjects := make(map[string]*payload.Payload)

	// [-GENESIS, A+, B+, C+]
	{
		// create transaction + payload
		transactions["[-GENESIS, A+, B+, C+]"] = transaction.New(
			transaction.NewInputs(
				transaction.NewOutputID(seed.Address(GENESIS), transaction.GenesisID),
			),

			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				seed.Address(A): {
					balance.New(balance.ColorIOTA, 1111),
				},
				seed.Address(B): {
					balance.New(balance.ColorIOTA, 1111),
				},
				seed.Address(C): {
					balance.New(balance.ColorIOTA, 1111),
				},
			}),
		)
		valueObjects["[-GENESIS, A+, B+, C+]"] = payload.New(payload.GenesisID, payload.GenesisID, transactions["[-GENESIS, A+, B+, C+]"])
		// attach payload
		tangle.AttachPayloadSync(valueObjects["[-GENESIS, A+, B+, C+]"])
	}
	// [-A, D+]
	{
		// create transaction + payload
		transactions["[-A, D+]"] = transaction.New(
			transaction.NewInputs(
				transaction.NewOutputID(seed.Address(A), transactions["[-GENESIS, A+, B+, C+]"].ID()),
			),

			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				seed.Address(D): {
					balance.New(balance.ColorIOTA, 1111),
				},
			}),
		)
		valueObjects["[-A, D+]"] = payload.New(payload.GenesisID, valueObjects["[-GENESIS, A+, B+, C+]"].ID(), transactions["[-A, D+]"])
		// attach payload
		tangle.AttachPayloadSync(valueObjects["[-A, D+]"])
	}
	// [-B, -C, E+]
	{
		// create transaction + payload
		transactions["[-B, -C, E+]"] = transaction.New(
			transaction.NewInputs(
				transaction.NewOutputID(seed.Address(B), transactions["[-GENESIS, A+, B+, C+]"].ID()),
				transaction.NewOutputID(seed.Address(C), transactions["[-GENESIS, A+, B+, C+]"].ID()),
			),

			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				seed.Address(E): {
					balance.New(balance.ColorIOTA, 2222),
				},
			}),
		)
		valueObjects["[-B, -C, E+]"] = payload.New(payload.GenesisID, valueObjects["[-GENESIS, A+, B+, C+]"].ID(), transactions["[-B, -C, E+]"])
		// attach payload
		tangle.AttachPayloadSync(valueObjects["[-B, -C, E+]"])
	}
	// [-B, -C, E+] (Reattachment)
	{
		// create payload
		valueObjects["[-B, -C, E+] (Reattachment)"] = payload.New(valueObjects["[-B, -C, E+]"].ID(), valueObjects["[-GENESIS, A+, B+, C+]"].ID(), transactions["[-B, -C, E+]"])
		// attach payload
		tangle.AttachPayloadSync(valueObjects["[-B, -C, E+] (Reattachment)"])
	}
	// [-A, F+]
	{
		// create transaction + payload
		transactions["[-A, F+]"] = transaction.New(
			transaction.NewInputs(
				transaction.NewOutputID(seed.Address(A), transactions["[-GENESIS, A+, B+, C+]"].ID()),
			),

			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				seed.Address(F): {
					balance.New(balance.ColorIOTA, 1111),
				},
			}),
		)
		valueObjects["[-A, F+]"] = payload.New(payload.GenesisID, valueObjects["[-GENESIS, A+, B+, C+]"].ID(), transactions["[-A, F+]"])
		// attach payload
		tangle.AttachPayloadSync(valueObjects["[-A, F+]"])
	}
	// [-E, -F, G+]
	{
		// create transaction + payload
		transactions["[-E, -F, G+]"] = transaction.New(
			transaction.NewInputs(
				transaction.NewOutputID(seed.Address(E), transactions["[-B, -C, E+]"].ID()),
				transaction.NewOutputID(seed.Address(F), transactions["[-A, F+]"].ID()),
			),

			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				seed.Address(G): {
					balance.New(balance.ColorIOTA, 3333),
				},
			}),
		)
		valueObjects["[-E, -F, G+]"] = payload.New(valueObjects["[-B, -C, E+]"].ID(), valueObjects["[-A, F+]"].ID(), transactions["[-E, -F, G+]"])
		// attach payload
		tangle.AttachPayloadSync(valueObjects["[-E, -F, G+]"])
	}

	return tangle, transactions, valueObjects, seed
}

func preparePropagationScenario2() (*Tangle, map[string]*transaction.Transaction, map[string]*payload.Payload, *wallet.Seed) {
	tangle, transactions, valueObjects, seed := preparePropagationScenario1()

	// [-C, H+]
	{
		// create transaction + payload
		transactions["[-C, H+]"] = transaction.New(
			transaction.NewInputs(
				transaction.NewOutputID(seed.Address(C), transactions["[-GENESIS, A+, B+, C+]"].ID()),
			),

			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				seed.Address(H): {
					balance.New(balance.ColorIOTA, 1111),
				},
			}),
		)
		valueObjects["[-C, H+]"] = payload.New(valueObjects["[-GENESIS, A+, B+, C+]"].ID(), valueObjects["[-A, D+]"].ID(), transactions["[-C, H+]"])

		// attach payload
		tangle.AttachPayloadSync(valueObjects["[-C, H+]"])
	}

	// [-H, -D, I+]
	{
		// create transaction + payload
		transactions["[-H, -D, I+]"] = transaction.New(
			transaction.NewInputs(
				transaction.NewOutputID(seed.Address(H), transactions["[-C, H+]"].ID()),
				transaction.NewOutputID(seed.Address(D), transactions["[-A, D+]"].ID()),
			),

			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				seed.Address(I): {
					balance.New(balance.ColorIOTA, 2222),
				},
			}),
		)
		valueObjects["[-H, -D, I+]"] = payload.New(valueObjects["[-C, H+]"].ID(), valueObjects["[-A, D+]"].ID(), transactions["[-H, -D, I+]"])

		// attach payload
		tangle.AttachPayloadSync(valueObjects["[-H, -D, I+]"])
	}

	// [-B, J+]
	{
		// create transaction + payload
		transactions["[-B, J+]"] = transaction.New(
			transaction.NewInputs(
				transaction.NewOutputID(seed.Address(B), transactions["[-GENESIS, A+, B+, C+]"].ID()),
			),

			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				seed.Address(J): {
					balance.New(balance.ColorIOTA, 1111),
				},
			}),
		)
		valueObjects["[-B, J+]"] = payload.New(valueObjects["[-C, H+]"].ID(), valueObjects["[-A, D+]"].ID(), transactions["[-B, J+]"])
		// attach payload
		tangle.AttachPayloadSync(valueObjects["[-B, J+]"])
	}

	return tangle, transactions, valueObjects, seed
}

func TestPropagationScenario1(t *testing.T) {
	// test past cone monotonicity - all value objects MUST be confirmed
	{
		tangle, transactions, valueObjects, _ := preparePropagationScenario1()

		setTransactionPreferredWithCheck(t, tangle, transactions["[-GENESIS, A+, B+, C+]"], true)
		verifyInclusionState(t, tangle, valueObjects["[-GENESIS, A+, B+, C+]"], true, false, true, false, false)

		// should not be confirmed because [-GENESIS, A+, B+, C+] is not confirmed
		setTransactionPreferredWithCheck(t, tangle, transactions["[-B, -C, E+]"], true)
		setTransactionFinalizedWithCheck(t, tangle, transactions["[-B, -C, E+]"])
		verifyInclusionState(t, tangle, valueObjects["[-B, -C, E+]"], true, true, true, false, false)

		// now finalize [-GENESIS, A+, B+, C+]
		setTransactionFinalizedWithCheck(t, tangle, transactions["[-GENESIS, A+, B+, C+]"])
		verifyInclusionState(t, tangle, valueObjects["[-GENESIS, A+, B+, C+]"], true, true, true, true, false)

		// and [-B, -C, E+] should be confirmed now too
		verifyInclusionState(t, tangle, valueObjects["[-B, -C, E+]"], true, true, true, true, false)
		// as well as the reattachment
		verifyInclusionState(t, tangle, valueObjects["[-B, -C, E+] (Reattachment)"], true, true, true, true, false)
	}

	// test future cone monotonicity simple - everything MUST be rejected and finalized if spending funds from rejected tx
	{
		tangle, transactions, valueObjects, _ := preparePropagationScenario1()

		setTransactionFinalizedWithCheck(t, tangle, transactions["[-GENESIS, A+, B+, C+]"])
		verifyInclusionState(t, tangle, valueObjects["[-GENESIS, A+, B+, C+]"], false, true, false, false, true)

		// check future cone to be rejected
		verifyInclusionState(t, tangle, valueObjects["[-B, -C, E+]"], false, true, false, false, true)
		verifyInclusionState(t, tangle, valueObjects["[-B, -C, E+] (Reattachment)"], false, true, false, false, true)
		verifyInclusionState(t, tangle, valueObjects["[-A, D+]"], false, true, false, false, true)
		verifyInclusionState(t, tangle, valueObjects["[-A, F+]"], false, true, false, false, true)
		verifyInclusionState(t, tangle, valueObjects["[-E, -F, G+]"], false, true, false, false, true)
	}

	// test future cone monotonicity more complex - everything MUST be rejected and finalized if spending funds from rejected tx
	{
		transactionsSlice := []string{
			"[-GENESIS, A+, B+, C+]",
			"[-A, D+]",
			"[-B, -C, E+]",
			"[-B, -C, E+] (Reattachment)",
			"[-A, F+]",
			"[-E, -F, G+]",
		}
		tangle, transactions, valueObjects, _ := preparePropagationScenario1()

		for _, name := range transactionsSlice {
			fmt.Println(name, valueObjects[name].Transaction().ID())
		}

		setTransactionPreferredWithCheck(t, tangle, transactions["[-GENESIS, A+, B+, C+]"], true)
		setTransactionFinalizedWithCheck(t, tangle, transactions["[-GENESIS, A+, B+, C+]"])
		verifyInclusionState(t, tangle, valueObjects["[-GENESIS, A+, B+, C+]"], true, true, true, true, false)

		// finalize & reject
		setTransactionFinalizedWithCheck(t, tangle, transactions["[-B, -C, E+]"])
		verifyInclusionState(t, tangle, valueObjects["[-B, -C, E+]"], false, true, false, false, true)

		// check future cone to be rejected
		verifyInclusionState(t, tangle, valueObjects["[-B, -C, E+] (Reattachment)"], false, true, false, false, true)

		// [-A, F+] should be rejected but the tx not finalized since it spends funds from [-GENESIS, A+, B+, C+] which is confirmed
		verifyInclusionState(t, tangle, valueObjects["[-A, F+]"], false, false, false, false, true)
		verifyBranchState(t, tangle, valueObjects["[-A, F+]"], false, false, false, true)
		//TODO:

		// [-E, -F, G+] should be finalized and rejected since it spends funds from [-B, -C, E+]
		verifyInclusionState(t, tangle, valueObjects["[-E, -F, G+]"], false, true, false, false, true)

		// [-A, D+] should be unchanged
		verifyInclusionState(t, tangle, valueObjects["[-A, D+]"], false, false, false, false, false)
		verifyBranchState(t, tangle, valueObjects["[-A, D+]"], false, false, false, false)
	}

	// simulate vote on [-A, F+] -> Branch A becomes rejected, Branch B confirmed
	{
		tangle, transactions, valueObjects, _ := preparePropagationScenario1()

		setTransactionPreferredWithCheck(t, tangle, transactions["[-GENESIS, A+, B+, C+]"], true)
		setTransactionFinalizedWithCheck(t, tangle, transactions["[-GENESIS, A+, B+, C+]"])
		verifyInclusionState(t, tangle, valueObjects["[-GENESIS, A+, B+, C+]"], true, true, true, true, false)

		// check future cone
		verifyInclusionState(t, tangle, valueObjects["[-B, -C, E+]"], false, false, false, false, false)
		verifyInclusionState(t, tangle, valueObjects["[-B, -C, E+] (Reattachment)"], false, false, false, false, false)
		verifyInclusionState(t, tangle, valueObjects["[-A, D+]"], false, false, false, false, false)
		verifyInclusionState(t, tangle, valueObjects["[-A, F+]"], false, false, false, false, false)
		verifyInclusionState(t, tangle, valueObjects["[-E, -F, G+]"], false, false, false, false, false)

		// confirm [-B, -C, E+]
		setTransactionPreferredWithCheck(t, tangle, transactions["[-B, -C, E+]"], true)
		setTransactionFinalizedWithCheck(t, tangle, transactions["[-B, -C, E+]"])
		verifyInclusionState(t, tangle, valueObjects["[-B, -C, E+]"], true, true, true, true, false)

		// prefer [-A, D+]
		setTransactionPreferredWithCheck(t, tangle, transactions["[-A, D+]"], true)
		verifyInclusionState(t, tangle, valueObjects["[-A, D+]"], true, false, true, false, false)
		verifyBranchState(t, tangle, valueObjects["[-A, D+]"], false, true, false, false)

		// simulate vote result to like [-A, F+] -> [-A, F+] becomes confirmed and [-A, D+] rejected
		setTransactionPreferredWithCheck(t, tangle, transactions["[-A, F+]"], true)
		setTransactionFinalizedWithCheck(t, tangle, transactions["[-A, F+]"])
		verifyInclusionState(t, tangle, valueObjects["[-A, F+]"], true, true, true, true, false)
		verifyBranchState(t, tangle, valueObjects["[-A, F+]"], true, true, true, false)

		verifyInclusionState(t, tangle, valueObjects["[-E, -F, G+]"], false, false, false, false, false)
		setTransactionPreferredWithCheck(t, tangle, transactions["[-E, -F, G+]"], true)
		setTransactionFinalizedWithCheck(t, tangle, transactions["[-E, -F, G+]"])
		verifyInclusionState(t, tangle, valueObjects["[-E, -F, G+]"], true, true, true, true, false)

		// [-A, D+] should be rejected
		verifyInclusionState(t, tangle, valueObjects["[-A, D+]"], false, true, false, false, true)
		verifyBranchState(t, tangle, valueObjects["[-A, D+]"], true, false, false, true)
	}

	// simulate vote on [-A, D+] -> Branch B becomes rejected, Branch A confirmed
	{
		tangle, transactions, valueObjects, _ := preparePropagationScenario1()

		// confirm [-GENESIS, A+, B+, C+]
		setTransactionPreferredWithCheck(t, tangle, transactions["[-GENESIS, A+, B+, C+]"], true)
		setTransactionFinalizedWithCheck(t, tangle, transactions["[-GENESIS, A+, B+, C+]"])
		verifyInclusionState(t, tangle, valueObjects["[-GENESIS, A+, B+, C+]"], true, true, true, true, false)

		// confirm [-B, -C, E+]
		setTransactionPreferredWithCheck(t, tangle, transactions["[-B, -C, E+]"], true)
		setTransactionFinalizedWithCheck(t, tangle, transactions["[-B, -C, E+]"])
		verifyInclusionState(t, tangle, valueObjects["[-B, -C, E+]"], true, true, true, true, false)

		// prefer [-A, F+] and thus Branch B
		setTransactionPreferredWithCheck(t, tangle, transactions["[-A, F+]"], true)
		verifyInclusionState(t, tangle, valueObjects["[-A, F+]"], true, false, true, false, false)
		verifyBranchState(t, tangle, valueObjects["[-A, F+]"], false, true, false, false)
		// prefer [-E, -F, G+]
		setTransactionPreferredWithCheck(t, tangle, transactions["[-E, -F, G+]"], true)
		verifyInclusionState(t, tangle, valueObjects["[-E, -F, G+]"], true, false, true, false, false)

		// simulate vote result to like [-A, D+] -> [-A, D+] becomes confirmed and [-A, F+], [-E, -F, G+] rejected
		setTransactionPreferredWithCheck(t, tangle, transactions["[-A, D+]"], true)
		setTransactionFinalizedWithCheck(t, tangle, transactions["[-A, D+]"])
		verifyInclusionState(t, tangle, valueObjects["[-A, D+]"], true, true, true, true, false)
		verifyBranchState(t, tangle, valueObjects["[-A, D+]"], true, true, true, false)

		// [-A, F+], [-E, -F, G+] should be finalized and rejected
		verifyInclusionState(t, tangle, valueObjects["[-A, F+]"], false, true, false, false, true)
		verifyBranchState(t, tangle, valueObjects["[-A, F+]"], true, false, false, true)
		verifyInclusionState(t, tangle, valueObjects["[-E, -F, G+]"], false, true, false, false, true)
	}
}

func TestPropagationScenario2(t *testing.T) {
	tangle, transactions, valueObjects, _ := preparePropagationScenario2()

	// confirm [-GENESIS, A+, B+, C+]
	setTransactionPreferredWithCheck(t, tangle, transactions["[-GENESIS, A+, B+, C+]"], true)
	setTransactionFinalizedWithCheck(t, tangle, transactions["[-GENESIS, A+, B+, C+]"])
	verifyInclusionState(t, tangle, valueObjects["[-GENESIS, A+, B+, C+]"], true, true, true, true, false)

	// prefer [-B, -C, E+] and thus Branch D
	setTransactionPreferredWithCheck(t, tangle, transactions["[-B, -C, E+]"], true)
	verifyInclusionState(t, tangle, valueObjects["[-B, -C, E+]"], true, false, true, false, false)
	verifyBranchState(t, tangle, valueObjects["[-B, -C, E+]"], false, true, false, false)
	verifyInclusionState(t, tangle, valueObjects["[-B, -C, E+] (Reattachment)"], true, false, true, false, false)

	// prefer [-A, F+] and thus Branch B
	setTransactionPreferredWithCheck(t, tangle, transactions["[-A, F+]"], true)
	verifyInclusionState(t, tangle, valueObjects["[-A, F+]"], true, false, true, false, false)
	verifyBranchState(t, tangle, valueObjects["[-A, F+]"], false, true, false, false)

	// prefer [-E, -F, G+]
	setTransactionPreferredWithCheck(t, tangle, transactions["[-E, -F, G+]"], true)
	verifyInclusionState(t, tangle, valueObjects["[-E, -F, G+]"], true, false, true, false, false)

	// verify states of other transactions, value objects and branches
	verifyInclusionState(t, tangle, valueObjects["[-A, D+]"], false, false, false, false, false)
	verifyBranchState(t, tangle, valueObjects["[-A, D+]"], false, false, false, false)

	verifyInclusionState(t, tangle, valueObjects["[-C, H+]"], false, false, false, false, false)
	verifyBranchState(t, tangle, valueObjects["[-C, H+]"], false, false, false, false)

	verifyInclusionState(t, tangle, valueObjects["[-H, -D, I+]"], false, false, false, false, false)

	verifyInclusionState(t, tangle, valueObjects["[-B, J+]"], false, false, false, false, false)
	verifyBranchState(t, tangle, valueObjects["[-B, J+]"], false, false, false, false)

	// prefer [-H, -D, I+] - should be liked after votes on [-A, D+] and [-C, H+]
	setTransactionPreferredWithCheck(t, tangle, transactions["[-H, -D, I+]"], true)
	verifyInclusionState(t, tangle, valueObjects["[-H, -D, I+]"], true, false, false, false, false)

	// simulate vote result to like [-A, D+] -> [-A, D+] becomes confirmed and [-A, F+], [-E, -F, G+] rejected
	setTransactionPreferredWithCheck(t, tangle, transactions["[-A, D+]"], true)
	setTransactionFinalizedWithCheck(t, tangle, transactions["[-A, D+]"])
	verifyInclusionState(t, tangle, valueObjects["[-A, D+]"], true, true, true, true, false)
	verifyBranchState(t, tangle, valueObjects["[-A, D+]"], true, true, true, false)

	verifyInclusionState(t, tangle, valueObjects["[-A, F+]"], false, true, false, false, true)
	verifyBranchState(t, tangle, valueObjects["[-A, F+]"], true, false, false, true)
	verifyInclusionState(t, tangle, valueObjects["[-E, -F, G+]"], false, true, false, false, true)

	// simulate vote result to like [-C, H+] -> [-C, H+] becomes confirmed and [-B, -C, E+], [-B, -C, E+] (Reattachment) rejected
	setTransactionPreferredWithCheck(t, tangle, transactions["[-C, H+]"], true)
	setTransactionFinalizedWithCheck(t, tangle, transactions["[-C, H+]"])
	verifyInclusionState(t, tangle, valueObjects["[-C, H+]"], true, true, true, true, false)
	verifyBranchState(t, tangle, valueObjects["[-C, H+]"], true, true, true, false)

	branches := make(map[string]branchmanager.BranchID)
	branches["A"] = branchmanager.NewBranchID(transactions["[-A, D+]"].ID())
	branches["C"] = branchmanager.NewBranchID(transactions["[-C, H+]"].ID())
	branches["AC"] = tangle.BranchManager().GenerateAggregatedBranchID(branches["A"], branches["C"])
	verifyBranchState2(t, tangle, branches["AC"], true, true, true, false)

	//verifyInclusionState(t, tangle, valueObjects["[-B, -C, E+]"], false, true, false, false, true)
	//verifyBranchState(t, tangle, valueObjects["[-B, -C, E+]"], true, false, false, true)
	//verifyInclusionState(t, tangle, valueObjects["[-B, -C, E+] (Reattachment)"], false, true, false, false, true)
	//
	//// [-H, -D, I+] should now be liked
	//verifyInclusionState(t, tangle, valueObjects["[-H, -D, I+]"], true, false, true, false, false)
	//setTransactionFinalizedWithCheck(t, tangle, transactions["[-H, -D, I+]"])
	//verifyInclusionState(t, tangle, valueObjects["[-H, -D, I+]"], true, true, true, true, false)
	//
	//// [-B, J+] should be unchanged
	//verifyInclusionState(t, tangle, valueObjects["[-B, J+]"], false, false, false, false, false)
	//// [-B, J+] should become confirmed after preferring and finalizing
	//setTransactionPreferredWithCheck(t, tangle, transactions["[-B, J+]"], true)
	//setTransactionFinalizedWithCheck(t, tangle, transactions["[-B, J+]"])
	//verifyInclusionState(t, tangle, valueObjects["[-B, J+]"], true, true, true, true, false)
}

func verifyBranchState(t *testing.T, tangle *Tangle, valueObject *payload.Payload, finalized, liked, confirmed, rejected bool) {
	assert.True(t, tangle.branchManager.Branch(branchmanager.NewBranchID(valueObject.Transaction().ID())).Consume(func(branch *branchmanager.Branch) {
		assert.Equalf(t, finalized, branch.Finalized(), "branch finalized state does not match")
		assert.Equalf(t, liked, branch.Liked(), "branch liked state does not match")

		assert.Equalf(t, confirmed, branch.Confirmed(), "branch confirmed state does not match")
		assert.Equalf(t, rejected, branch.Rejected(), "branch rejected state does not match")
	}))
}

func verifyBranchState2(t *testing.T, tangle *Tangle, id branchmanager.BranchID, finalized, liked, confirmed, rejected bool) {
	assert.True(t, tangle.branchManager.Branch(id).Consume(func(branch *branchmanager.Branch) {
		assert.Equalf(t, finalized, branch.Finalized(), "branch finalized state does not match")
		assert.Equalf(t, liked, branch.Liked(), "branch liked state does not match")

		assert.Equalf(t, confirmed, branch.Confirmed(), "branch confirmed state does not match")
		assert.Equalf(t, rejected, branch.Rejected(), "branch rejected state does not match")
	}))
}

func verifyInclusionState(t *testing.T, tangle *Tangle, valueObject *payload.Payload, preferred, finalized, liked, confirmed, rejected bool) {
	tx := valueObject.Transaction()

	// check outputs
	tx.Outputs().ForEach(func(address address.Address, balances []*balance.Balance) bool {
		assert.True(t, tangle.TransactionOutput(transaction.NewOutputID(address, tx.ID())).Consume(func(output *Output) {
			assert.Equalf(t, liked, output.Liked(), "output liked state does not match")
			assert.Equalf(t, confirmed, output.Confirmed(), "output confirmed state does not match")
			assert.Equalf(t, rejected, output.Rejected(), "output rejected state does not match")
		}))
		return true
	})

	// check transaction
	assert.True(t, tangle.TransactionMetadata(tx.ID()).Consume(func(metadata *TransactionMetadata) {
		assert.Equalf(t, preferred, metadata.Preferred(), "tx preferred state does not match")
		assert.Equalf(t, finalized, metadata.Finalized(), "tx finalized state does not match")

		assert.Equalf(t, liked, metadata.Liked(), "tx liked state does not match")
		assert.Equalf(t, confirmed, metadata.Confirmed(), "tx confirmed state does not match")
		assert.Equalf(t, rejected, metadata.Rejected(), "tx rejected state does not match")
	}))

	// check value object
	assert.True(t, tangle.PayloadMetadata(valueObject.ID()).Consume(func(payloadMetadata *PayloadMetadata) {
		assert.Equalf(t, liked, payloadMetadata.Liked(), "value object liked state does not match")
		assert.Equalf(t, confirmed, payloadMetadata.Confirmed(), "value object confirmed state does not match")
		assert.Equalf(t, rejected, payloadMetadata.Rejected(), "value object rejected state does not match")
	}))
}

func setTransactionPreferredWithCheck(t *testing.T, tangle *Tangle, tx *transaction.Transaction, preferred bool) {
	modified, err := tangle.SetTransactionPreferred(tx.ID(), preferred)
	require.NoError(t, err)
	assert.True(t, modified)
}
func setTransactionFinalizedWithCheck(t *testing.T, tangle *Tangle, tx *transaction.Transaction) {
	modified, err := tangle.SetTransactionFinalized(tx.ID())
	require.NoError(t, err)
	assert.True(t, modified)
}

func TestLucasScenario(t *testing.T) {
	// create tangle
	tangle := New(mapdb.NewMapDB())
	defer tangle.Shutdown()

	// create seed for testing
	seed := wallet.NewSeed()

	// initialize tangle with genesis block (+GENESIS)
	tangle.LoadSnapshot(map[transaction.ID]map[address.Address][]*balance.Balance{
		transaction.GenesisID: {
			seed.Address(GENESIS): {
				balance.New(balance.ColorIOTA, 3333),
			},
		},
	})

	// create dictionaries so we can address the created entities by their aliases from the picture
	transactions := make(map[string]*transaction.Transaction)
	valueObjects := make(map[string]*payload.Payload)
	branches := make(map[string]branchmanager.BranchID)

	// [-GENESIS, A+, B+, C+]
	{
		// create transaction + payload
		transactions["[-GENESIS, A+, B+, C+]"] = transaction.New(
			transaction.NewInputs(
				transaction.NewOutputID(seed.Address(GENESIS), transaction.GenesisID),
			),

			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				seed.Address(A): {
					balance.New(balance.ColorIOTA, 1111),
				},
				seed.Address(B): {
					balance.New(balance.ColorIOTA, 1111),
				},
				seed.Address(C): {
					balance.New(balance.ColorIOTA, 1111),
				},
			}),
		)
		transactions["[-GENESIS, A+, B+, C+]"].Sign(signaturescheme.ED25519(*seed.KeyPair(GENESIS)))
		valueObjects["[-GENESIS, A+, B+, C+]"] = payload.New(payload.GenesisID, payload.GenesisID, transactions["[-GENESIS, A+, B+, C+]"])

		// check if signatures are valid
		assert.True(t, transactions["[-GENESIS, A+, B+, C+]"].SignaturesValid())

		// attach payload
		tangle.AttachPayloadSync(valueObjects["[-GENESIS, A+, B+, C+]"])

		// check if transaction metadata is found in database
		assert.True(t, tangle.TransactionMetadata(transactions["[-GENESIS, A+, B+, C+]"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.True(t, transactionMetadata.Solid(), "the transaction is not solid")
			assert.Equal(t, branchmanager.MasterBranchID, transactionMetadata.BranchID(), "the transaction was booked into the wrong branch")
		}))

		// check if payload metadata is found in database
		assert.True(t, tangle.PayloadMetadata(valueObjects["[-GENESIS, A+, B+, C+]"].ID()).Consume(func(payloadMetadata *PayloadMetadata) {
			assert.True(t, payloadMetadata.IsSolid(), "the payload is not solid")
			assert.Equal(t, branchmanager.MasterBranchID, payloadMetadata.BranchID(), "the payload was booked into the wrong branch")
		}))

		// check if the balance on address GENESIS is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(GENESIS)).Consume(func(output *Output) {
			assert.Equal(t, 1, output.ConsumerCount(), "the output should be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 3333)}, output.Balances())
			assert.Equal(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// check if the balance on address A is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(A)).Consume(func(output *Output) {
			assert.Equal(t, 0, output.ConsumerCount(), "the output should not be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// check if the balance on address B is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(B)).Consume(func(output *Output) {
			assert.Equal(t, 0, output.ConsumerCount(), "the output should not be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// check if the balance on address C is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(C)).Consume(func(output *Output) {
			assert.Equal(t, 0, output.ConsumerCount(), "the output should not be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))
	}

	// [-A, D+]
	{
		// create transaction + payload
		transactions["[-A, D+]"] = transaction.New(
			transaction.NewInputs(
				transaction.NewOutputID(seed.Address(A), transactions["[-GENESIS, A+, B+, C+]"].ID()),
			),

			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				seed.Address(D): {
					balance.New(balance.ColorIOTA, 1111),
				},
			}),
		)
		transactions["[-A, D+]"].Sign(signaturescheme.ED25519(*seed.KeyPair(A)))
		valueObjects["[-A, D+]"] = payload.New(payload.GenesisID, valueObjects["[-GENESIS, A+, B+, C+]"].ID(), transactions["[-A, D+]"])

		// check if signatures are valid
		assert.True(t, transactions["[-A, D+]"].SignaturesValid())

		// attach payload
		tangle.AttachPayloadSync(valueObjects["[-A, D+]"])

		// check if payload metadata is found in database
		assert.True(t, tangle.TransactionMetadata(transactions["[-A, D+]"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.True(t, transactionMetadata.Solid(), "the transaction is not solid")
			assert.Equal(t, branchmanager.MasterBranchID, transactionMetadata.BranchID(), "the transaction was booked into the wrong branch")
		}))

		// check if transaction metadata is found in database
		assert.True(t, tangle.PayloadMetadata(valueObjects["[-A, D+]"].ID()).Consume(func(payloadMetadata *PayloadMetadata) {
			assert.True(t, payloadMetadata.IsSolid(), "the payload is not solid")
			assert.Equal(t, branchmanager.MasterBranchID, payloadMetadata.BranchID(), "the payload was booked into the wrong branch")
		}))

		// check if the balance on address A is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(A)).Consume(func(output *Output) {
			assert.Equal(t, 1, output.ConsumerCount(), "the output should be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// check if the balance on address D is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(D)).Consume(func(output *Output) {
			assert.Equal(t, 0, output.ConsumerCount(), "the output should not be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))
	}

	// [-B, -C, E+]
	{
		// create transaction + payload
		transactions["[-B, -C, E+]"] = transaction.New(
			transaction.NewInputs(
				transaction.NewOutputID(seed.Address(B), transactions["[-GENESIS, A+, B+, C+]"].ID()),
				transaction.NewOutputID(seed.Address(C), transactions["[-GENESIS, A+, B+, C+]"].ID()),
			),

			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				seed.Address(E): {
					balance.New(balance.ColorIOTA, 2222),
				},
			}),
		)
		transactions["[-B, -C, E+]"].Sign(signaturescheme.ED25519(*seed.KeyPair(B)))
		transactions["[-B, -C, E+]"].Sign(signaturescheme.ED25519(*seed.KeyPair(C)))
		valueObjects["[-B, -C, E+]"] = payload.New(payload.GenesisID, valueObjects["[-GENESIS, A+, B+, C+]"].ID(), transactions["[-B, -C, E+]"])

		// check if signatures are valid
		assert.True(t, transactions["[-B, -C, E+]"].SignaturesValid())

		// attach payload
		tangle.AttachPayloadSync(valueObjects["[-B, -C, E+]"])

		// check if payload metadata is found in database
		assert.True(t, tangle.TransactionMetadata(transactions["[-B, -C, E+]"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.True(t, transactionMetadata.Solid(), "the transaction is not solid")
			assert.Equal(t, branchmanager.MasterBranchID, transactionMetadata.BranchID(), "the transaction was booked into the wrong branch")
		}))

		// check if transaction metadata is found in database
		assert.True(t, tangle.PayloadMetadata(valueObjects["[-B, -C, E+]"].ID()).Consume(func(payloadMetadata *PayloadMetadata) {
			assert.True(t, payloadMetadata.IsSolid(), "the payload is not solid")
			assert.Equal(t, branchmanager.MasterBranchID, payloadMetadata.BranchID(), "the payload was booked into the wrong branch")
		}))

		// check if the balance on address B is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(B)).Consume(func(output *Output) {
			assert.Equal(t, 1, output.ConsumerCount(), "the output should be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// check if the balance on address C is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(C)).Consume(func(output *Output) {
			assert.Equal(t, 1, output.ConsumerCount(), "the output should be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// check if the balance on address E is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(E)).Consume(func(output *Output) {
			assert.Equal(t, 0, output.ConsumerCount(), "the output should not be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 2222)}, output.Balances())
			assert.Equal(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))
	}

	// [-B, -C, E+] (Reattachment)
	{
		// create payload
		valueObjects["[-B, -C, E+] (Reattachment)"] = payload.New(valueObjects["[-B, -C, E+]"].ID(), valueObjects["[-GENESIS, A+, B+, C+]"].ID(), transactions["[-B, -C, E+]"])

		// attach payload
		tangle.AttachPayloadSync(valueObjects["[-B, -C, E+] (Reattachment)"])

		// check if transaction metadata is found in database
		assert.True(t, tangle.TransactionMetadata(transactions["[-B, -C, E+]"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.True(t, transactionMetadata.Solid(), "the transaction is not solid")
			assert.Equal(t, branchmanager.MasterBranchID, transactionMetadata.BranchID(), "the transaction was booked into the wrong branch")
		}))

		// check if payload metadata is found in database
		assert.True(t, tangle.PayloadMetadata(valueObjects["[-B, -C, E+] (Reattachment)"].ID()).Consume(func(payloadMetadata *PayloadMetadata) {
			assert.True(t, payloadMetadata.IsSolid(), "the payload is not solid")
			assert.Equal(t, branchmanager.MasterBranchID, payloadMetadata.BranchID(), "the payload was booked into the wrong branch")
		}))

		// check if the balance on address B is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(B)).Consume(func(output *Output) {
			assert.Equal(t, 1, output.ConsumerCount(), "the output should be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// check if the balance on address C is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(C)).Consume(func(output *Output) {
			assert.Equal(t, 1, output.ConsumerCount(), "the output should be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// check if the balance on address E is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(E)).Consume(func(output *Output) {
			assert.Equal(t, 0, output.ConsumerCount(), "the output should not be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 2222)}, output.Balances())
			assert.Equal(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))
	}

	// [-A, F+]
	{
		// create transaction + payload
		transactions["[-A, F+]"] = transaction.New(
			transaction.NewInputs(
				transaction.NewOutputID(seed.Address(A), transactions["[-GENESIS, A+, B+, C+]"].ID()),
			),

			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				seed.Address(F): {
					balance.New(balance.ColorIOTA, 1111),
				},
			}),
		)
		transactions["[-A, F+]"].Sign(signaturescheme.ED25519(*seed.KeyPair(A)))
		valueObjects["[-A, F+]"] = payload.New(payload.GenesisID, valueObjects["[-GENESIS, A+, B+, C+]"].ID(), transactions["[-A, F+]"])

		// check if signatures are valid
		assert.True(t, transactions["[-A, F+]"].SignaturesValid())

		// attach payload
		tangle.AttachPayloadSync(valueObjects["[-A, F+]"])

		// create aliases for the branches
		branches["A"] = branchmanager.NewBranchID(transactions["[-A, D+]"].ID())
		branches["B"] = branchmanager.NewBranchID(transactions["[-A, F+]"].ID())

		// check if payload metadata is found in database
		assert.True(t, tangle.TransactionMetadata(transactions["[-A, F+]"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.True(t, transactionMetadata.Solid(), "the transaction is not solid")
			assert.Equal(t, branches["B"], transactionMetadata.BranchID(), "the transaction was booked into the wrong branch")
		}))

		// check if transaction metadata is found in database
		assert.True(t, tangle.PayloadMetadata(valueObjects["[-A, F+]"].ID()).Consume(func(payloadMetadata *PayloadMetadata) {
			assert.True(t, payloadMetadata.IsSolid(), "the payload is not solid")
			assert.Equal(t, branches["B"], payloadMetadata.BranchID(), "the payload was booked into the wrong branch")
		}))

		// check if the balance on address A is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(A)).Consume(func(output *Output) {
			assert.Equal(t, 2, output.ConsumerCount(), "the output should be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// check if the balance on address F is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(F)).Consume(func(output *Output) {
			assert.Equal(t, 0, output.ConsumerCount(), "the output should not be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branches["B"], output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// check if the balance on address D is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(D)).Consume(func(output *Output) {
			assert.Equal(t, 0, output.ConsumerCount(), "the output should be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branches["A"], output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// check if transaction metadata is found in database
		assert.True(t, tangle.PayloadMetadata(valueObjects["[-A, D+]"].ID()).Consume(func(payloadMetadata *PayloadMetadata) {
			assert.True(t, payloadMetadata.IsSolid(), "the payload is not solid")
			assert.Equal(t, branches["A"], payloadMetadata.BranchID(), "the payload was booked into the wrong branch")
		}))

		// check if the branches are conflicting
		branchesConflicting, err := tangle.branchManager.BranchesConflicting(branches["A"], branches["B"])
		require.NoError(t, err)
		assert.True(t, branchesConflicting, "the branches should be conflicting")
	}

	// [-E, -F, G+]
	{
		// create transaction + payload
		transactions["[-E, -F, G+]"] = transaction.New(
			transaction.NewInputs(
				transaction.NewOutputID(seed.Address(E), transactions["[-B, -C, E+]"].ID()),
				transaction.NewOutputID(seed.Address(F), transactions["[-A, F+]"].ID()),
			),

			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				seed.Address(G): {
					balance.New(balance.ColorIOTA, 3333),
				},
			}),
		)
		transactions["[-E, -F, G+]"].Sign(signaturescheme.ED25519(*seed.KeyPair(E)))
		transactions["[-E, -F, G+]"].Sign(signaturescheme.ED25519(*seed.KeyPair(F)))
		valueObjects["[-E, -F, G+]"] = payload.New(valueObjects["[-B, -C, E+]"].ID(), valueObjects["[-A, F+]"].ID(), transactions["[-E, -F, G+]"])

		// check if signatures are valid
		assert.True(t, transactions["[-E, -F, G+]"].SignaturesValid())

		// attach payload
		tangle.AttachPayloadSync(valueObjects["[-E, -F, G+]"])

		// check if payload metadata is found in database
		assert.True(t, tangle.TransactionMetadata(transactions["[-E, -F, G+]"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.True(t, transactionMetadata.Solid(), "the transaction is not solid")
			assert.Equal(t, branches["B"], transactionMetadata.BranchID(), "the transaction was booked into the wrong branch")

			log.Println("-E, -F, G+] Preferred:", transactionMetadata.Preferred())
			log.Println("-E, -F, G+] Finalized:", transactionMetadata.Finalized())
		}))

		// check if transaction metadata is found in database
		assert.True(t, tangle.PayloadMetadata(valueObjects["[-E, -F, G+]"].ID()).Consume(func(payloadMetadata *PayloadMetadata) {
			assert.True(t, payloadMetadata.IsSolid(), "the payload is not solid")
			assert.Equal(t, branches["B"], payloadMetadata.BranchID(), "the payload was booked into the wrong branch")

			log.Println("-E, -F, G+] Payload Liked:", payloadMetadata.Liked())
			log.Println("-E, -F, G+] Payload Confirmed:", payloadMetadata.Confirmed())
			log.Println("-E, -F, G+] Payload Rejected:", payloadMetadata.Rejected())
		}))

		// check if the balance on address E is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(E)).Consume(func(output *Output) {
			assert.Equal(t, 1, output.ConsumerCount(), "the output should be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 2222)}, output.Balances())
			assert.Equal(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// check if the balance on address F is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(F)).Consume(func(output *Output) {
			assert.Equal(t, 1, output.ConsumerCount(), "the output should be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branches["B"], output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// check if the balance on address G is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(G)).Consume(func(output *Output) {
			assert.Equal(t, 0, output.ConsumerCount(), "the output should be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 3333)}, output.Balances())
			assert.Equal(t, branches["B"], output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		////////

		modified, err := tangle.SetTransactionFinalized(transactions["[-A, F+]"].ID())
		require.NoError(t, err)
		assert.True(t, modified)

		assert.True(t, tangle.TransactionMetadata(transactions["[-A, F+]"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			log.Println("[-A, F+] Preferred:", transactionMetadata.Preferred())
			log.Println("[-A, F+] Finalized:", transactionMetadata.Finalized())
		}))

		assert.True(t, tangle.PayloadMetadata(valueObjects["[-A, F+]"].ID()).Consume(func(payloadMetadata *PayloadMetadata) {
			log.Println("[-A, F+] Payload Liked:", payloadMetadata.Liked())
			log.Println("[-A, F+] Payload Confirmed:", payloadMetadata.Confirmed())
			log.Println("[-A, F+] Payload Rejected:", payloadMetadata.Rejected())
		}))

		// check if payload metadata is found in database
		assert.True(t, tangle.TransactionMetadata(transactions["[-E, -F, G+]"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.True(t, transactionMetadata.Solid(), "the transaction is not solid")
			assert.Equal(t, branches["B"], transactionMetadata.BranchID(), "the transaction was booked into the wrong branch")

			log.Println("-E, -F, G+] Preferred:", transactionMetadata.Preferred())
			log.Println("-E, -F, G+] Finalized:", transactionMetadata.Finalized())
		}))

		// check if transaction metadata is found in database
		assert.True(t, tangle.PayloadMetadata(valueObjects["[-E, -F, G+]"].ID()).Consume(func(payloadMetadata *PayloadMetadata) {
			assert.True(t, payloadMetadata.IsSolid(), "the payload is not solid")
			assert.Equal(t, branches["B"], payloadMetadata.BranchID(), "the payload was booked into the wrong branch")

			log.Println("-E, -F, G+] Payload Liked:", payloadMetadata.Liked())
			log.Println("-E, -F, G+] Payload Confirmed:", payloadMetadata.Confirmed())
			log.Println("-E, -F, G+] Payload Rejected:", payloadMetadata.Rejected())
		}))

		////////
	}

	// [-F, -D, Y+]
	{
		// create transaction + payload
		transactions["[-F, -D, Y+]"] = transaction.New(
			transaction.NewInputs(
				transaction.NewOutputID(seed.Address(D), transactions["[-A, D+]"].ID()),
				transaction.NewOutputID(seed.Address(F), transactions["[-A, F+]"].ID()),
			),

			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				seed.Address(Y): {
					balance.New(balance.ColorIOTA, 2222),
				},
			}),
		)
		transactions["[-F, -D, Y+]"].Sign(signaturescheme.ED25519(*seed.KeyPair(D)))
		transactions["[-F, -D, Y+]"].Sign(signaturescheme.ED25519(*seed.KeyPair(F)))
		valueObjects["[-F, -D, Y+]"] = payload.New(valueObjects["[-A, F+]"].ID(), valueObjects["[-A, D+]"].ID(), transactions["[-F, -D, Y+]"])

		// check if signatures are valid
		assert.True(t, transactions["[-F, -D, Y+]"].SignaturesValid())

		// attach payload
		tangle.AttachPayloadSync(valueObjects["[-F, -D, Y+]"])

		// check if all of the invalids transactions models were deleted
		assert.False(t, tangle.Transaction(transactions["[-F, -D, Y+]"].ID()).Consume(func(metadata *transaction.Transaction) {}), "the transaction should not be found")
		assert.False(t, tangle.TransactionMetadata(transactions["[-F, -D, Y+]"].ID()).Consume(func(metadata *TransactionMetadata) {}), "the transaction metadata should not be found")
		assert.False(t, tangle.Payload(valueObjects["[-F, -D, Y+]"].ID()).Consume(func(payload *payload.Payload) {}), "the payload should not be found")
		assert.False(t, tangle.PayloadMetadata(valueObjects["[-F, -D, Y+]"].ID()).Consume(func(payloadMetadata *PayloadMetadata) {}), "the payload metadata should not be found")
		assert.True(t, tangle.Approvers(valueObjects["[-A, F+]"].ID()).Consume(func(approver *PayloadApprover) {
			assert.NotEqual(t, approver.ApprovingPayloadID(), valueObjects["[-F, -D, Y+]"].ID(), "the invalid value object should not show up as an approver")
		}), "the should be approvers of the referenced output")
		assert.False(t, tangle.Approvers(valueObjects["[-A, D+]"].ID()).Consume(func(approver *PayloadApprover) {}), "approvers should be empty")
		assert.False(t, tangle.Attachments(transactions["[-F, -D, Y+]"].ID()).Consume(func(attachment *Attachment) {}), "the transaction should not have any attachments")
		assert.False(t, tangle.Consumers(transaction.NewOutputID(seed.Address(D), transactions["[-A, D+]"].ID())).Consume(func(consumer *Consumer) {}), "the consumers of the used input should be empty")
		assert.True(t, tangle.Consumers(transaction.NewOutputID(seed.Address(F), transactions["[-A, F+]"].ID())).Consume(func(consumer *Consumer) {
			assert.NotEqual(t, consumer.TransactionID(), transactions["[-F, -D, Y+]"].ID(), "the consumers should not contain the invalid transaction")
		}), "the consumers should not be empty")
	}

	// [-B, -C, E+] (2nd Reattachment)
	{
		valueObjects["[-B, -C, E+] (2nd Reattachment)"] = payload.New(valueObjects["[-A, F+]"].ID(), valueObjects["[-A, D+]"].ID(), transactions["[-B, -C, E+]"])

		// attach payload
		tangle.AttachPayloadSync(valueObjects["[-B, -C, E+] (2nd Reattachment)"])

		// check if all of the valid transactions models were NOT deleted
		assert.True(t, tangle.Transaction(transactions["[-B, -C, E+]"].ID()).Consume(func(metadata *transaction.Transaction) {}))

		// check if transaction metadata is found in database
		assert.True(t, tangle.TransactionMetadata(transactions["[-B, -C, E+]"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.True(t, transactionMetadata.Solid(), "the transaction is not solid")
			assert.Equal(t, branchmanager.MasterBranchID, transactionMetadata.BranchID(), "the transaction was booked into the wrong branch")
		}))

		// check if payload and its corresponding models are not found in the database (payload was invalid)
		assert.False(t, tangle.Payload(valueObjects["[-B, -C, E+] (2nd Reattachment)"].ID()).Consume(func(payload *payload.Payload) {}), "the payload should not exist")
		assert.False(t, tangle.PayloadMetadata(valueObjects["[-B, -C, E+] (2nd Reattachment)"].ID()).Consume(func(payloadMetadata *PayloadMetadata) {}), "the payload metadata should not exist")
		assert.True(t, tangle.Attachments(transactions["[-B, -C, E+]"].ID()).Consume(func(attachment *Attachment) {
			assert.NotEqual(t, valueObjects["[-B, -C, E+] (2nd Reattachment)"].ID(), attachment.PayloadID(), "the attachment to the payload should be deleted")
		}), "there should be attachments of the transaction")
		assert.True(t, tangle.Approvers(valueObjects["[-A, F+]"].ID()).Consume(func(approver *PayloadApprover) {
			assert.NotEqual(t, valueObjects["[-A, F+]"].ID(), approver.ApprovingPayloadID(), "there should not be an approver reference to the invalid payload")
			assert.NotEqual(t, valueObjects["[-A, D+]"].ID(), approver.ApprovingPayloadID(), "there should not be an approver reference to the invalid payload")
		}), "there should be approvers")
		assert.False(t, tangle.Approvers(valueObjects["[-A, D+]"].ID()).Consume(func(approver *PayloadApprover) {}), "there should be no approvers")
	}

	// [-C, H+]
	{
		// create transaction + payload
		transactions["[-C, H+]"] = transaction.New(
			transaction.NewInputs(
				transaction.NewOutputID(seed.Address(C), transactions["[-GENESIS, A+, B+, C+]"].ID()),
			),

			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				seed.Address(H): {
					balance.New(balance.ColorIOTA, 1111),
				},
			}),
		)
		transactions["[-C, H+]"].Sign(signaturescheme.ED25519(*seed.KeyPair(C)))
		valueObjects["[-C, H+]"] = payload.New(valueObjects["[-GENESIS, A+, B+, C+]"].ID(), valueObjects["[-A, D+]"].ID(), transactions["[-C, H+]"])

		// check if signatures are valid
		assert.True(t, transactions["[-C, H+]"].SignaturesValid())

		// attach payload
		tangle.AttachPayloadSync(valueObjects["[-C, H+]"])

		// create alias for the branch
		branches["C"] = branchmanager.NewBranchID(transactions["[-C, H+]"].ID())
		branches["AC"] = tangle.BranchManager().GenerateAggregatedBranchID(branches["A"], branches["C"])

		// check if transaction metadata is found in database
		assert.True(t, tangle.TransactionMetadata(transactions["[-C, H+]"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.True(t, transactionMetadata.Solid(), "the transaction is not solid")
			assert.Equal(t, branches["C"], transactionMetadata.BranchID(), "the transaction was booked into the wrong branch")
		}))

		// check if payload metadata is found in database
		assert.True(t, tangle.PayloadMetadata(valueObjects["[-C, H+]"].ID()).Consume(func(payloadMetadata *PayloadMetadata) {
			assert.True(t, payloadMetadata.IsSolid(), "the payload is not solid")
			assert.NotEqual(t, branches["C"], payloadMetadata.BranchID(), "the payload was booked into the wrong branch")
			assert.Equal(t, branches["AC"], payloadMetadata.BranchID(), "the payload was booked into the wrong branch")
		}))

		// check if the balance on address C is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(C)).Consume(func(output *Output) {
			assert.Equal(t, 2, output.ConsumerCount(), "the output should be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// check if the balance on address H is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(H)).Consume(func(output *Output) {
			assert.Equal(t, 0, output.ConsumerCount(), "the output should not be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branches["C"], output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// Branch D

		// create alias for the branch
		branches["D"] = branchmanager.NewBranchID(transactions["[-B, -C, E+]"].ID())
		branches["BD"] = tangle.branchManager.GenerateAggregatedBranchID(branches["B"], branches["D"])

		{
			// check if transaction metadata is found in database
			assert.True(t, tangle.PayloadMetadata(valueObjects["[-B, -C, E+]"].ID()).Consume(func(payloadMetadata *PayloadMetadata) {
				assert.True(t, payloadMetadata.IsSolid(), "the payload is not solid")
				assert.Equal(t, branches["D"], payloadMetadata.BranchID(), "the payload was booked into the wrong branch")
			}))

			// check if transaction metadata is found in database
			assert.True(t, tangle.PayloadMetadata(valueObjects["[-B, -C, E+] (Reattachment)"].ID()).Consume(func(payloadMetadata *PayloadMetadata) {
				assert.True(t, payloadMetadata.IsSolid(), "the payload is not solid")
				assert.Equal(t, branches["D"], payloadMetadata.BranchID(), "the payload was booked into the wrong branch")
			}))
		}

		// check if the branches C and D are conflicting
		branchesConflicting, err := tangle.branchManager.BranchesConflicting(branches["C"], branches["D"])
		require.NoError(t, err)
		assert.True(t, branchesConflicting, "the branches should be conflicting")

		// Aggregated Branch [BD]
		{
			// check if transaction metadata is found in database
			assert.True(t, tangle.PayloadMetadata(valueObjects["[-E, -F, G+]"].ID()).Consume(func(payloadMetadata *PayloadMetadata) {
				assert.True(t, payloadMetadata.IsSolid(), "the payload is not solid")
				assert.Equal(t, branches["BD"], payloadMetadata.BranchID(), "the payload was booked into the wrong branch")
			}))

			// check if transaction metadata is found in database
			assert.True(t, tangle.PayloadMetadata(valueObjects["[-E, -F, G+]"].ID()).Consume(func(payloadMetadata *PayloadMetadata) {
				assert.True(t, payloadMetadata.IsSolid(), "the payload is not solid")
				assert.Equal(t, branches["BD"], payloadMetadata.BranchID(), "the payload was booked into the wrong branch")
			}))
		}
	}

	// [-H, -D, I+]
	{
		// create transaction + payload
		transactions["[-H, -D, I+]"] = transaction.New(
			transaction.NewInputs(
				transaction.NewOutputID(seed.Address(H), transactions["[-C, H+]"].ID()),
				transaction.NewOutputID(seed.Address(D), transactions["[-A, D+]"].ID()),
			),

			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				seed.Address(I): {
					balance.New(balance.ColorIOTA, 2222),
				},
			}),
		)
		transactions["[-H, -D, I+]"].Sign(signaturescheme.ED25519(*seed.KeyPair(H)))
		transactions["[-H, -D, I+]"].Sign(signaturescheme.ED25519(*seed.KeyPair(D)))
		valueObjects["[-H, -D, I+]"] = payload.New(valueObjects["[-C, H+]"].ID(), valueObjects["[-A, D+]"].ID(), transactions["[-H, -D, I+]"])

		// check if signatures are valid
		assert.True(t, transactions["[-H, -D, I+]"].SignaturesValid())

		// attach payload
		tangle.AttachPayloadSync(valueObjects["[-H, -D, I+]"])

		// create alias for the branch
		branches["AC"] = tangle.branchManager.GenerateAggregatedBranchID(branches["A"], branches["C"])

		// check if transaction metadata is found in database
		assert.True(t, tangle.TransactionMetadata(transactions["[-H, -D, I+]"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.True(t, transactionMetadata.Solid(), "the transaction is not solid")
			assert.Equal(t, branches["AC"], transactionMetadata.BranchID(), "the transaction was booked into the wrong branch")
		}))

		// check if payload metadata is found in database
		assert.True(t, tangle.PayloadMetadata(valueObjects["[-H, -D, I+]"].ID()).Consume(func(payloadMetadata *PayloadMetadata) {
			assert.True(t, payloadMetadata.IsSolid(), "the payload is not solid")
			assert.Equal(t, branches["AC"], payloadMetadata.BranchID(), "the payload was booked into the wrong branch")
		}))

		// check if the balance on address H is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(H)).Consume(func(output *Output) {
			assert.Equal(t, 1, output.ConsumerCount(), "the output should be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branches["C"], output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// check if the balance on address D is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(D)).Consume(func(output *Output) {
			assert.Equal(t, 1, output.ConsumerCount(), "the output should be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branches["A"], output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// check if the balance on address I is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(I)).Consume(func(output *Output) {
			assert.Equal(t, 0, output.ConsumerCount(), "the output should not be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 2222)}, output.Balances())
			assert.Equal(t, branches["AC"], output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))
	}

	// [-B, J+]
	{
		// create transaction + payload
		transactions["[-B, J+]"] = transaction.New(
			transaction.NewInputs(
				transaction.NewOutputID(seed.Address(B), transactions["[-GENESIS, A+, B+, C+]"].ID()),
			),

			transaction.NewOutputs(map[address.Address][]*balance.Balance{
				seed.Address(J): {
					balance.New(balance.ColorIOTA, 1111),
				},
			}),
		)
		transactions["[-B, J+]"].Sign(signaturescheme.ED25519(*seed.KeyPair(B)))
		valueObjects["[-B, J+]"] = payload.New(valueObjects["[-C, H+]"].ID(), valueObjects["[-A, D+]"].ID(), transactions["[-B, J+]"])

		// check if signatures are valid
		assert.True(t, transactions["[-B, J+]"].SignaturesValid())

		// attach payload
		tangle.AttachPayloadSync(valueObjects["[-B, J+]"])

		// create alias for the branch
		branches["E"] = branchmanager.NewBranchID(transactions["[-B, J+]"].ID())

		// check if transaction metadata is found in database
		assert.True(t, tangle.TransactionMetadata(transactions["[-B, J+]"].ID()).Consume(func(transactionMetadata *TransactionMetadata) {
			assert.True(t, transactionMetadata.Solid(), "the transaction is not solid")
			assert.Equal(t, branches["E"], transactionMetadata.BranchID(), "the transaction was booked into the wrong branch")
		}))

		// create alias for the branch
		branches["ACE"] = tangle.branchManager.GenerateAggregatedBranchID(branches["A"], branches["C"], branches["E"])

		// check if payload metadata is found in database
		assert.True(t, tangle.PayloadMetadata(valueObjects["[-B, J+]"].ID()).Consume(func(payloadMetadata *PayloadMetadata) {
			assert.True(t, payloadMetadata.IsSolid(), "the payload is not solid")
			assert.Equal(t, branches["ACE"], payloadMetadata.BranchID(), "the payload was booked into the wrong branch")
		}))

		// check if the balance on address B is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(B)).Consume(func(output *Output) {
			assert.Equal(t, 2, output.ConsumerCount(), "the output should be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branchmanager.MasterBranchID, output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// check if the balance on address J is found in the database
		assert.True(t, tangle.OutputsOnAddress(seed.Address(J)).Consume(func(output *Output) {
			assert.Equal(t, 0, output.ConsumerCount(), "the output should not be spent")
			assert.Equal(t, []*balance.Balance{balance.New(balance.ColorIOTA, 1111)}, output.Balances())
			assert.Equal(t, branches["E"], output.BranchID(), "the output was booked into the wrong branch")
			assert.True(t, output.Solid(), "the output is not solid")
		}))

		// check if the branches D and E are conflicting
		branchesConflicting, err := tangle.branchManager.BranchesConflicting(branches["D"], branches["E"])
		require.NoError(t, err)
		assert.True(t, branchesConflicting, "the branches should be conflicting")

	}
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
			totals[bal.Color()] += bal.Value()
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
