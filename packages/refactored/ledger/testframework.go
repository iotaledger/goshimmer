package ledger

import (
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/refactored/branchdag"
	"github.com/iotaledger/goshimmer/packages/refactored/generics"
	"github.com/iotaledger/goshimmer/packages/refactored/types/utxo"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	Ledger *Ledger

	transactionsByAlias map[string]*MockedTransaction
	outputIDsByAlias    map[string]utxo.OutputID
}

func NewTestFramework(options ...Option) (new *TestFramework) {
	new = &TestFramework{
		Ledger: New(mapdb.NewMapDB(), NewMockedVM(), options...),

		transactionsByAlias: make(map[string]*MockedTransaction),
		outputIDsByAlias:    make(map[string]utxo.OutputID),
	}

	genesisOutput := NewOutput(NewMockedOutput(utxo.EmptyTransactionID, 0))
	genesisOutputMetadata := NewOutputMetadata(genesisOutput.ID())
	genesisOutputMetadata.SetSolid(true)
	genesisOutputMetadata.SetGradeOfFinality(gof.High)

	genesisOutput.ID().RegisterAlias("Genesis")
	new.outputIDsByAlias["Genesis"] = genesisOutput.ID()

	new.Ledger.outputStorage.Store(genesisOutput).Release()
	new.Ledger.outputMetadataStorage.Store(genesisOutputMetadata).Release()

	return new
}

func (t *TestFramework) Transaction(txAlias string) (tx *MockedTransaction) {
	tx, exists := t.transactionsByAlias[txAlias]
	if !exists {
		panic(fmt.Sprintf("tried to retrieve transaction with unknown alias: %s", txAlias))
	}

	return tx
}

func (t *TestFramework) CreateTransaction(txAlias string, outputCount uint16, inputAliases ...string) {
	mockedInputs := make([]*MockedInput, 0)
	for _, inputAlias := range inputAliases {
		outputID, exists := t.outputIDsByAlias[inputAlias]
		if !exists {
			panic(fmt.Sprintf("unknown input alias: %s", inputAlias))
		}

		mockedInputs = append(mockedInputs, NewMockedInput(outputID))
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

func (t *TestFramework) AssertBranchDAG(testing *testing.T, expectedParents map[string][]string) {
	for branchAlias, expectedParentAliases := range expectedParents {
		currentBranchID := branchdag.NewBranchID(t.Transaction(branchAlias).ID())

		expectedBranchIDs := t.BranchIDs(expectedParentAliases...)

		assert.True(testing, t.Ledger.BranchDAG.Branch(currentBranchID).Consume(func(branch *branchdag.Branch) {
			assert.Truef(testing, expectedBranchIDs.Equal(branch.Parents()), "Branch(%s): expected parents %s are not equal to actual parents %s", currentBranchID, expectedBranchIDs, branch.Parents())
		}))
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

func (t *TestFramework) AssertBranchIDs(testing *testing.T, expectedBranches map[string][]string) {
	for txAlias, expectedBranchAliases := range expectedBranches {
		currentTx := t.Transaction(txAlias)

		expectedBranchIDs := t.BranchIDs(expectedBranchAliases...)

		assert.True(testing, t.Ledger.CachedTransactionMetadata(currentTx.ID()).Consume(func(txMetadata *TransactionMetadata) {
			assert.Truef(testing, expectedBranchIDs.Equal(txMetadata.BranchIDs()), "Transaction(%s): expected %s is not equal to actual %s", txAlias, expectedBranchIDs, txMetadata.BranchIDs())
		}))

		for i := uint16(0); i < currentTx.outputCount; i++ {
			assert.True(testing, t.Ledger.CachedOutputMetadata(utxo.NewOutputID(currentTx.ID(), i, []byte(""))).Consume(func(outputMetadata *OutputMetadata) {
				assert.Truef(testing, expectedBranchIDs.Equal(outputMetadata.BranchIDs()), "Output(%s): expected %s is not equal to actual %s", outputMetadata.ID(), expectedBranchIDs, outputMetadata.BranchIDs())
			}))
		}
	}
}

func (t *TestFramework) IssueTransaction(txAlias string) (err error) {
	transaction, exists := t.transactionsByAlias[txAlias]
	if !exists {
		panic(fmt.Sprintf("unknown transaction alias: %s", txAlias))
	}

	return t.Ledger.StoreAndProcessTransaction(transaction)
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
	return generics.Map(m.inputs, (*MockedInput).utxoInput)
}

func (m *MockedTransaction) Bytes() []byte {
	return nil
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

var _ utxo.VM = new(MockedVM)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
