package utxo

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/objectstorage/generic/model"
)

func TestOutputCommitment(t *testing.T) {
	output1 := NewMockedOutput()
	output1.SetID(NewOutputID(NewTransactionID([]byte{1, 2, 3}), 0))
	output2 := NewMockedOutput()
	output2.SetID(NewOutputID(NewTransactionID([]byte{1, 2, 3}), 1))

	outputCommitment := new(OutputCommitment)
	require.NoError(t, outputCommitment.FromOutputs(output1, output2))

	proof, err := outputCommitment.Proof(0)
	require.NoError(t, err)

	err = proof.Validate(output1)
	require.NoError(t, err)
}

// region MockedOutput /////////////////////////////////////////////////////////////////////////////////////////////////

// MockedOutput is the container for the data produced by executing a MockedTransaction.
type MockedOutput struct {
	model.Storable[OutputID, MockedOutput, *MockedOutput, mockedOutput] `serix:"0"`
}

type mockedOutput struct {
	UniqueEssence uint64 `serix:"0"`
}

// NewMockedOutput creates a new MockedOutput based on the utxo.TransactionID and its index within the MockedTransaction.
func NewMockedOutput() *MockedOutput {
	return model.NewStorable[OutputID, MockedOutput](&mockedOutput{
		UniqueEssence: atomic.AddUint64(&_uniqueEssenceCounter, 1),
	})
}

var _uniqueEssenceCounter uint64

// code contract (make sure the struct implements all required methods).
var _ Output = new(MockedOutput)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
