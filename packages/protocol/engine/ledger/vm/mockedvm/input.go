package mockedvm

import (
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/hive.go/stringify"
)

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
