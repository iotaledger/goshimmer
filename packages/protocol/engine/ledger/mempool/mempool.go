package mempool

import (
	"context"

	"github.com/iotaledger/goshimmer/packages/core/confirmation"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/ds/walker"
	"github.com/iotaledger/hive.go/objectstorage/generic"
	"github.com/iotaledger/hive.go/runtime/module"
)

type MemPool interface {
	// Events is a dictionary for MemPool related events.
	Events() *Events

	// Storage provides access to the stored models inside the MemPool.
	Storage() Storage

	// Utils provides various helpers to access the MemPool.
	Utils() Utils

	// ConflictDAG is a reference to the ConflictDAG that is used by this MemPool.
	ConflictDAG() *conflictdag.ConflictDAG[utxo.TransactionID, utxo.OutputID]

	// StoreAndProcessTransaction stores and processes the given Transaction.
	StoreAndProcessTransaction(ctx context.Context, tx utxo.Transaction) (err error)

	// PruneTransaction removes a Transaction from the MemPool (e.g. after it was orphaned or found to be invalid). If the
	// pruneFutureCone flag is true, then we do not just remove the named Transaction but also its future cone.
	PruneTransaction(txID utxo.TransactionID, pruneFutureCone bool)

	// SetTransactionInclusionSlot sets the inclusion timestamp of a Transaction.
	SetTransactionInclusionSlot(id utxo.TransactionID, inclusionSlot slot.Index)

	// CheckTransaction checks the validity of a Transaction.
	CheckTransaction(ctx context.Context, tx utxo.Transaction) (err error)

	// VM is the vm used for transaction validation.
	VM() vm.VM

	// Shutdown should be called when the module is stopped.
	Shutdown()

	module.Interface
}

type Utils interface {
	// ResolveInputs returns the OutputIDs that were referenced by the given Inputs.
	ResolveInputs(inputs []utxo.Input) (outputIDs utxo.OutputIDs)

	ConflictIDsInFutureCone(conflictIDs utxo.TransactionIDs) (conflictIDsInFutureCone utxo.TransactionIDs)

	ReferencedTransactions(tx utxo.Transaction) (transactionIDs utxo.TransactionIDs)

	// TransactionConfirmationState returns the ConfirmationState of the Transaction with the given TransactionID.
	TransactionConfirmationState(txID utxo.TransactionID) (confirmationState confirmation.State)

	// WithTransactionAndMetadata walks over the transactions that consume the named OutputIDs and calls the callback
	// with their corresponding Transaction and TransactionMetadata.
	WithTransactionAndMetadata(txID utxo.TransactionID, callback func(tx utxo.Transaction, txMetadata *TransactionMetadata))

	// WalkConsumingTransactionID walks over the TransactionIDs that consume the named OutputIDs.
	WalkConsumingTransactionID(entryPoints utxo.OutputIDs, callback func(consumingTxID utxo.TransactionID, walker *walker.Walker[utxo.OutputID]))

	// WalkConsumingTransactionMetadata walks over the transactions that consume the named OutputIDs and calls the callback
	// with their corresponding TransactionMetadata.
	WalkConsumingTransactionMetadata(entryPoints utxo.OutputIDs, callback func(txMetadata *TransactionMetadata, walker *walker.Walker[utxo.OutputID]))

	ConfirmedConsumer(outputID utxo.OutputID) (consumerID utxo.TransactionID)
}

type Storage interface {
	// OutputStorage is an object storage used to persist Output objects.
	OutputStorage() *generic.ObjectStorage[utxo.Output]

	// OutputMetadataStorage is an object storage used to persist OutputMetadata objects.
	OutputMetadataStorage() *generic.ObjectStorage[*OutputMetadata]

	// CachedOutput retrieves the CachedObject representing the named Output. The optional computeIfAbsentCallback can be
	// used to dynamically Construct a non-existing Output.
	CachedOutput(outputID utxo.OutputID, computeIfAbsentCallback ...func(outputID utxo.OutputID) utxo.Output) (cachedOutput *generic.CachedObject[utxo.Output])

	// CachedOutputs retrieves the CachedObjects containing the named Outputs.
	CachedOutputs(outputIDs utxo.OutputIDs) (cachedOutputs generic.CachedObjects[utxo.Output])

	// CachedOutputMetadata retrieves the CachedObject representing the named OutputMetadata. The optional
	// computeIfAbsentCallback can be used to dynamically Construct a non-existing OutputMetadata.
	CachedOutputMetadata(outputID utxo.OutputID, computeIfAbsentCallback ...func(outputID utxo.OutputID) *OutputMetadata) (cachedOutputMetadata *generic.CachedObject[*OutputMetadata])

	// CachedOutputsMetadata retrieves the CachedObjects containing the named OutputMetadata.
	CachedOutputsMetadata(outputIDs utxo.OutputIDs) (cachedOutputsMetadata generic.CachedObjects[*OutputMetadata])

	// CachedTransaction retrieves the CachedObject representing the named Transaction. The optional computeIfAbsentCallback
	// can be used to dynamically Construct a non-existing Transaction.
	CachedTransaction(transactionID utxo.TransactionID, computeIfAbsentCallback ...func(transactionID utxo.TransactionID) utxo.Transaction) (cachedTransaction *generic.CachedObject[utxo.Transaction])

	// CachedTransactionMetadata retrieves the CachedObject representing the named TransactionMetadata. The optional
	// computeIfAbsentCallback can be used to dynamically Construct a non-existing TransactionMetadata.
	CachedTransactionMetadata(transactionID utxo.TransactionID, computeIfAbsentCallback ...func(transactionID utxo.TransactionID) *TransactionMetadata) (cachedTransactionMetadata *generic.CachedObject[*TransactionMetadata])

	// CachedConsumers retrieves the CachedObjects containing the named Consumers.
	CachedConsumers(outputID utxo.OutputID) (cachedConsumers generic.CachedObjects[*Consumer])

	ForEachOutputID(callback func(utxo.OutputID) bool)
}
