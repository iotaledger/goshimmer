package mockedvm

import (
	"sync/atomic"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models/payload"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/objectstorage/generic/model"
)

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
