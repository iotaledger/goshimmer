package ledgerstate

import (
	"testing"

	"github.com/stretchr/testify/mock"
)

type UtxoDagMock struct {
	mock.Mock
	test    *testing.T
	utxoDag IUTXODAG
}

// NewUtxoDagMock creates a mock for UTXODAG
func NewUtxoDagMock(t *testing.T, utxoDag IUTXODAG) *UtxoDagMock {
	u := &UtxoDagMock{
		test: t,
	}
	u.Test(t)
	u.utxoDag = utxoDag
	return u
}

func (u *UtxoDagMock) Events() *UTXODAGEvents {
	return u.utxoDag.Events()
}

func (u *UtxoDagMock) InclusionState(transactionID TransactionID) (inclusionState InclusionState, err error) {
	args := u.Called(transactionID)
	inclusionState = args.Get(0).(InclusionState)
	return
}

func (u *UtxoDagMock) Shutdown() {
	u.utxoDag.Shutdown()
}

func (u *UtxoDagMock) StoreTransaction(transaction *Transaction) (stored bool, solidityType SolidityType, err error) {
	return u.utxoDag.StoreTransaction(transaction)
}

func (u *UtxoDagMock) CheckTransaction(transaction *Transaction) (err error) {
	return u.utxoDag.CheckTransaction(transaction)
}

func (u *UtxoDagMock) CachedTransaction(transactionID TransactionID) (cachedTransaction *CachedTransaction) {
	return u.utxoDag.CachedTransaction(transactionID)
}

func (u *UtxoDagMock) Transaction(transactionID TransactionID) (transaction *Transaction) {
	return u.utxoDag.Transaction(transactionID)
}

func (u *UtxoDagMock) Transactions() (transactions map[TransactionID]*Transaction) {
	return u.utxoDag.Transactions()
}

func (u *UtxoDagMock) CachedTransactionMetadata(transactionID TransactionID, computeIfAbsentCallback ...func(transactionID TransactionID) *TransactionMetadata) (cachedTransactionMetadata *CachedTransactionMetadata) {
	return u.utxoDag.CachedTransactionMetadata(transactionID, computeIfAbsentCallback...)
}

func (u *UtxoDagMock) CachedOutput(outputID OutputID) (cachedOutput *CachedOutput) {
	return u.utxoDag.CachedOutput(outputID)
}

func (u *UtxoDagMock) CachedOutputMetadata(outputID OutputID) (cachedOutput *CachedOutputMetadata) {
	return u.utxoDag.CachedOutputMetadata(outputID)
}

func (u *UtxoDagMock) CachedConsumers(outputID OutputID, optionalSolidityType ...SolidityType) (cachedConsumers CachedConsumers) {
	return u.utxoDag.CachedConsumers(outputID, optionalSolidityType...)
}

func (u *UtxoDagMock) LoadSnapshot(snapshot *Snapshot) {
	u.utxoDag.LoadSnapshot(snapshot)
}

func (u *UtxoDagMock) CachedAddressOutputMapping(address Address) (cachedAddressOutputMappings CachedAddressOutputMappings) {
	return u.utxoDag.CachedAddressOutputMapping(address)
}

func (u *UtxoDagMock) SetTransactionConfirmed(transactionID TransactionID) (err error) {
	return u.utxoDag.SetTransactionConfirmed(transactionID)
}

func (u *UtxoDagMock) ConsumedOutputs(transaction *Transaction) (cachedInputs CachedOutputs) {
	return u.utxoDag.ConsumedOutputs(transaction)
}

func (u *UtxoDagMock) ManageStoreAddressOutputMapping(output Output) {
	u.utxoDag.ManageStoreAddressOutputMapping(output)
}

func (u *UtxoDagMock) StoreAddressOutputMapping(address Address, outputID OutputID) {
	u.utxoDag.StoreAddressOutputMapping(address, outputID)
}
