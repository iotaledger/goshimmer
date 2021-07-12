package ledgerstate

import (
	"github.com/stretchr/testify/mock"
	"testing"
)

type utxoDagMock struct {
	mock.Mock
	test    *testing.T
	utxoDag IUTXODAG
}

// NewUtxoDagMock creates a mock for UTXODAG
func NewUtxoDagMock(t *testing.T, utxoDag IUTXODAG) *utxoDagMock {
	u := &utxoDagMock{
		test: t,
	}
	u.Test(t)
	u.utxoDag = utxoDag
	return u
}

func (u *utxoDagMock) Events() *UTXODAGEvents {
	return u.utxoDag.Events()
}

func (u *utxoDagMock) InclusionState(transactionID TransactionID) (inclusionState InclusionState, err error) {
	args := u.Called(transactionID)
	inclusionState = args.Get(0).(InclusionState)
	return
}

func (u *utxoDagMock) Shutdown() {
	u.utxoDag.Shutdown()
	return
}

func (u *utxoDagMock) StoreTransaction(transaction *Transaction) (stored bool, solidityType SolidityType, err error) {
	return u.utxoDag.StoreTransaction(transaction)
}

func (u *utxoDagMock) CheckTransaction(transaction *Transaction) (err error) {
	return u.utxoDag.CheckTransaction(transaction)
}

func (u *utxoDagMock) CachedTransaction(transactionID TransactionID) (cachedTransaction *CachedTransaction) {
	return u.utxoDag.CachedTransaction(transactionID)
}

func (u *utxoDagMock) Transaction(transactionID TransactionID) (transaction *Transaction) {
	return u.utxoDag.Transaction(transactionID)
}

func (u *utxoDagMock) Transactions() (transactions map[TransactionID]*Transaction) {
	return u.utxoDag.Transactions()
}

func (u *utxoDagMock) CachedTransactionMetadata(transactionID TransactionID, computeIfAbsentCallback ...func(transactionID TransactionID) *TransactionMetadata) (cachedTransactionMetadata *CachedTransactionMetadata) {
	return u.utxoDag.CachedTransactionMetadata(transactionID, computeIfAbsentCallback...)
}

func (u *utxoDagMock) CachedOutput(outputID OutputID) (cachedOutput *CachedOutput) {
	return u.utxoDag.CachedOutput(outputID)
}

func (u *utxoDagMock) CachedOutputMetadata(outputID OutputID) (cachedOutput *CachedOutputMetadata) {
	return u.utxoDag.CachedOutputMetadata(outputID)
}

func (u *utxoDagMock) CachedConsumers(outputID OutputID, optionalSolidityType ...SolidityType) (cachedConsumers CachedConsumers) {
	return u.utxoDag.CachedConsumers(outputID, optionalSolidityType...)
}

func (u *utxoDagMock) LoadSnapshot(snapshot *Snapshot) {
	u.utxoDag.LoadSnapshot(snapshot)
}

func (u *utxoDagMock) CachedAddressOutputMapping(address Address) (cachedAddressOutputMappings CachedAddressOutputMappings) {
	return u.utxoDag.CachedAddressOutputMapping(address)
}

func (u *utxoDagMock) SetTransactionConfirmed(transactionID TransactionID) (err error) {
	return u.utxoDag.SetTransactionConfirmed(transactionID)
}

func (u *utxoDagMock) ConsumedOutputs(transaction *Transaction) (cachedInputs CachedOutputs) {
	return u.utxoDag.ConsumedOutputs(transaction)
}

func (u *utxoDagMock) ManageStoreAddressOutputMapping(output Output) {
	u.utxoDag.ManageStoreAddressOutputMapping(output)
}

func (u *utxoDagMock) StoreAddressOutputMapping(address Address, outputID OutputID) {
	u.utxoDag.StoreAddressOutputMapping(address, outputID)
}
