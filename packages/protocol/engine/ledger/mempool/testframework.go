package mempool

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/xerrors"

	"github.com/iotaledger/goshimmer/packages/core/confirmation"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/mockedvm"
	"github.com/iotaledger/hive.go/ds/advancedset"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

// TestFramework provides common testing functionality for the ledger package. As such, it helps to easily build an
// UTXO-DAG by specifying transactions outputs/inputs via aliases.
// It makes use of a simplified MockedVM, with MockedTransaction, MockedOutput and MockedInput.
type TestFramework struct {
	// Instance contains a reference to the MemPool instance that the TestFramework is using.
	Instance MemPool

	ConflictDAG *conflictdag.TestFramework

	// test contains a reference to the testing instance.
	test *testing.T

	// transactionsByAlias contains a dictionary that maps a human-readable alias to a MockedTransaction.
	transactionsByAlias map[string]*mockedvm.MockedTransaction

	// transactionsByAliasMutex contains a mutex that is used to synchronize parallel access to the transactionsByAlias.
	transactionsByAliasMutex sync.RWMutex

	// outputIDsByAlias contains a dictionary that maps a human-readable alias to an OutputID.
	outputIDsByAlias map[string]utxo.OutputID

	// outputIDsByAliasMutex contains a mutex that is used to synchronize parallel access to the outputIDsByAlias.
	outputIDsByAliasMutex sync.RWMutex
}

// NewTestFramework creates a new instance of the TestFramework with one default output "Genesis" which has to be
// consumed by the first transaction.
func NewTestFramework(test *testing.T, instance MemPool) *TestFramework {
	t := &TestFramework{
		test:                test,
		Instance:            instance,
		ConflictDAG:         conflictdag.NewTestFramework(test, instance.ConflictDAG()),
		transactionsByAlias: make(map[string]*mockedvm.MockedTransaction),
		outputIDsByAlias:    make(map[string]utxo.OutputID),
	}

	genesisOutput := mockedvm.NewMockedOutput(utxo.EmptyTransactionID, 0, 0)
	cachedObject, stored := t.Instance.Storage().OutputStorage().StoreIfAbsent(genesisOutput)
	if stored {
		cachedObject.Release()

		genesisOutputMetadata := NewOutputMetadata(genesisOutput.ID())
		genesisOutputMetadata.SetConfirmationState(confirmation.Confirmed)
		t.Instance.Storage().OutputMetadataStorage().Store(genesisOutputMetadata).Release()

		t.outputIDsByAlias["Genesis"] = genesisOutput.ID()
		t.ConflictDAG.RegisterConflictIDAlias("Genesis", utxo.EmptyTransactionID)
		t.ConflictDAG.RegisterConflictSetIDAlias("Genesis", genesisOutput.ID())
		genesisOutput.ID().RegisterAlias("Genesis")
	}
	return t
}

// Transaction gets the created MockedTransaction by the given alias.
// Panics if it doesn't exist.
func (t *TestFramework) Transaction(txAlias string) (tx *mockedvm.MockedTransaction) {
	t.transactionsByAliasMutex.RLock()
	defer t.transactionsByAliasMutex.RUnlock()

	tx, exists := t.transactionsByAlias[txAlias]
	if !exists {
		panic(fmt.Sprintf("tried to retrieve transaction with unknown alias: %s", txAlias))
	}

	return tx
}

// OutputID gets the created utxo.OutputID by the given alias.
// Panics if it doesn't exist.
func (t *TestFramework) OutputID(alias string) (outputID utxo.OutputID) {
	t.outputIDsByAliasMutex.RLock()
	defer t.outputIDsByAliasMutex.RUnlock()

	outputID, exists := t.outputIDsByAlias[alias]
	if !exists {
		panic(fmt.Sprintf("unknown output alias: %s", alias))
	}

	return outputID
}

// TransactionIDs gets all MockedTransaction given by txAliases.
// Panics if an alias doesn't exist.
func (t *TestFramework) TransactionIDs(txAliases ...string) (txIDs utxo.TransactionIDs) {
	txIDs = utxo.NewTransactionIDs()
	for _, expectedConflictAlias := range txAliases {
		txIDs.Add(t.Transaction(expectedConflictAlias).ID())
	}

	return txIDs
}

// ConflictIDs gets all conflictdag.ConflictIDs given by txAliases.
// Panics if an alias doesn't exist.
func (t *TestFramework) ConflictIDs(txAliases ...string) (conflictIDs *advancedset.AdvancedSet[utxo.TransactionID]) {
	return t.ConflictDAG.ConflictIDs(txAliases...)
}

// CreateTransaction creates a transaction with the given alias and outputCount. Inputs for the transaction are specified
// by their aliases where <txAlias.outputCount>. Panics if an input does not exist.
func (t *TestFramework) CreateTransaction(txAlias string, outputCount uint16, inputAliases ...string) (tx *mockedvm.MockedTransaction) {
	mockedInputs := make([]*mockedvm.MockedInput, 0)
	for _, inputAlias := range inputAliases {
		mockedInputs = append(mockedInputs, mockedvm.NewMockedInput(t.OutputID(inputAlias)))
	}

	t.transactionsByAliasMutex.Lock()
	defer t.transactionsByAliasMutex.Unlock()
	tx = mockedvm.NewMockedTransaction(mockedInputs, outputCount)
	tx.ID().RegisterAlias(txAlias)
	t.transactionsByAlias[txAlias] = tx
	t.ConflictDAG.RegisterConflictIDAlias(txAlias, tx.ID())

	t.outputIDsByAliasMutex.Lock()
	defer t.outputIDsByAliasMutex.Unlock()

	for i := uint16(0); i < outputCount; i++ {
		outputID := t.MockOutputFromTx(tx, i)
		outputAlias := txAlias + "." + strconv.Itoa(int(i))

		outputID.RegisterAlias(outputAlias)
		t.outputIDsByAlias[outputAlias] = outputID
		t.ConflictDAG.RegisterConflictSetIDAlias(outputAlias, outputID)
	}

	return tx
}

// IssueTransactions issues the transaction given by txAlias.
func (t *TestFramework) IssueTransactions(txAliases ...string) (err error) {
	for _, txAlias := range txAliases {
		if err = t.Instance.StoreAndProcessTransaction(context.Background(), t.Transaction(txAlias)); err != nil {
			return xerrors.Errorf("failed to issue transaction '%s': %w", txAlias, err)
		}
	}

	return nil
}

// MockOutputFromTx creates an utxo.OutputID from a given MockedTransaction and outputIndex.
func (t *TestFramework) MockOutputFromTx(tx *mockedvm.MockedTransaction, outputIndex uint16) (mockedOutputID utxo.OutputID) {
	return utxo.NewOutputID(tx.ID(), outputIndex)
}

// AssertConflictDAG asserts the structure of the conflict DAG as specified in expectedParents.
// "conflict3": {"conflict1","conflict2"} asserts that "conflict3" should have "conflict1" and "conflict2" as parents.
// It also verifies the reverse mapping, that there is a child reference (conflictdag.ChildConflict)
// from "conflict1"->"conflict3" and "conflict2"->"conflict3".
func (t *TestFramework) AssertConflictDAG(expectedParents map[string][]string) {
	t.ConflictDAG.AssertConflictParentsAndChildren(expectedParents)
}

// AssertConflicts asserts conflict membership from conflictID -> conflicts but also the reverse mapping conflict -> conflictIDs.
// expectedConflictAliases should be specified as
// "output.0": {"conflict1", "conflict2"}.
func (t *TestFramework) AssertConflicts(expectedConflictSetToConflictsAliases map[string][]string) {
	t.ConflictDAG.AssertConflictSetsAndConflicts(expectedConflictSetToConflictsAliases)
}

// AssertConflictIDs asserts that the given transactions and their outputs are booked into the specified conflicts.
func (t *TestFramework) AssertConflictIDs(expectedConflicts map[string][]string) {
	for txAlias, expectedConflictAliases := range expectedConflicts {
		currentTx := t.Transaction(txAlias)

		expectedConflictIDs := t.ConflictIDs(expectedConflictAliases...)

		t.ConsumeTransactionMetadata(currentTx.ID(), func(txMetadata *TransactionMetadata) {
			require.Truef(t.test, expectedConflictIDs.Equal(txMetadata.ConflictIDs()), "Transaction(%s): expected %s is not equal to actual %s", txAlias, expectedConflictIDs, txMetadata.ConflictIDs())
		})

		t.ConsumeTransactionOutputs(currentTx, func(outputMetadata *OutputMetadata) {
			require.Truef(t.test, expectedConflictIDs.Equal(outputMetadata.ConflictIDs()), "Output(%s): expected %s is not equal to actual %s", outputMetadata.ID(), expectedConflictIDs, outputMetadata.ConflictIDs())
		})
	}
}

// AssertBranchConfirmationState asserts the confirmation state of the given branch.
func (t *TestFramework) AssertBranchConfirmationState(txAlias string, validator func(state confirmation.State) bool) {
	require.True(t.test, validator(t.ConflictDAG.ConfirmationState(txAlias)))
}

// AssertTransactionConfirmationState asserts the confirmation state of the given transaction.
func (t *TestFramework) AssertTransactionConfirmationState(txAlias string, validator func(state confirmation.State) bool) {
	t.ConsumeTransactionMetadata(t.Transaction(txAlias).ID(), func(txMetadata *TransactionMetadata) {
		require.True(t.test, validator(txMetadata.ConfirmationState()))
	})
}

// AssertBooked asserts the booking status of all given transactions.
func (t *TestFramework) AssertBooked(expectedBookedMap map[string]bool) {
	for txAlias, expectedBooked := range expectedBookedMap {
		currentTx := t.Transaction(txAlias)
		t.ConsumeTransactionMetadata(currentTx.ID(), func(txMetadata *TransactionMetadata) {
			require.Equalf(t.test, expectedBooked, txMetadata.IsBooked(), "Transaction(%s): expected booked(%s) but has booked(%s)", txAlias, expectedBooked, txMetadata.IsBooked())

			_ = txMetadata.OutputIDs().ForEach(func(outputID utxo.OutputID) (err error) {
				// Check if output exists according to the Booked status of the enclosing Transaction.
				require.Equalf(t.test, expectedBooked, t.Instance.Storage().CachedOutputMetadata(outputID).Consume(func(_ *OutputMetadata) {}),
					"Output(%s): expected booked(%s) but has booked(%s)", outputID, expectedBooked, txMetadata.IsBooked())
				return nil
			})
		})
	}
}

// AllBooked returns whether all given transactions are booked.
func (t *TestFramework) AllBooked(txAliases ...string) (allBooked bool) {
	for _, txAlias := range txAliases {
		t.ConsumeTransactionMetadata(t.Transaction(txAlias).ID(), func(txMetadata *TransactionMetadata) {
			allBooked = txMetadata.IsBooked()
		})

		if !allBooked {
			return
		}
	}

	return
}

// ConsumeTransactionMetadata loads and consumes TransactionMetadata. Asserts that the loaded entity exists.
func (t *TestFramework) ConsumeTransactionMetadata(txID utxo.TransactionID, consumer func(txMetadata *TransactionMetadata)) {
	require.Truef(t.test, t.Instance.Storage().CachedTransactionMetadata(txID).Consume(consumer), "failed to load metadata of %s", txID)
}

// ConsumeOutputMetadata loads and consumes OutputMetadata. Asserts that the loaded entity exists.
func (t *TestFramework) ConsumeOutputMetadata(outputID utxo.OutputID, consumer func(outputMetadata *OutputMetadata)) {
	require.True(t.test, t.Instance.Storage().CachedOutputMetadata(outputID).Consume(consumer))
}

// ConsumeOutput loads and consumes Output. Asserts that the loaded entity exists.
func (t *TestFramework) ConsumeOutput(outputID utxo.OutputID, consumer func(output utxo.Output)) {
	require.True(t.test, t.Instance.Storage().CachedOutput(outputID).Consume(consumer))
}

// ConsumeTransactionOutputs loads and consumes all OutputMetadata of the given Transaction. Asserts that the loaded entities exists.
func (t *TestFramework) ConsumeTransactionOutputs(mockTx *mockedvm.MockedTransaction, consumer func(outputMetadata *OutputMetadata)) {
	t.ConsumeTransactionMetadata(mockTx.ID(), func(txMetadata *TransactionMetadata) {
		require.EqualValuesf(t.test, mockTx.M.OutputCount, txMetadata.OutputIDs().Size(), "Output count in %s do not match", mockTx.ID())

		for _, outputID := range txMetadata.OutputIDs().Slice() {
			t.ConsumeOutputMetadata(outputID, consumer)
		}
	})
}
