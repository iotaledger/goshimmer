package ledger

import (
	"sync"
	"sync/atomic"

	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/refactored/generics"
	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

// region MockedInput //////////////////////////////////////////////////////////////////////////////////////////////////

type MockedInput struct {
	outputID utxo.OutputID
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
		copy(transactionID[:], marshalutil.New().WriteUint64(m.uniqueEssence).Bytes())
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
	return stringify.Struct("MockedTransaction",
		stringify.StructField("id", m.ID()),
	)
}

var _ utxo.Transaction = new(MockedTransaction)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MockedVM /////////////////////////////////////////////////////////////////////////////////////////////////////

type MockedVM struct{}

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
