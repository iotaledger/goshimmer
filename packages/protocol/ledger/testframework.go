package ledger

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/model"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/serix"
	"github.com/iotaledger/hive.go/core/stringify"
	"github.com/iotaledger/hive.go/core/types/confirmation"
	"github.com/iotaledger/hive.go/core/workerpool"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/protocol/models/payload"
	"github.com/iotaledger/goshimmer/packages/protocol/models/payloadtype"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

// TestFramework provides common testing functionality for the ledger package. As such, it helps to easily build an
// UTXO-DAG by specifying transactions outputs/inputs via aliases.
// It makes use of a simplified MockedVM, with MockedTransaction, MockedOutput and MockedInput.
type TestFramework struct {
	// Instance contains a reference to the Ledger instance that the TestFramework is using.
	Instance *Ledger

	// test contains a reference to the testing instance.
	test *testing.T

	// transactionsByAlias contains a dictionary that maps a human-readable alias to a MockedTransaction.
	transactionsByAlias map[string]*MockedTransaction

	// transactionsByAliasMutex contains a mutex that is used to synchronize parallel access to the transactionsByAlias.
	transactionsByAliasMutex sync.RWMutex

	// outputIDsByAlias contains a dictionary that maps a human-readable alias to an OutputID.
	outputIDsByAlias map[string]utxo.OutputID

	// outputIDsByAliasMutex contains a mutex that is used to synchronize parallel access to the outputIDsByAlias.
	outputIDsByAliasMutex sync.RWMutex
}

func NewTestLedger(t *testing.T, workers *workerpool.Group, optsLedger ...options.Option[Ledger]) *Ledger {
	storage := blockdag.NewTestStorage(t, workers)
	ledger := New(workers.CreatePool("Ledger", 2), storage, optsLedger...)

	t.Cleanup(func() {
		workers.Wait()
		ledger.Shutdown()
		storage.Shutdown()
	})

	return ledger
}

// NewTestFramework creates a new instance of the TestFramework with one default output "Genesis" which has to be
// consumed by the first transaction.
func NewTestFramework(test *testing.T, instance *Ledger) *TestFramework {
	t := &TestFramework{
		test:                test,
		Instance:            instance,
		transactionsByAlias: make(map[string]*MockedTransaction),
		outputIDsByAlias:    make(map[string]utxo.OutputID),
	}

	genesisOutput := NewMockedOutput(utxo.EmptyTransactionID, 0, 0)
	cachedObject, stored := t.Instance.Storage.OutputStorage.StoreIfAbsent(genesisOutput)
	if stored {
		cachedObject.Release()

		genesisOutputMetadata := NewOutputMetadata(genesisOutput.ID())
		genesisOutputMetadata.SetConfirmationState(confirmation.Confirmed)
		t.Instance.Storage.OutputMetadataStorage.Store(genesisOutputMetadata).Release()

		t.outputIDsByAlias["Genesis"] = genesisOutput.ID()
		genesisOutput.ID().RegisterAlias("Genesis")
	}
	return t
}

func NewDefaultTestFramework(t *testing.T, workers *workerpool.Group, optsLedger ...options.Option[Ledger]) *TestFramework {
	return NewTestFramework(t, NewTestLedger(t, workers.CreateGroup("Ledger"), optsLedger...))
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
	return t.ConflictDAGTestFramework.ConflictIDs(txAliases...)
}

// CreateTransaction creates a transaction with the given alias and outputCount. Inputs for the transaction are specified
// by their aliases where <txAlias.outputCount>. Panics if an input does not exist.
func (t *TestFramework) CreateTransaction(txAlias string, outputCount uint16, inputAliases ...string) (tx *MockedTransaction) {
	mockedInputs := make([]*MockedInput, 0)
	for _, inputAlias := range inputAliases {
		mockedInputs = append(mockedInputs, NewMockedInput(t.OutputID(inputAlias)))
	}

	t.transactionsByAliasMutex.Lock()
	defer t.transactionsByAliasMutex.Unlock()
	tx = NewMockedTransaction(mockedInputs, outputCount)
	tx.ID().RegisterAlias(txAlias)
	t.transactionsByAlias[txAlias] = tx
	t.ConflictDAGTestFramework.RegisterConflictIDAlias(txAlias, tx.ID())

	t.outputIDsByAliasMutex.Lock()
	defer t.outputIDsByAliasMutex.Unlock()

	for i := uint16(0); i < outputCount; i++ {
		outputID := t.MockOutputFromTx(tx, i)
		outputAlias := txAlias + "." + strconv.Itoa(int(i))

		outputID.RegisterAlias(outputAlias)
		t.outputIDsByAlias[outputAlias] = outputID
		t.ConflictDAGTestFramework.RegisterConflictSetIDAlias(outputAlias, outputID)
	}

	return tx
}

// IssueTransaction issues the transaction given by txAlias.
func (t *TestFramework) IssueTransaction(txAlias string) (err error) {
	return t.Instance.StoreAndProcessTransaction(context.Background(), t.Transaction(txAlias))
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
	t.ConflictDAGTestFramework.AssertConflictParentsAndChildren(expectedParents)
}

// AssertConflicts asserts conflict membership from conflictID -> conflicts but also the reverse mapping conflict -> conflictIDs.
// expectedConflictAliases should be specified as
// "output.0": {"conflict1", "conflict2"}.
func (t *TestFramework) AssertConflicts(expectedConflictSetToConflictsAliases map[string][]string) {
	t.ConflictDAGTestFramework.AssertConflictSetsAndConflicts(expectedConflictSetToConflictsAliases)
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

// AssertBooked asserts the booking status of all given transactions.
func (t *TestFramework) AssertBooked(expectedBookedMap map[string]bool) {
	for txAlias, expectedBooked := range expectedBookedMap {
		currentTx := t.Transaction(txAlias)
		t.ConsumeTransactionMetadata(currentTx.ID(), func(txMetadata *TransactionMetadata) {
			require.Equalf(t.test, expectedBooked, txMetadata.IsBooked(), "Transaction(%s): expected booked(%s) but has booked(%s)", txAlias, expectedBooked, txMetadata.IsBooked())

			_ = txMetadata.OutputIDs().ForEach(func(outputID utxo.OutputID) (err error) {
				// Check if output exists according to the Booked status of the enclosing Transaction.
				require.Equalf(t.test, expectedBooked, t.Instance.Storage.CachedOutputMetadata(outputID).Consume(func(_ *OutputMetadata) {}),
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
	require.Truef(t.test, t.Instance.Storage.CachedTransactionMetadata(txID).Consume(consumer), "failed to load metadata of %s", txID)
}

// ConsumeOutputMetadata loads and consumes OutputMetadata. Asserts that the loaded entity exists.
func (t *TestFramework) ConsumeOutputMetadata(outputID utxo.OutputID, consumer func(outputMetadata *OutputMetadata)) {
	require.True(t.test, t.Instance.Storage.CachedOutputMetadata(outputID).Consume(consumer))
}

// ConsumeOutput loads and consumes Output. Asserts that the loaded entity exists.
func (t *TestFramework) ConsumeOutput(outputID utxo.OutputID, consumer func(output utxo.Output)) {
	require.True(t.test, t.Instance.Storage.CachedOutput(outputID).Consume(consumer))
}

// ConsumeTransactionOutputs loads and consumes all OutputMetadata of the given Transaction. Asserts that the loaded entities exists.
func (t *TestFramework) ConsumeTransactionOutputs(mockTx *MockedTransaction, consumer func(outputMetadata *OutputMetadata)) {
	t.ConsumeTransactionMetadata(mockTx.ID(), func(txMetadata *TransactionMetadata) {
		require.EqualValuesf(t.test, mockTx.M.OutputCount, txMetadata.OutputIDs().Size(), "Output count in %s do not match", mockTx.ID())

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
func NewMockedInput(outputID utxo.OutputID) *MockedInput {
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

	Balance uint64 `serix:"2"`
}

// NewMockedOutput creates a new MockedOutput based on the utxo.TransactionID and its index within the MockedTransaction.
func NewMockedOutput(txID utxo.TransactionID, index uint16, balance uint64) (out *MockedOutput) {
	out = model.NewStorable[utxo.OutputID, MockedOutput](&mockedOutput{
		TxID:    txID,
		Index:   index,
		Balance: balance,
	})
	out.SetID(utxo.OutputID{TransactionID: txID, Index: index})
	return out
}

// code contract (make sure the struct implements all required methods).
var _ utxo.Output = new(MockedOutput)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MockedTransaction ////////////////////////////////////////////////////////////////////////////////////////////

// MockedTransactionType represents the payload Type of mocked Transactions.
var MockedTransactionType payload.Type

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

	tx.SetID(utxo.NewTransactionID(lo.PanicOnErr(tx.Bytes())))

	return tx
}

// Inputs returns the inputs of the Transaction.
func (m *MockedTransaction) Inputs() (inputs []utxo.Input) {
	return lo.Map(m.M.Inputs, (*MockedInput).utxoInput)
}

// Type returns the type of the Transaction.
func (m *MockedTransaction) Type() payload.Type {
	return MockedTransactionType
}

// code contract (make sure the struct implements all required methods).
var (
	_ utxo.Transaction = new(MockedTransaction)
	_ payload.Payload  = new(MockedTransaction)
)

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
func (m *MockedVM) ExecuteTransaction(transaction utxo.Transaction, inputs *utxo.Outputs, _ ...uint64) (outputs []utxo.Output, err error) {
	mockedTransaction := transaction.(*MockedTransaction)

	outputs = make([]utxo.Output, mockedTransaction.M.OutputCount)
	for i := uint16(0); i < mockedTransaction.M.OutputCount; i++ {
		outputs[i] = NewMockedOutput(mockedTransaction.ID(), i, uint64(i))
		outputs[i].SetID(utxo.NewOutputID(mockedTransaction.ID(), i))
	}

	return
}

// code contract (make sure the struct implements all required methods).
var _ vm.VM = new(MockedVM)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

func init() {
	MockedTransactionType = payload.NewType(payloadtype.MockedTransaction, "MockedTransactionType")

	if err := serix.DefaultAPI.RegisterTypeSettings(MockedTransaction{}, serix.TypeSettings{}.WithObjectType(uint32(new(MockedTransaction).Type()))); err != nil {
		panic(errors.Wrap(err, "error registering Transaction type settings"))
	}

	if err := serix.DefaultAPI.RegisterInterfaceObjects((*payload.Payload)(nil), new(MockedTransaction)); err != nil {
		panic(errors.Wrap(err, "error registering Transaction as Payload interface"))
	}

	if err := serix.DefaultAPI.RegisterTypeSettings(MockedOutput{}, serix.TypeSettings{}.WithObjectType(uint8(devnetvm.ExtendedLockedOutputType+1))); err != nil {
		panic(errors.Wrap(err, "error registering ExtendedLockedOutput type settings"))
	}

	if err := serix.DefaultAPI.RegisterInterfaceObjects((*utxo.Output)(nil), new(MockedOutput)); err != nil {
		panic(errors.Wrap(err, "error registering utxo.Output interface implementations"))
	}
}
