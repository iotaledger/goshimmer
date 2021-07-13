package ledgerstate

import (
	"testing"

	"github.com/stretchr/testify/mock"
)

// UtxoDagMock is the mock for UTXODAG that is used for unit tests.
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

// Events returns all events of the underlying UTXODAG
func (u *UtxoDagMock) Events() *UTXODAGEvents {
	return u.utxoDag.Events()
}

// InclusionState returns the mocked InclusionState of the Transaction with the given TransactionID.
func (u *UtxoDagMock) InclusionState(transactionID TransactionID) (inclusionState InclusionState, err error) {
	args := u.Called(transactionID)
	inclusionState = args.Get(0).(InclusionState)
	return
}

// Shutdown shuts down the underlying UTXODAG and persists its state.
func (u *UtxoDagMock) Shutdown() {
	u.utxoDag.Shutdown()
}

// StoreTransaction uses the underling UTXODAG method to store the transaction.
func (u *UtxoDagMock) StoreTransaction(transaction *Transaction) (stored bool, solidityType SolidityType, err error) {
	return u.utxoDag.StoreTransaction(transaction)
}

// CheckTransaction uses the underling UTXODAG method to contain fast checks that have to be performed before booking a Transaction.
func (u *UtxoDagMock) CheckTransaction(transaction *Transaction) (err error) {
	return u.utxoDag.CheckTransaction(transaction)
}

// CachedTransaction uses the underling UTXODAG method to retrieves the Transaction with the given TransactionID from the object storage.
func (u *UtxoDagMock) CachedTransaction(transactionID TransactionID) (cachedTransaction *CachedTransaction) {
	return u.utxoDag.CachedTransaction(transactionID)
}

// Transaction uses the underling UTXODAG method to return a specific transaction, consumed.
func (u *UtxoDagMock) Transaction(transactionID TransactionID) (transaction *Transaction) {
	return u.utxoDag.Transaction(transactionID)
}

// Transactions uses the underling UTXODAG method to return all the transactions, consumed.
func (u *UtxoDagMock) Transactions() (transactions map[TransactionID]*Transaction) {
	return u.utxoDag.Transactions()
}

// CachedTransactionMetadata uses the underling UTXODAG method to retrieve the TransactionMetadata
// with the given TransactionID from the object storage.
func (u *UtxoDagMock) CachedTransactionMetadata(transactionID TransactionID, computeIfAbsentCallback ...func(transactionID TransactionID) *TransactionMetadata) (cachedTransactionMetadata *CachedTransactionMetadata) {
	return u.utxoDag.CachedTransactionMetadata(transactionID, computeIfAbsentCallback...)
}

// CachedOutput uses the underling UTXODAG method to retrieve the Output with the given OutputID from the object storage.
func (u *UtxoDagMock) CachedOutput(outputID OutputID) (cachedOutput *CachedOutput) {
	return u.utxoDag.CachedOutput(outputID)
}

// CachedOutputMetadata uses the underling UTXODAG method to retrieve the OutputMetadata with the given OutputID from the object storage.
func (u *UtxoDagMock) CachedOutputMetadata(outputID OutputID) (cachedOutput *CachedOutputMetadata) {
	return u.utxoDag.CachedOutputMetadata(outputID)
}

// CachedConsumers uses the underling UTXODAG method to retrieve the Consumers of the given OutputID from the object storage.
func (u *UtxoDagMock) CachedConsumers(outputID OutputID, optionalSolidityType ...SolidityType) (cachedConsumers CachedConsumers) {
	return u.utxoDag.CachedConsumers(outputID, optionalSolidityType...)
}

// LoadSnapshot uses the underling UTXODAG method to create a set of outputs in the UTXO-DAG, that are forming the genesis
// for future transactions.
func (u *UtxoDagMock) LoadSnapshot(snapshot *Snapshot) {
	u.utxoDag.LoadSnapshot(snapshot)
}

// CachedAddressOutputMapping uses the underling UTXODAG method to retrieve the outputs for the given address.
func (u *UtxoDagMock) CachedAddressOutputMapping(address Address) (cachedAddressOutputMappings CachedAddressOutputMappings) {
	return u.utxoDag.CachedAddressOutputMapping(address)
}

// SetTransactionConfirmed uses the underling UTXODAG method to mark a Transaction (and all Transactions in its past cone)
// as confirmed. It also marks the
// conflicting Transactions to be rejected.
func (u *UtxoDagMock) SetTransactionConfirmed(transactionID TransactionID) (err error) {
	return u.utxoDag.SetTransactionConfirmed(transactionID)
}

// ConsumedOutputs uses the underling UTXODAG method to return the consumed (cached)Outputs of the given Transaction.
func (u *UtxoDagMock) ConsumedOutputs(transaction *Transaction) (cachedInputs CachedOutputs) {
	return u.utxoDag.ConsumedOutputs(transaction)
}

// ManageStoreAddressOutputMapping uses the underling UTXODAG method to mangage how to store the address-output mapping
// dependent on which type of output it is.
func (u *UtxoDagMock) ManageStoreAddressOutputMapping(output Output) {
	u.utxoDag.ManageStoreAddressOutputMapping(output)
}

// StoreAddressOutputMapping uses the underling UTXODAG method to store the address-output mapping.
func (u *UtxoDagMock) StoreAddressOutputMapping(address Address, outputID OutputID) {
	u.utxoDag.StoreAddressOutputMapping(address, outputID)
}
