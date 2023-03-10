package mockedvm

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/protocol/models/payload"
	"github.com/iotaledger/goshimmer/packages/protocol/models/payloadtype"
	"github.com/iotaledger/hive.go/serializer/v2/serix"
)

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
