package ledger

import (
	"sync"

	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

// region MockedInput //////////////////////////////////////////////////////////////////////////////////////////////////

type MockedInput struct {
	outputID utxo.OutputID
}

func (m *MockedInput) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (input *MockedInput, err error) {
	if input = m; input == nil {
		input = new(MockedInput)
	}

	if m.outputID, err = utxo.OutputIDFromMarshalUtil(marshalUtil); err != nil {
		return nil, err
	}

	return m, nil
}

func (m *MockedInput) Bytes() (serializedInput []byte) {
	return m.outputID.Bytes()
}

func (m *MockedInput) String() (humanReadableInput string) {
	return stringify.Struct("MockedInput",
		stringify.StructField("outputID", m.outputID),
	)
}

var _ utxo.Input = new(MockedInput)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MockedOutput /////////////////////////////////////////////////////////////////////////////////////////////////

type MockedOutput struct {
	id      utxo.OutputID
	idMutex sync.RWMutex

	objectstorage.StorableObjectFlags
}

func (m *MockedOutput) ID() (id utxo.OutputID) {
	m.idMutex.RLock()
	defer m.idMutex.RUnlock()

	return m.id
}

func (m *MockedOutput) SetID(id utxo.OutputID) {
	m.idMutex.Lock()
	defer m.idMutex.Unlock()

	m.id = id
}

func (m *MockedOutput) Bytes() (serializedOutput []byte) {
	return m.ID().Bytes()
}

func (m *MockedOutput) String() (humanReadableOutput string) {
	return stringify.Struct("MockedOutput",
		stringify.StructField("id", m.ID()),
	)
}

func (m *MockedOutput) FromObjectStorage(key, _ []byte) (object objectstorage.StorableObject, err error) {
	id, _, err := utxo.OutputIDFromBytes(key)
	if err != nil {
		return nil, err
	}

	output := new(MockedOutput)
	output.SetID(id)

	return output, nil
}

func (m *MockedOutput) ObjectStorageKey() []byte {
	return m.ID().Bytes()
}

func (m *MockedOutput) ObjectStorageValue() []byte {
	return nil
}

var _ utxo.Output = new(MockedOutput)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region mockedTransaction ////////////////////////////////////////////////////////////////////////////////////////////

type mockedTransaction struct {
	id           utxo.TransactionID
	inputs       []utxo.Input
	outputsCount int

	objectstorage.StorableObjectFlags
}

func (m *mockedTransaction) ID() utxo.TransactionID {
	return m.id
}

func (m *mockedTransaction) Inputs() []utxo.Input {
	return m.inputs
}

func (m *mockedTransaction) Bytes() []byte {
	return m.ObjectStorageValue()
}

func (m *mockedTransaction) FromObjectStorage(_, data []byte) (result objectstorage.StorableObject, err error) {
	marshalUtil := marshalutil.New(data)

	if m.id, err = utxo.TransactionIDFromMarshalUtil(marshalUtil); err != nil {
		return
	}

	inputsCount, err := marshalUtil.ReadUint64()
	if err != nil {
		return
	}
	inputs := make([]utxo.Input, int(inputsCount))
	for i := uint64(0); i < inputsCount; i++ {
		if inputs[i], err = new(MockedInput).FromMarshalUtil(marshalUtil); err != nil {
			return
		}
	}
	outputsCount, err := marshalUtil.ReadUint64()
	if err != nil {
		return
	}
	m.outputsCount = int(outputsCount)

	return m, nil
}

func (m *mockedTransaction) ObjectStorageKey() []byte {
	return m.id.Bytes()
}

func (m *mockedTransaction) ObjectStorageValue() []byte {
	marshalUtil := marshalutil.New()

	marshalUtil.Write(m.ID())
	marshalUtil.WriteUint64(uint64(len(m.Inputs())))
	for _, input := range m.Inputs() {
		marshalUtil.Write(input)
	}
	marshalUtil.WriteUint64(uint64(m.outputsCount))

	return marshalUtil.Bytes()
}

var _ utxo.Transaction = new(mockedTransaction)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region mockedVM /////////////////////////////////////////////////////////////////////////////////////////////////////

type mockedVM struct{}

func (m *mockedVM) ParseTransaction(transactionBytes []byte) (transaction utxo.Transaction, err error) {
	mockedTx, err := new(mockedTransaction).FromObjectStorage(nil, transactionBytes)

	return mockedTx.(*mockedTransaction), err
}

func (m *mockedVM) ParseOutput(outputBytes []byte) (output utxo.Output, err error) {
	// TODO implement me
	panic("implement me")
}

func (m *mockedVM) ResolveInput(input utxo.Input) (outputID utxo.OutputID) {
	return input.(*MockedInput).outputID
}

func (m *mockedVM) ExecuteTransaction(transaction utxo.Transaction, inputs []utxo.Output, executionLimit ...uint64) (outputs []utxo.Output, err error) {
	// TODO implement me
	panic("implement me")
}

var _ utxo.VM = new(mockedVM)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
