package ledgerstate

import (
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"
)

// region UTXODAG //////////////////////////////////////////////////////////////////////////////////////////////////////

type UTXODAG struct {
	Events *UTXODAGEvents

	transactionStorage          *objectstorage.ObjectStorage
	transactionMetadataStorage  *objectstorage.ObjectStorage
	outputStorage               *objectstorage.ObjectStorage
	consumerStorage             *objectstorage.ObjectStorage
	addressOutputMappingStorage *objectstorage.ObjectStorage
	branchDAG                   *BranchDAG
}

func NewUTXODAG(store kvstore.KVStore, branchDAG *BranchDAG) (utxoDAG *UTXODAG) {
	utxoDAG = &UTXODAG{
	}

	return
}

func (u *UTXODAG) Transaction(transactionID TransactionID) (cachedTransaction *CachedTransaction) {
	return
}

func (u *UTXODAG) TransactionMetadata(transactionID TransactionID) (cachedTransactionMetadata *CachedTransactionMetadata) {
	return
}

func (u *UTXODAG) Output(outputID OutputID) (cachedOutput *CachedOutput) {
	return
}

func (u *UTXODAG) OutputsOnAddress(address Address) (cachedOutputsOnAddresses *CachedOutputsOnAddresses) {
	return
}

func (u *UTXODAG) Consumers(outputID OutputID) (consumers *CachedConsumers) {
	return
}

func (u *UTXODAG) SetTransactionPreferred(transactionID TransactionID, preferred bool) (modified bool, err error) {
	return
}

func (u *UTXODAG) SetTransactionFinalized(transactionID TransactionID, finalized bool) (modified bool, err error) {
	return
}

func (u *UTXODAG) IsSolid(transaction *Transaction) (solid bool, err error) {
	return
}

func (u *UTXODAG) IsBalancesValid(transaction *Transaction) (valid bool, err error) {
	// check sum of balances == 0

	return
}

func (u *UTXODAG) BranchOfTransaction(transaction *Transaction) (branch BranchID, err error) {
	// check that branches are not conflicting
	// check that transaction does not use an input that was already spent in its past cone

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region UTXODAGEvents ////////////////////////////////////////////////////////////////////////////////////////////////

type UTXODAGEvents struct {
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedOutputsOnAddresses /////////////////////////////////////////////////////////////////////////////////////

type CachedOutputsOnAddresses struct {

}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Consumer /////////////////////////////////////////////////////////////////////////////////////////////////////

type Consumer struct {
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedConsumer ///////////////////////////////////////////////////////////////////////////////////////////////

type CachedConsumer struct {
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedConsumers //////////////////////////////////////////////////////////////////////////////////////////////

type CachedConsumers struct {
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////