package ledger

import (
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/iotaledger/hive.go/generics/lo"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/ledger/branchdag"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

// TestFramework provides common testing functionality for the ledger package. As such, it helps to easily build an
// UTXO-DAG by specifying transactions outputs/inputs via aliases.
// It makes use of a simplified MockedVM, with MockedTransaction, MockedOutput and MockedInput.
type TestFramework struct {
	t                   *testing.T
	ledger              *Ledger
	transactionsByAlias map[string]*MockedTransaction
	outputIDsByAlias    map[string]utxo.OutputID

	transactionsByAliasMutex sync.RWMutex
	outputIDsByAliasMutex    sync.RWMutex
}

// NewTestFramework creates a new instance of the TestFramework with one default output "Genesis" which has to be
// consumed by the first transaction.
func NewTestFramework(t *testing.T, options ...Option) (new *TestFramework) {
	new = &TestFramework{
		ledger: New(options...),

		t:                   t,
		transactionsByAlias: make(map[string]*MockedTransaction),
		outputIDsByAlias:    make(map[string]utxo.OutputID),
	}

	genesisOutput := NewOutput(NewMockedOutput(utxo.EmptyTransactionID, 0))
	genesisOutputMetadata := NewOutputMetadata(genesisOutput.ID())
	genesisOutputMetadata.SetGradeOfFinality(gof.High)

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
	for _, expectedBranchAlias := range txAliases {
		txIDs.Add(t.Transaction(expectedBranchAlias).ID())
	}

	return txIDs
}

// BranchIDs gets all branchdag.BranchIDs given by txAliases.
// Panics if an alias doesn't exist.
func (t *TestFramework) BranchIDs(txAliases ...string) (branchIDs branchdag.BranchIDs) {
	branchIDs = branchdag.NewBranchIDs()
	for _, expectedBranchAlias := range txAliases {
		if expectedBranchAlias == "MasterBranch" {
			branchIDs.Add(branchdag.MasterBranchID)
			continue
		}

		branchIDs.Add(branchdag.NewBranchID(t.Transaction(expectedBranchAlias).ID()))
	}

	return branchIDs
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
	return t.ledger.StoreAndProcessTransaction(t.Transaction(txAlias))
}

// MockOutputFromTx creates an utxo.OutputID from a given MockedTransaction and outputIndex.
func (t *TestFramework) MockOutputFromTx(tx *MockedTransaction, outputIndex uint16) (mockedOutputID utxo.OutputID) {
	return utxo.NewOutputID(tx.ID(), outputIndex, []byte(""))
}

// AssertBranchDAG asserts the structure of the branch DAG as specified in expectedParents.
// "branch3": {"branch1","branch2"} asserts that "branch3" should have "branch1" and "branch2" as parents.
// It also verifies the reverse mapping, that there is a child reference (branchdag.ChildBranch)
// from "branch1"->"branch3" and "branch2"->"branch3".
func (t *TestFramework) AssertBranchDAG(expectedParents map[string][]string) {
	// Parent -> child references.
	childBranches := make(map[branchdag.BranchID]branchdag.BranchIDs)

	for branchAlias, expectedParentAliases := range expectedParents {
		currentBranchID := branchdag.NewBranchID(t.Transaction(branchAlias).ID())
		expectedBranchIDs := t.BranchIDs(expectedParentAliases...)

		// Verify child -> parent references.
		t.ConsumeBranch(currentBranchID, func(branch *branchdag.Branch) {
			assert.Truef(t.t, expectedBranchIDs.Equal(branch.Parents()), "Branch(%s): expected parents %s are not equal to actual parents %s", currentBranchID, expectedBranchIDs, branch.Parents())
		})

		for _, parentBranchID := range expectedBranchIDs.Slice() {
			if _, exists := childBranches[parentBranchID]; !exists {
				childBranches[parentBranchID] = branchdag.NewBranchIDs()
			}
			childBranches[parentBranchID].Add(currentBranchID)
		}
	}

	// Verify parent -> child references.
	for parentBranchID, childBranchIDs := range childBranches {
		cachedChildBranches := t.ledger.BranchDAG.Storage.CachedChildBranches(parentBranchID)
		assert.Equalf(t.t, childBranchIDs.Size(), len(cachedChildBranches), "child branches count does not match for parent branch %s, expected=%s, actual=%s", parentBranchID, childBranchIDs, cachedChildBranches.Unwrap())
		cachedChildBranches.Release()

		for _, childBranchID := range childBranchIDs.Slice() {
			assert.Truef(t.t, t.ledger.BranchDAG.Storage.CachedChildBranch(parentBranchID, childBranchID).Consume(func(childBranch *branchdag.ChildBranch) {}), "could not load ChildBranch %s,%s", parentBranchID, childBranchID)
		}
	}
}

// AssertConflicts asserts conflict membership from conflictID -> branches but also the reverse mapping branch -> conflictIDs.
// expectedConflictAliases should be specified as
// "output.0": {"branch1", "branch2"}
func (t *TestFramework) AssertConflicts(expectedConflictsAliases map[string][]string) {
	// Branch -> conflictIDs.
	branchConflicts := make(map[branchdag.BranchID]branchdag.ConflictIDs)

	for outputAlias, expectedConflictMembersAliases := range expectedConflictsAliases {
		conflictID := branchdag.NewConflictID(t.OutputID(outputAlias))
		expectedConflictMembers := t.BranchIDs(expectedConflictMembersAliases...)

		// Check count of conflict members for this conflictID.
		cachedConflictMembers := t.ledger.BranchDAG.Storage.CachedConflictMembers(conflictID)
		assert.Equalf(t.t, expectedConflictMembers.Size(), len(cachedConflictMembers), "conflict member count does not match for conflict %s, expected=%s, actual=%s", conflictID, expectedConflictsAliases, cachedConflictMembers.Unwrap())
		cachedConflictMembers.Release()

		// Verify that all named branches are stored as conflict members (conflictID -> branchIDs).
		for _, branchID := range expectedConflictMembers.Slice() {
			assert.Truef(t.t, t.ledger.BranchDAG.Storage.CachedConflictMember(conflictID, branchID).Consume(func(conflictMember *branchdag.ConflictMember) {}), "could not load ConflictMember %s,%s", conflictID, branchID)

			if _, exists := branchConflicts[branchID]; !exists {
				branchConflicts[branchID] = branchdag.NewConflictIDs()
			}
			branchConflicts[branchID].Add(conflictID)
		}
	}

	// Make sure that all branches have all specified conflictIDs (reverse mapping).
	for branchID, expectedConflicts := range branchConflicts {
		t.ConsumeBranch(branchID, func(branch *branchdag.Branch) {
			assert.Truef(t.t, expectedConflicts.Equal(branch.ConflictIDs()), "%s: conflicts expected=%s, actual=%s", branchID, expectedConflicts, branch.ConflictIDs())
		})
	}
}

// AssertBranchIDs asserts that the given transactions and their outputs are booked into the specified branches.
func (t *TestFramework) AssertBranchIDs(expectedBranches map[string][]string) {
	for txAlias, expectedBranchAliases := range expectedBranches {
		currentTx := t.Transaction(txAlias)

		expectedBranchIDs := t.BranchIDs(expectedBranchAliases...)

		t.ConsumeTransactionMetadata(currentTx.ID(), func(txMetadata *TransactionMetadata) {
			assert.Truef(t.t, expectedBranchIDs.Equal(txMetadata.BranchIDs()), "Transaction(%s): expected %s is not equal to actual %s", txAlias, expectedBranchIDs, txMetadata.BranchIDs())
		})

		t.ConsumeTransactionOutputs(currentTx, func(outputMetadata *OutputMetadata) {
			assert.Truef(t.t, expectedBranchIDs.Equal(outputMetadata.BranchIDs()), "Output(%s): expected %s is not equal to actual %s", outputMetadata.ID(), expectedBranchIDs, outputMetadata.BranchIDs())
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

// ConsumeBranch loads and consumes branchdag.Branch. Asserts that the loaded entity exists.
func (t *TestFramework) ConsumeBranch(branchID branchdag.BranchID, consumer func(branch *branchdag.Branch)) {
	assert.Truef(t.t, t.ledger.BranchDAG.Storage.CachedBranch(branchID).Consume(consumer), "failed to load branch %s", branchID)
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
func (t *TestFramework) ConsumeOutput(outputID utxo.OutputID, consumer func(output *Output)) {
	assert.True(t.t, t.ledger.Storage.CachedOutput(outputID).Consume(consumer))
}

// ConsumeTransactionOutputs loads and consumes all OutputMetadata of the given Transaction. Asserts that the loaded entities exists.
func (t *TestFramework) ConsumeTransactionOutputs(mockTx *MockedTransaction, consumer func(outputMetadata *OutputMetadata)) {
	t.ConsumeTransactionMetadata(mockTx.ID(), func(txMetadata *TransactionMetadata) {
		assert.EqualValuesf(t.t, mockTx.outputCount, txMetadata.OutputIDs().Size(), "Output count in %s do not match", mockTx.ID())

		for _, outputID := range txMetadata.OutputIDs().Slice() {
			t.ConsumeOutputMetadata(outputID, consumer)
		}
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MockedInput //////////////////////////////////////////////////////////////////////////////////////////////////

// MockedInput is a mocked entity that allows to "address" which Outputs are supposed to be used by a Transaction.
type MockedInput struct {
	outputID utxo.OutputID
}

// NewMockedInput creates a new MockedInput from an utxo.OutputID.
func NewMockedInput(outputID utxo.OutputID) *MockedInput {
	return &MockedInput{outputID: outputID}
}

// FromMarshalUtil un-serializes a MockedInput using a MarshalUtil.
func (m *MockedInput) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (err error) {
	if m == nil {
		*m = *new(MockedInput)
	}

	if err = m.outputID.FromMarshalUtil(marshalUtil); err != nil {
		return err
	}

	return nil
}

// Bytes returns a serialized version of the MockedInput.
func (m *MockedInput) Bytes() (serializedInput []byte) {
	return m.outputID.Bytes()
}

// String returns a human-readable version of the MockedInput.
func (m *MockedInput) String() (humanReadableInput string) {
	return stringify.Struct("MockedInput",
		stringify.StructField("outputID", m.outputID),
	)
}

func (m *MockedInput) utxoInput() utxo.Input {
	return m
}

var _ utxo.Input = new(MockedInput)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MockedOutput /////////////////////////////////////////////////////////////////////////////////////////////////

// MockedOutput is the container for the data produced by executing a MockedTransaction.
type MockedOutput struct {
	id      *utxo.OutputID
	idMutex sync.Mutex

	txID  utxo.TransactionID
	index uint16

	objectstorage.StorableObjectFlags
}

// NewMockedOutput creates a new MockedOutput based on the utxo.TransactionID and its index within the MockedTransaction.
func NewMockedOutput(txID utxo.TransactionID, index uint16) (new *MockedOutput) {
	return &MockedOutput{
		txID:  txID,
		index: index,
	}
}

// FromMarshalUtil un-serializes a MockedOutput using a MarshalUtil.
func (m *MockedOutput) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (err error) {
	if m == nil {
		*m = MockedOutput{}
	}

	if err = m.txID.FromMarshalUtil(marshalUtil); err != nil {
		return err
	}
	if m.index, err = marshalUtil.ReadUint16(); err != nil {
		return err
	}

	return nil
}

// ID returns the identifier of the Output.
func (m *MockedOutput) ID() (id utxo.OutputID) {
	m.idMutex.Lock()
	defer m.idMutex.Unlock()

	if m.id == nil {
		derivedID := utxo.NewOutputID(m.txID, m.index, []byte(""))
		m.id = &derivedID
	}

	return *m.id
}

// SetID sets the identifier of the Output.
func (m *MockedOutput) SetID(id utxo.OutputID) {
	m.idMutex.Lock()
	defer m.idMutex.Unlock()

	m.id = &id
}

// TransactionID returns the identifier of the Transaction that created this Output.
func (m *MockedOutput) TransactionID() (txID utxo.TransactionID) {
	return m.txID
}

// Index returns the unique Index of the Output in respect to its TransactionID.
func (m *MockedOutput) Index() (index uint16) {
	return m.index
}

// Bytes returns a serialized version of the MockedOutput.
func (m *MockedOutput) Bytes() (serializedOutput []byte) {
	return marshalutil.New().
		Write(m.txID).
		WriteUint16(m.index).
		Bytes()
}

// String returns a human-readable version of the MockedOutput.
func (m *MockedOutput) String() (humanReadableOutput string) {
	return stringify.Struct("MockedOutput",
		stringify.StructField("id", m.ID()),
		stringify.StructField("transactionID", m.TransactionID()),
		stringify.StructField("index", m.Index()),
	)
}

var _ utxo.Output = new(MockedOutput)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MockedTransaction ////////////////////////////////////////////////////////////////////////////////////////////

var _uniqueEssenceCounter uint64

// MockedTransaction is the type that is used to describe instructions how to modify the ledger state for MockedVM.
type MockedTransaction struct {
	id      *utxo.TransactionID
	idMutex sync.Mutex

	inputs        []*MockedInput
	outputCount   uint16
	uniqueEssence uint64

	objectstorage.StorableObjectFlags
}

// NewMockedTransaction creates a new MockedTransaction with the given inputs and specified outputCount.
// A unique essence is simulated by an atomic counter, incremented globally for each MockedTransaction created.
func NewMockedTransaction(inputs []*MockedInput, outputCount uint16) (new *MockedTransaction) {
	return &MockedTransaction{
		inputs:        inputs,
		outputCount:   outputCount,
		uniqueEssence: atomic.AddUint64(&_uniqueEssenceCounter, 1),
	}
}

// FromMarshalUtil un-serializes a MockedTransaction using a MarshalUtil.
func (m *MockedTransaction) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (err error) {
	if m == nil {
		*m = MockedTransaction{}
	}

	inputCount, err := marshalUtil.ReadUint16()
	if err != nil {
		return err
	}
	m.inputs = make([]*MockedInput, int(inputCount))
	for i := 0; i < int(inputCount); i++ {
		m.inputs[i] = new(MockedInput)
		if err = m.inputs[i].FromMarshalUtil(marshalUtil); err != nil {
			return err
		}
	}

	if m.outputCount, err = marshalUtil.ReadUint16(); err != nil {
		return err
	}

	if m.uniqueEssence, err = marshalUtil.ReadUint64(); err != nil {
		return err
	}

	return nil
}

// ID returns the identifier of the Transaction.
func (m *MockedTransaction) ID() (transactionID utxo.TransactionID) {
	m.idMutex.Lock()
	defer m.idMutex.Unlock()

	if m.id == nil {
		copy(transactionID.Identifier[:], marshalutil.New().WriteUint64(m.uniqueEssence).Bytes())
		m.id = &transactionID
	}

	return *m.id
}

// SetID sets the identifier of the Transaction.
func (m *MockedTransaction) SetID(id utxo.TransactionID) {
	m.idMutex.Lock()
	defer m.idMutex.Unlock()

	m.id = &id
}

// Inputs returns the inputs of the Transaction.
func (m *MockedTransaction) Inputs() []utxo.Input {
	return lo.Map(m.inputs, (*MockedInput).utxoInput)
}

// Bytes returns a serialized version of the MockedTransaction.
func (m *MockedTransaction) Bytes() []byte {
	marshalUtil := marshalutil.New().
		WriteUint16(uint16(len(m.inputs)))

	for i := 0; i < len(m.inputs); i++ {
		marshalUtil.Write(m.inputs[i])
	}

	return marshalUtil.
		WriteUint16(m.outputCount).
		WriteUint64(m.uniqueEssence).
		Bytes()
}

// String returns a human-readable version of the MockedTransaction.
func (m *MockedTransaction) String() (humanReadable string) {
	inputIDs := utxo.NewOutputIDs()
	for _, input := range m.Inputs() {
		inputIDs.Add(input.(*MockedInput).outputID)
	}

	outputIDs := utxo.NewOutputIDs()
	for i := uint16(0); i < m.outputCount; i++ {
		outputIDs.Add(utxo.NewOutputID(m.ID(), i, []byte("")))
	}

	return stringify.Struct("MockedTransaction",
		stringify.StructField("id", m.ID()),
		stringify.StructField("inputs", inputIDs),
		stringify.StructField("outputs", outputIDs),
	)
}

var _ utxo.Transaction = new(MockedTransaction)

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
	if err = mockedTx.FromMarshalUtil(marshalutil.New(transactionBytes)); err != nil {
		return nil, err
	}

	return mockedTx, nil
}

// ParseOutput un-serializes an Output from the given sequence of bytes.
func (m *MockedVM) ParseOutput(outputBytes []byte) (output utxo.Output, err error) {
	mockedOutput := new(MockedOutput)
	if err = mockedOutput.FromMarshalUtil(marshalutil.New(outputBytes)); err != nil {
		return nil, err
	}

	return mockedOutput, nil
}

// ResolveInput translates the Input into an OutputID.
func (m *MockedVM) ResolveInput(input utxo.Input) (outputID utxo.OutputID) {
	return input.(*MockedInput).outputID
}

// ExecuteTransaction executes the Transaction and determines the Outputs from the given Inputs. It returns an error
// if the execution fails.
func (m *MockedVM) ExecuteTransaction(transaction utxo.Transaction, _ []utxo.Output, _ ...uint64) (outputs []utxo.Output, err error) {
	mockedTransaction := transaction.(*MockedTransaction)

	outputs = make([]utxo.Output, mockedTransaction.outputCount)
	for i := uint16(0); i < mockedTransaction.outputCount; i++ {
		outputs[i] = NewMockedOutput(mockedTransaction.ID(), i)
	}

	return
}

var _ vm.VM = new(MockedVM)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
