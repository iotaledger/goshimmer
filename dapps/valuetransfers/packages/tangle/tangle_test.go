package tangle

import (
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/branchmanager"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		snapshot := map[transaction.ID]map[address.Address][]*balance.Balance{transaction.GenesisID: outputs}
		tangle.LoadSnapshot(snapshot)

		// build tx spending "outputs"
		inputIDs := make([]transaction.OutputID, 0)
		for addr := range outputs {
			inputIDs = append(inputIDs, transaction.NewOutputID(addr, transaction.GenesisID))
		}
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
		snapshot := map[transaction.ID]map[address.Address][]*balance.Balance{transaction.GenesisID: outputs}
		tangle.LoadSnapshot(snapshot)

		// build tx spending "outputs"
		inputIDs := make([]transaction.OutputID, 0)
		for addr := range outputs {
			inputIDs = append(inputIDs, transaction.NewOutputID(addr, transaction.GenesisID))
		}
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
		snapshot := map[transaction.ID]map[address.Address][]*balance.Balance{transaction.GenesisID: outputs}
		tangle.LoadSnapshot(snapshot)

		// build tx spending "outputs"
		inputIDs := make([]transaction.OutputID, 0)
		for addr := range outputs {
			inputIDs = append(inputIDs, transaction.NewOutputID(addr, transaction.GenesisID))
		}
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
		snapshot := map[transaction.ID]map[address.Address][]*balance.Balance{transaction.GenesisID: outputs}
		tangle.LoadSnapshot(snapshot)

		// build tx spending "outputs"
		inputIDs := make([]transaction.OutputID, 0)
		for addr := range outputs {
			inputIDs = append(inputIDs, transaction.NewOutputID(addr, transaction.GenesisID))
		}
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
	//{
	//	tangle := New(mapdb.NewMapDB())
	//
	//	// create conflicting branches
	//	cachedBranch2, _ := tangle.BranchManager().Fork(branchmanager.BranchID{2}, []branchmanager.BranchID{branchmanager.MasterBranchID}, []branchmanager.ConflictID{{0}})
	//	branch2 := cachedBranch2.Unwrap()
	//	cachedBranch3, _ := tangle.BranchManager().Fork(branchmanager.BranchID{3}, []branchmanager.BranchID{branchmanager.MasterBranchID}, []branchmanager.ConflictID{{0}})
	//	branch3 := cachedBranch3.Unwrap()
	//	// create outputs for conflicting branches
	//	var cachedOutputs []*CachedOutput
	//	for _, branch := range []*branchmanager.Branch{branch2, branch3} {
	//		input := NewOutput(address.Random(), transaction.GenesisID, branch.ID(), []*balance.Balance{balance.New(balance.ColorIOTA, 1)})
	//		input.SetSolid(true)
	//		cachedObject, _ := tangle.outputStorage.StoreIfAbsent(input)
	//		defer cachedObject.Release()
	//		cachedOutputs = append(cachedOutputs, &CachedOutput{CachedObject: cachedObject})
	//	}
	//
	//}

}

func TestPayloadBranchID(t *testing.T) {

}

func TestCheckPayloadSolidity(t *testing.T) {

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
