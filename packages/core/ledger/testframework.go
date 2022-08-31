package ledger

import (
	"context"
	"encoding/binary"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/iotaledger/hive.go/core/types/confirmation"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/model"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/serix"
	"github.com/iotaledger/hive.go/core/stringify"
	"github.com/iotaledger/hive.go/core/types"

	"github.com/iotaledger/goshimmer/packages/core/conflictdag"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/ledger/vm"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

// TestFramework provides common testing functionality for the ledger package. As such, it helps to easily build an
// UTXO-DAG by specifying transactions outputs/inputs via aliases.
// It makes use of a simplified MockedVM, with MockedTransaction, MockedOutput and MockedInput.
type TestFramework struct {
	// t contains a reference to the testing instance.
	t *testing.T

	// ledger contains a reference to the Ledger instance that the TestFramework is using.
	ledger *Ledger

	// transactionsByAlias contains a dictionary that maps a human-readable alias to a MockedTransaction.
	transactionsByAlias map[string]*MockedTransaction

	// transactionsByAliasMutex contains a mutex that is used to synchronize parallel access to the transactionsByAlias.
	transactionsByAliasMutex sync.RWMutex

	// outputIDsByAlias contains a dictionary that maps a human-readable alias to an OutputID.
	outputIDsByAlias map[string]utxo.OutputID

	// outputIDsByAliasMutex contains a mutex that is used to synchronize parallel access to the outputIDsByAlias.
	outputIDsByAliasMutex sync.RWMutex
}

// NewTestFramework creates a new instance of the TestFramework with one default output "Genesis" which has to be
// consumed by the first transaction.
func NewTestFramework(t *testing.T, options ...Option) (new *TestFramework) {
	new = &TestFramework{
		t:                   t,
		ledger:              New(options...),
		transactionsByAlias: make(map[string]*MockedTransaction),
		outputIDsByAlias:    make(map[string]utxo.OutputID),
	}

	genesisOutput := NewMockedOutput(utxo.EmptyTransactionID, 0)
	genesisOutputMetadata := NewOutputMetadata(genesisOutput.ID())
	genesisOutputMetadata.SetConfirmationState(confirmation.Confirmed)

	genesisOutput.ID().RegisterAlias("Genesis")
	new.outputIDsByAlias["Genesis"] = genesisOutput.ID()

	new.ledger.Storage.outputStorage.Store(genesisOutput).Release()
	new.ledger.Storage.outputMetadataStorage.Store(genesisOutputMetadata).Release()

	return new
}

// Transaction gets the created MockedTransaction by the given alias.
// Panics if it doesn't exist.
func (t *TestFramework) Transaction(txAlias string) (tx *MockedTransaction) {
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
func (t *TestFramework) ConflictIDs(txAliases ...string) (conflictIDs *set.AdvancedSet[utxo.TransactionID]) {
	conflictIDs = set.NewAdvancedSet[utxo.TransactionID]()
	for _, expectedConflictAlias := range txAliases {
		if expectedConflictAlias == "MasterConflict" {
			conflictIDs.Add(utxo.TransactionID{})
			continue
		}

		conflictIDs.Add(t.Transaction(expectedConflictAlias).ID())
	}

	return conflictIDs
}

// CreateTransaction creates a transaction with the given alias and outputCount. Inputs for the transaction are specified
// by their aliases where <txAlias.outputCount>. Panics if an input does not exist.
func (t *TestFramework) CreateTransaction(txAlias string, outputCount uint16, inputAliases ...string) {
	mockedInputs := make([]*MockedInput, 0)
	for _, inputAlias := range inputAliases {
		mockedInputs = append(mockedInputs, NewMockedInput(t.OutputID(inputAlias)))
	}

	t.transactionsByAliasMutex.Lock()
	defer t.transactionsByAliasMutex.Unlock()
	tx := NewMockedTransaction(mockedInputs, outputCount)
	tx.ID().RegisterAlias(txAlias)
	t.transactionsByAlias[txAlias] = tx

	t.outputIDsByAliasMutex.Lock()
	defer t.outputIDsByAliasMutex.Unlock()

	for i := uint16(0); i < outputCount; i++ {
		outputID := t.MockOutputFromTx(tx, i)
		outputAlias := txAlias + "." + strconv.Itoa(int(i))

		outputID.RegisterAlias(outputAlias)
		t.outputIDsByAlias[outputAlias] = outputID
	}
}

// IssueTransaction issues the transaction given by txAlias.
func (t *TestFramework) IssueTransaction(txAlias string) (err error) {
	return t.ledger.StoreAndProcessTransaction(context.Background(), t.Transaction(txAlias))
}

func (t *TestFramework) WaitUntilAllTasksProcessed() (self *TestFramework) {
	// time.Sleep(100 * time.Millisecond)
	event.Loop.WaitUntilAllTasksProcessed()
	return t
}

// MockOutputFromTx creates an utxo.OutputID from a given MockedTransaction and outputIndex.
func (t *TestFramework) MockOutputFromTx(tx *MockedTransaction, outputIndex uint16) (mockedOutputID utxo.OutputID) {
	return utxo.NewOutputID(tx.ID(), outputIndex)
}

// AssertConflictDAG asserts the structure of the conflict DAG as specified in expectedParents.
// "conflict3": {"conflict1","conflict2"} asserts that "conflict3" should have "conflict1" and "conflict2" as parents.
// It also verifies the reverse mapping, that there is a child reference (conflictdag.ChildConflict)
// from "conflict1"->"conflict3" and "conflict2"->"conflict3".
func (t *TestFramework) AssertConflictDAG(expectedParents map[string][]string) {
	// Parent -> child references.
	childConflicts := make(map[utxo.TransactionID]*set.AdvancedSet[utxo.TransactionID])

	for conflictAlias, expectedParentAliases := range expectedParents {
		currentConflictID := t.Transaction(conflictAlias).ID()
		expectedConflictIDs := t.ConflictIDs(expectedParentAliases...)

		// Verify child -> parent references.
		t.ConsumeConflict(currentConflictID, func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
			assert.Truef(t.t, expectedConflictIDs.Equal(conflict.Parents()), "Conflict(%s): expected parents %s are not equal to actual parents %s", currentConflictID, expectedConflictIDs, conflict.Parents())
		})

		for _, parentConflictID := range expectedConflictIDs.Slice() {
			if _, exists := childConflicts[parentConflictID]; !exists {
				childConflicts[parentConflictID] = set.NewAdvancedSet[utxo.TransactionID]()
			}
			childConflicts[parentConflictID].Add(currentConflictID)
		}
	}

	// Verify parent -> child references.
	for parentConflictID, childConflictIDs := range childConflicts {
		cachedChildConflicts := t.ledger.ConflictDAG.Storage.CachedChildConflicts(parentConflictID)
		assert.Equalf(t.t, childConflictIDs.Size(), len(cachedChildConflicts), "child conflicts count does not match for parent conflict %s, expected=%s, actual=%s", parentConflictID, childConflictIDs, cachedChildConflicts.Unwrap())
		cachedChildConflicts.Release()

		for _, childConflictID := range childConflictIDs.Slice() {
			assert.Truef(t.t, t.ledger.ConflictDAG.Storage.CachedChildConflict(parentConflictID, childConflictID).Consume(func(childConflict *conflictdag.ChildConflict[utxo.TransactionID]) {}), "could not load ChildConflict %s,%s", parentConflictID, childConflictID)
		}
	}
}

// AssertConflicts asserts conflict membership from conflictID -> conflicts but also the reverse mapping conflict -> conflictIDs.
// expectedConflictAliases should be specified as
// "output.0": {"conflict1", "conflict2"}
func (t *TestFramework) AssertConflicts(expectedConflictsAliases map[string][]string) {
	// Conflict -> conflictIDs.
	ConflictResources := make(map[utxo.TransactionID]*set.AdvancedSet[utxo.OutputID])

	for resourceAlias, expectedConflictMembersAliases := range expectedConflictsAliases {
		resourceID := t.OutputID(resourceAlias)
		expectedConflictMembers := t.ConflictIDs(expectedConflictMembersAliases...)

		// Check count of conflict members for this conflictID.
		cachedConflictMembers := t.ledger.ConflictDAG.Storage.CachedConflictMembers(resourceID)
		assert.Equalf(t.t, expectedConflictMembers.Size(), len(cachedConflictMembers), "conflict member count does not match for conflict %s, expected=%s, actual=%s", resourceID, expectedConflictsAliases, cachedConflictMembers.Unwrap())
		cachedConflictMembers.Release()

		// Verify that all named conflicts are stored as conflict members (conflictID -> conflictIDs).
		for _, conflictID := range expectedConflictMembers.Slice() {
			assert.Truef(t.t, t.ledger.ConflictDAG.Storage.CachedConflictMember(resourceID, conflictID).Consume(func(conflictMember *conflictdag.ConflictMember[utxo.OutputID, utxo.TransactionID]) {}), "could not load ConflictMember %s,%s", resourceID, conflictID)

			if _, exists := ConflictResources[conflictID]; !exists {
				ConflictResources[conflictID] = set.NewAdvancedSet[utxo.OutputID]()
			}
			ConflictResources[conflictID].Add(resourceID)
		}
	}

	// Make sure that all conflicts have all specified conflictIDs (reverse mapping).
	for conflictID, expectedConflicts := range ConflictResources {
		t.ConsumeConflict(conflictID, func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
			assert.Truef(t.t, expectedConflicts.Equal(conflict.ConflictSetIDs()), "%s: conflicts expected=%s, actual=%s", conflictID, expectedConflicts, conflict.ConflictSetIDs())
		})
	}
}

// AssertConflictIDs asserts that the given transactions and their outputs are booked into the specified conflicts.
func (t *TestFramework) AssertConflictIDs(expectedConflicts map[string][]string) {
	for txAlias, expectedConflictAliases := range expectedConflicts {
		currentTx := t.Transaction(txAlias)

		expectedConflictIDs := t.ConflictIDs(expectedConflictAliases...)

		t.ConsumeTransactionMetadata(currentTx.ID(), func(txMetadata *TransactionMetadata) {
			assert.Truef(t.t, expectedConflictIDs.Equal(txMetadata.ConflictIDs()), "Transaction(%s): expected %s is not equal to actual %s", txAlias, expectedConflictIDs, txMetadata.ConflictIDs())
		})

		t.ConsumeTransactionOutputs(currentTx, func(outputMetadata *OutputMetadata) {
			assert.Truef(t.t, expectedConflictIDs.Equal(outputMetadata.ConflictIDs()), "Output(%s): expected %s is not equal to actual %s", outputMetadata.ID(), expectedConflictIDs, outputMetadata.ConflictIDs())
		})
	}
}

// AssertBooked asserts the booking status of all given transactions.
func (t *TestFramework) AssertBooked(expectedBookedMap map[string]bool) {
	for txAlias, expectedBooked := range expectedBookedMap {
		currentTx := t.Transaction(txAlias)
		t.ConsumeTransactionMetadata(currentTx.ID(), func(txMetadata *TransactionMetadata) {
			assert.Equalf(t.t, expectedBooked, txMetadata.IsBooked(), "Transaction(%s): expected booked(%s) but has booked(%s)", txAlias, expectedBooked, txMetadata.IsBooked())

			_ = txMetadata.OutputIDs().ForEach(func(outputID utxo.OutputID) (err error) {
				// Check if output exists according to the Booked status of the enclosing Transaction.
				assert.Equalf(t.t, expectedBooked, t.ledger.Storage.CachedOutputMetadata(outputID).Consume(func(_ *OutputMetadata) {}),
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

// ConsumeConflict loads and consumes conflictdag.Conflict. Asserts that the loaded entity exists.
func (t *TestFramework) ConsumeConflict(conflictID utxo.TransactionID, consumer func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID])) {
	assert.Truef(t.t, t.ledger.ConflictDAG.Storage.CachedConflict(conflictID).Consume(consumer), "failed to load conflict %s", conflictID)
}

// ConsumeTransactionMetadata loads and consumes TransactionMetadata. Asserts that the loaded entity exists.
func (t *TestFramework) ConsumeTransactionMetadata(txID utxo.TransactionID, consumer func(txMetadata *TransactionMetadata)) {
	assert.Truef(t.t, t.ledger.Storage.CachedTransactionMetadata(txID).Consume(consumer), "failed to load metadata of %s", txID)
}

// ConsumeOutputMetadata loads and consumes OutputMetadata. Asserts that the loaded entity exists.
func (t *TestFramework) ConsumeOutputMetadata(outputID utxo.OutputID, consumer func(outputMetadata *OutputMetadata)) {
	assert.True(t.t, t.ledger.Storage.CachedOutputMetadata(outputID).Consume(consumer))
}

// ConsumeOutput loads and consumes Output. Asserts that the loaded entity exists.
func (t *TestFramework) ConsumeOutput(outputID utxo.OutputID, consumer func(output utxo.Output)) {
	assert.True(t.t, t.ledger.Storage.CachedOutput(outputID).Consume(consumer))
}

// ConsumeTransactionOutputs loads and consumes all OutputMetadata of the given Transaction. Asserts that the loaded entities exists.
func (t *TestFramework) ConsumeTransactionOutputs(mockTx *MockedTransaction, consumer func(outputMetadata *OutputMetadata)) {
	t.ConsumeTransactionMetadata(mockTx.ID(), func(txMetadata *TransactionMetadata) {
		assert.EqualValuesf(t.t, mockTx.M.OutputCount, txMetadata.OutputIDs().Size(), "Output count in %s do not match", mockTx.ID())

		for _, outputID := range txMetadata.OutputIDs().Slice() {
			t.ConsumeOutputMetadata(outputID, consumer)
		}
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MockedInput //////////////////////////////////////////////////////////////////////////////////////////////////

// MockedInput is a mocked entity that allows to "address" which Outputs are supposed to be used by a Transaction.
type MockedInput struct {
	// outputID contains the referenced OutputID.
	OutputID utxo.OutputID `serix:"0"`
}

// NewMockedInput creates a new MockedInput from an utxo.OutputID.
func NewMockedInput(outputID utxo.OutputID) (new *MockedInput) {
	return &MockedInput{OutputID: outputID}
}

// String returns a human-readable version of the MockedInput.
func (m *MockedInput) String() (humanReadable string) {
	return stringify.Struct("MockedInput",
		stringify.NewStructField("OutputID", m.OutputID),
	)
}

// utxoInput type-casts the MockedInput to a utxo.Input.
func (m *MockedInput) utxoInput() (input utxo.Input) {
	return m
}

// code contract (make sure the struct implements all required methods).
var _ utxo.Input = new(MockedInput)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MockedOutput /////////////////////////////////////////////////////////////////////////////////////////////////

// MockedOutput is the container for the data produced by executing a MockedTransaction.
type MockedOutput struct {
	model.Storable[utxo.OutputID, MockedOutput, *MockedOutput, mockedOutput] `serix:"0"`
}

type mockedOutput struct {
	// TxID contains the identifier of the Transaction that created this MockedOutput.
	TxID utxo.TransactionID `serix:"0"`

	// Index contains the Index of the Output in respect to it's creating Transaction (the nth Output will have the
	// Index n).
	Index uint16 `serix:"1"`
}

// NewMockedOutput creates a new MockedOutput based on the utxo.TransactionID and its index within the MockedTransaction.
func NewMockedOutput(txID utxo.TransactionID, index uint16) (out *MockedOutput) {
	out = model.NewStorable[utxo.OutputID, MockedOutput](&mockedOutput{
		TxID:  txID,
		Index: index,
	})
	out.SetID(utxo.OutputID{TransactionID: txID, Index: index})
	return out
}

// code contract (make sure the struct implements all required methods).
var _ utxo.Output = new(MockedOutput)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MockedTransaction ////////////////////////////////////////////////////////////////////////////////////////////

// MockedTransaction is the type that is used to describe instructions how to modify the ledger state for MockedVM.
type MockedTransaction struct {
	model.Storable[utxo.TransactionID, MockedTransaction, *MockedTransaction, mockedTransaction] `serix:"0"`
}

type mockedTransaction struct {
	// Inputs contains the list of MockedInput objects that address the consumed Outputs.
	Inputs []*MockedInput `serix:"0,lengthPrefixType=uint16"`

	// OutputCount contains the number of Outputs that this MockedTransaction creates.
	OutputCount uint16 `serix:"1"`

	// UniqueEssence contains a unique value for each created MockedTransaction to ensure a unique TransactionID.
	UniqueEssence uint64 `serix:"2"`
}

// NewMockedTransaction creates a new MockedTransaction with the given inputs and specified outputCount.
// A unique essence is simulated by an atomic counter, incremented globally for each MockedTransaction created.
func NewMockedTransaction(inputs []*MockedInput, outputCount uint16) (tx *MockedTransaction) {
	tx = model.NewStorable[utxo.TransactionID, MockedTransaction](&mockedTransaction{
		Inputs:        inputs,
		OutputCount:   outputCount,
		UniqueEssence: atomic.AddUint64(&_uniqueEssenceCounter, 1),
	})

	b := types.Identifier{}
	binary.BigEndian.PutUint64(b[:], tx.M.UniqueEssence)
	tx.SetID(utxo.TransactionID{Identifier: b})

	return tx
}

// Inputs returns the inputs of the Transaction.
func (m *MockedTransaction) Inputs() (inputs []utxo.Input) {
	return lo.Map(m.M.Inputs, (*MockedInput).utxoInput)
}

// code contract (make sure the struct implements all required methods).
var _ utxo.Transaction = new(MockedTransaction)

// _uniqueEssenceCounter contains a counter that is used to generate unique TransactionIDs.
var _uniqueEssenceCounter uint64

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MockedVM /////////////////////////////////////////////////////////////////////////////////////////////////////

// MockedVM is an implementation of UTXO-based VMs for testing purposes.
type MockedVM struct{}

// NewMockedVM creates a new MockedVM.
func NewMockedVM() *MockedVM {
	return new(MockedVM)
}

// ParseTransaction un-serializes a Transaction from the given sequence of bytes.
func (m *MockedVM) ParseTransaction(transactionBytes []byte) (transaction utxo.Transaction, err error) {
	mockedTx := new(MockedTransaction)
	if _, err = serix.DefaultAPI.Decode(context.Background(), transactionBytes, mockedTx, serix.WithValidation()); err != nil {
		return nil, err
	}

	return mockedTx, nil
}

// ParseOutput un-serializes an Output from the given sequence of bytes.
func (m *MockedVM) ParseOutput(outputBytes []byte) (output utxo.Output, err error) {
	newOutput := new(MockedOutput)
	if _, err = serix.DefaultAPI.Decode(context.Background(), outputBytes, newOutput, serix.WithValidation()); err != nil {
		return nil, err
	}

	return newOutput, nil
}

// ResolveInput translates the Input into an OutputID.
func (m *MockedVM) ResolveInput(input utxo.Input) (outputID utxo.OutputID) {
	return input.(*MockedInput).OutputID
}

// ExecuteTransaction executes the Transaction and determines the Outputs from the given Inputs. It returns an error
// if the execution fails.
func (m *MockedVM) ExecuteTransaction(transaction utxo.Transaction, _ *utxo.Outputs, _ ...uint64) (outputs []utxo.Output, err error) {
	mockedTransaction := transaction.(*MockedTransaction)

	outputs = make([]utxo.Output, mockedTransaction.M.OutputCount)
	for i := uint16(0); i < mockedTransaction.M.OutputCount; i++ {
		outputs[i] = NewMockedOutput(mockedTransaction.ID(), i)
		outputs[i].SetID(utxo.NewOutputID(mockedTransaction.ID(), i))
	}

	return
}

// code contract (make sure the struct implements all required methods).
var _ vm.VM = new(MockedVM)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
