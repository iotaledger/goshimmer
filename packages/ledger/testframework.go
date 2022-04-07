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

type TestFramework struct {
	t                   *testing.T
	ledger              *Ledger
	transactionsByAlias map[string]*MockedTransaction
	outputIDsByAlias    map[string]utxo.OutputID
}

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

func (t *TestFramework) Transaction(txAlias string) (tx *MockedTransaction) {
	tx, exists := t.transactionsByAlias[txAlias]
	if !exists {
		panic(fmt.Sprintf("tried to retrieve transaction with unknown alias: %s", txAlias))
	}

	return tx
}

func (t *TestFramework) OutputID(alias string) (outputID utxo.OutputID) {
	outputID, exists := t.outputIDsByAlias[alias]
	if !exists {
		panic(fmt.Sprintf("unknown output alias: %s", alias))
	}

	return outputID
}

func (t *TestFramework) CreateTransaction(txAlias string, outputCount uint16, inputAliases ...string) {
	mockedInputs := make([]*MockedInput, 0)
	for _, inputAlias := range inputAliases {
		mockedInputs = append(mockedInputs, NewMockedInput(t.OutputID(inputAlias)))
	}

	tx := NewMockedTransaction(mockedInputs, outputCount)
	tx.ID().RegisterAlias(txAlias)
	t.transactionsByAlias[txAlias] = tx

	for i := uint16(0); i < outputCount; i++ {
		outputID := utxo.NewOutputID(tx.ID(), i, []byte(""))
		outputAlias := txAlias + "." + strconv.Itoa(int(i))

		outputID.RegisterAlias(outputAlias)
		t.outputIDsByAlias[outputAlias] = outputID
	}
}

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

func (t *TestFramework) TransactionIDs(txAliases ...string) (txIDs utxo.TransactionIDs) {
	txIDs = utxo.NewTransactionIDs()
	for _, expectedBranchAlias := range txAliases {
		txIDs.Add(t.Transaction(expectedBranchAlias).ID())
	}

	return txIDs
}

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

func (t *TestFramework) AssertBooked(expectedBookedMap map[string]bool) {
	for txAlias, expectedBooked := range expectedBookedMap {
		currentTx := t.Transaction(txAlias)
		t.ConsumeTransactionMetadata(currentTx.ID(), func(txMetadata *TransactionMetadata) {
			assert.Equalf(t.t, expectedBooked, txMetadata.Booked(), "Transaction(%s): expected booked(%s) but has booked(%s)", txAlias, expectedBooked, txMetadata.Booked())

			_ = txMetadata.OutputIDs().ForEach(func(outputID utxo.OutputID) (err error) {
				// Check if output exists according to the Booked status of the enclosing Transaction.
				assert.Equalf(t.t, expectedBooked, t.ledger.Storage.CachedOutputMetadata(outputID).Consume(func(_ *OutputMetadata) {}),
					"Output(%s): expected booked(%s) but has booked(%s)", outputID, expectedBooked, txMetadata.Booked())
				return nil
			})
		})
	}
}

func (t *TestFramework) AllBooked(txAliases ...string) (allBooked bool) {
	for _, txAlias := range txAliases {
		t.ConsumeTransactionMetadata(t.Transaction(txAlias).ID(), func(txMetadata *TransactionMetadata) {
			allBooked = txMetadata.Booked()
		})

		if !allBooked {
			return
		}
	}

	return
}

func (t *TestFramework) ConsumeBranch(branchID branchdag.BranchID, consumer func(branch *branchdag.Branch)) {
	assert.Truef(t.t, t.ledger.BranchDAG.Storage.CachedBranch(branchID).Consume(consumer), "failed to load branch %s", branchID)
}

func (t *TestFramework) ConsumeTransactionMetadata(txID utxo.TransactionID, consumer func(txMetadata *TransactionMetadata)) {
	assert.Truef(t.t, t.ledger.Storage.CachedTransactionMetadata(txID).Consume(consumer), "failed to load metadata of %s", txID)
}

func (t *TestFramework) ConsumeOutputMetadata(outputID utxo.OutputID, consumer func(outputMetadata *OutputMetadata)) {
	assert.True(t.t, t.ledger.Storage.CachedOutputMetadata(outputID).Consume(consumer))
}

func (t *TestFramework) ConsumeOutput(outputID utxo.OutputID, consumer func(output *Output)) {
	assert.True(t.t, t.ledger.Storage.CachedOutput(outputID).Consume(consumer))
}

func (t *TestFramework) ConsumeTransactionOutputs(mockTx *MockedTransaction, consumer func(outputMetadata *OutputMetadata)) {
	t.ConsumeTransactionMetadata(mockTx.ID(), func(txMetadata *TransactionMetadata) {
		assert.EqualValuesf(t.t, mockTx.outputCount, txMetadata.OutputIDs().Size(), "Output count in %s do not match", mockTx.ID())

		for _, outputID := range txMetadata.OutputIDs().Slice() {
			t.ConsumeOutputMetadata(outputID, consumer)
		}
	})
}

func (t *TestFramework) IssueTransaction(txAlias string) (err error) {
	transaction, exists := t.transactionsByAlias[txAlias]
	if !exists {
		panic(fmt.Sprintf("unknown transaction alias: %s", txAlias))
	}

	return t.ledger.StoreAndProcessTransaction(transaction)
}

func (t *TestFramework) MockOutputFromTx(tx *MockedTransaction, outputIndex uint16) (mockedOutputID utxo.OutputID) {
	return utxo.NewOutputID(tx.ID(), outputIndex, []byte(""))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MockedInput //////////////////////////////////////////////////////////////////////////////////////////////////

type MockedInput struct {
	outputID utxo.OutputID
}

func NewMockedInput(outputID utxo.OutputID) *MockedInput {
	return &MockedInput{outputID: outputID}
}

func (m *MockedInput) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (err error) {
	if m == nil {
		*m = *new(MockedInput)
	}

	if err = m.outputID.FromMarshalUtil(marshalUtil); err != nil {
		return err
	}

	return nil
}

func (m *MockedInput) Bytes() (serializedInput []byte) {
	return m.outputID.Bytes()
}

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

type MockedOutput struct {
	id      *utxo.OutputID
	idMutex sync.Mutex

	txID  utxo.TransactionID
	index uint16

	objectstorage.StorableObjectFlags
}

func NewMockedOutput(txID utxo.TransactionID, index uint16) (new *MockedOutput) {
	return &MockedOutput{
		txID:  txID,
		index: index,
	}
}

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

func (m *MockedOutput) ID() (id utxo.OutputID) {
	m.idMutex.Lock()
	defer m.idMutex.Unlock()

	if m.id == nil {
		derivedID := utxo.NewOutputID(m.txID, m.index, []byte(""))
		m.id = &derivedID
	}

	return *m.id
}

func (m *MockedOutput) SetID(id utxo.OutputID) {
	m.idMutex.Lock()
	defer m.idMutex.Unlock()

	m.id = &id
}

func (m *MockedOutput) TransactionID() (txID utxo.TransactionID) {
	return m.txID
}

func (m *MockedOutput) Index() (index uint16) {
	return m.index
}

func (m *MockedOutput) Bytes() (serializedOutput []byte) {
	return marshalutil.New().
		Write(m.txID).
		WriteUint16(m.index).
		Bytes()
}

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

type MockedTransaction struct {
	id      *utxo.TransactionID
	idMutex sync.Mutex

	inputs        []*MockedInput
	outputCount   uint16
	uniqueEssence uint64

	objectstorage.StorableObjectFlags
}

func NewMockedTransaction(inputs []*MockedInput, outputCount uint16) (new *MockedTransaction) {
	return &MockedTransaction{
		inputs:        inputs,
		outputCount:   outputCount,
		uniqueEssence: atomic.AddUint64(&_uniqueEssenceCounter, 1),
	}
}

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

func (m *MockedTransaction) ID() (transactionID utxo.TransactionID) {
	m.idMutex.Lock()
	defer m.idMutex.Unlock()

	if m.id == nil {
		copy(transactionID.Identifier[:], marshalutil.New().WriteUint64(m.uniqueEssence).Bytes())
		m.id = &transactionID
	}

	return *m.id
}

func (m *MockedTransaction) SetID(id utxo.TransactionID) {
	m.idMutex.Lock()
	defer m.idMutex.Unlock()

	m.id = &id
}

func (m *MockedTransaction) Inputs() []utxo.Input {
	return lo.Map(m.inputs, (*MockedInput).utxoInput)
}

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

type MockedVM struct{}

func NewMockedVM() *MockedVM {
	return new(MockedVM)
}

func (m *MockedVM) ParseTransaction(transactionBytes []byte) (transaction utxo.Transaction, err error) {
	mockedTx := new(MockedTransaction)
	if err = mockedTx.FromMarshalUtil(marshalutil.New(transactionBytes)); err != nil {
		return nil, err
	}

	return mockedTx, nil
}

func (m *MockedVM) ParseOutput(outputBytes []byte) (output utxo.Output, err error) {
	mockedOutput := new(MockedOutput)
	if err = mockedOutput.FromMarshalUtil(marshalutil.New(outputBytes)); err != nil {
		return nil, err
	}

	return mockedOutput, nil
}

func (m *MockedVM) ResolveInput(input utxo.Input) (outputID utxo.OutputID) {
	return input.(*MockedInput).outputID
}

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
