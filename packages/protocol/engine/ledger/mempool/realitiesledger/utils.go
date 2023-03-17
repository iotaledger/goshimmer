package realitiesledger

import (
	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/cerrors"
	"github.com/iotaledger/goshimmer/packages/core/confirmation"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/ds/set"
	"github.com/iotaledger/hive.go/ds/walker"
	"github.com/iotaledger/hive.go/lo"
)

// Utils is a RealitiesLedger component that bundles utility related API to simplify common interactions with the RealitiesLedger.
type Utils struct {
	// ledger contains a reference to the RealitiesLedger that created the Utils.
	ledger *RealitiesLedger
}

// newUtils returns a new Utils instance for the given RealitiesLedger.
func newUtils(ledger *RealitiesLedger) *Utils {
	return &Utils{
		ledger: ledger,
	}
}

func (u *Utils) ConflictIDsInFutureCone(conflictIDs utxo.TransactionIDs) (conflictIDsInFutureCone utxo.TransactionIDs) {
	conflictIDsInFutureCone = utxo.NewTransactionIDs()

	for conflictIDWalker := conflictIDs.Iterator(); conflictIDWalker.HasNext(); {
		conflictID := conflictIDWalker.Next()

		conflictIDsInFutureCone.Add(conflictID)

		if u.ledger.conflictDAG.ConfirmationState(advancedset.New(conflictID)).IsAccepted() {
			u.ledger.storage.CachedTransactionMetadata(conflictID).Consume(func(txMetadata *mempool.TransactionMetadata) {
				u.WalkConsumingTransactionMetadata(txMetadata.OutputIDs(), func(consumingTxMetadata *mempool.TransactionMetadata, walker *walker.Walker[utxo.OutputID]) {
					u.ledger.mutex.RLock(consumingTxMetadata.ID())
					defer u.ledger.mutex.RUnlock(consumingTxMetadata.ID())

					conflictIDsInFutureCone.AddAll(consumingTxMetadata.ConflictIDs())

					walker.PushAll(consumingTxMetadata.OutputIDs().Slice()...)
				})
			})
			continue
		}

		conflict, exists := u.ledger.conflictDAG.Conflict(conflictID)
		if !exists {
			continue
		}
		_ = conflict.Children().ForEach(func(childConflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) (err error) {
			conflictIDWalker.Push(childConflict.ID())
			return nil
		})
	}

	return conflictIDsInFutureCone
}

// ResolveInputs returns the OutputIDs that were referenced by the given Inputs.
func (u *Utils) ResolveInputs(inputs []utxo.Input) (outputIDs utxo.OutputIDs) {
	return utxo.NewOutputIDs(lo.Map(inputs, u.ledger.optsVM.ResolveInput)...)
}

// UnprocessedConsumingTransactions returns the unprocessed consuming transactions of the named OutputIDs.
func (u *Utils) UnprocessedConsumingTransactions(outputIDs utxo.OutputIDs) (consumingTransactions utxo.TransactionIDs) {
	consumingTransactions = utxo.NewTransactionIDs()
	for it := outputIDs.Iterator(); it.HasNext(); {
		u.ledger.storage.CachedConsumers(it.Next()).Consume(func(consumer *mempool.Consumer) {
			if consumer.IsBooked() {
				return
			}

			consumingTransactions.Add(consumer.TransactionID())
		})
	}

	return consumingTransactions
}

// WalkConsumingTransactionID walks over the TransactionIDs that consume the named OutputIDs.
func (u *Utils) WalkConsumingTransactionID(entryPoints utxo.OutputIDs, callback func(consumingTxID utxo.TransactionID, walker *walker.Walker[utxo.OutputID])) {
	if entryPoints.Size() == 0 {
		return
	}

	seenTransactions := set.New[utxo.TransactionID](false)
	futureConeWalker := walker.New[utxo.OutputID](false).PushAll(entryPoints.Slice()...)
	for futureConeWalker.HasNext() {
		u.ledger.storage.CachedConsumers(futureConeWalker.Next()).Consume(func(consumer *mempool.Consumer) {
			if futureConeWalker.WalkStopped() || !seenTransactions.Add(consumer.TransactionID()) {
				return
			}

			callback(consumer.TransactionID(), futureConeWalker)
		})
	}
}

// WalkConsumingTransactionMetadata walks over the transactions that consume the named OutputIDs and calls the callback
// with their corresponding TransactionMetadata.
func (u *Utils) WalkConsumingTransactionMetadata(entryPoints utxo.OutputIDs, callback func(txMetadata *mempool.TransactionMetadata, walker *walker.Walker[utxo.OutputID])) {
	u.WalkConsumingTransactionID(entryPoints, func(consumingTxID utxo.TransactionID, walker *walker.Walker[utxo.OutputID]) {
		u.ledger.storage.CachedTransactionMetadata(consumingTxID).Consume(func(txMetadata *mempool.TransactionMetadata) {
			callback(txMetadata, walker)
		})
	})
}

// WithTransactionAndMetadata walks over the transactions that consume the named OutputIDs and calls the callback
// with their corresponding Transaction and TransactionMetadata.
func (u *Utils) WithTransactionAndMetadata(txID utxo.TransactionID, callback func(tx utxo.Transaction, txMetadata *mempool.TransactionMetadata)) {
	u.ledger.storage.CachedTransaction(txID).Consume(func(tx utxo.Transaction) {
		u.ledger.storage.CachedTransactionMetadata(txID).Consume(func(txMetadata *mempool.TransactionMetadata) {
			callback(tx, txMetadata)
		})
	})
}

// TransactionConflictIDs returns the ConflictIDs of the given TransactionID.
func (u *Utils) TransactionConflictIDs(txID utxo.TransactionID) (conflictIDs *advancedset.AdvancedSet[utxo.TransactionID], err error) {
	conflictIDs = advancedset.New[utxo.TransactionID]()
	if !u.ledger.storage.CachedTransactionMetadata(txID).Consume(func(metadata *mempool.TransactionMetadata) {
		conflictIDs = metadata.ConflictIDs()
	}) {
		return nil, errors.WithMessagef(cerrors.ErrFatal, "failed to load TransactionMetadata with %s", txID)
	}

	return conflictIDs, nil
}

func (u *Utils) ReferencedTransactions(tx utxo.Transaction) (transactionIDs utxo.TransactionIDs) {
	transactionIDs = utxo.NewTransactionIDs()
	u.ledger.storage.CachedOutputs(u.ResolveInputs(tx.Inputs())).Consume(func(output utxo.Output) {
		transactionIDs.Add(output.ID().TransactionID)
	})
	return transactionIDs
}

// ConflictingTransactions returns the TransactionIDs that are conflicting with the given Transaction.
func (u *Utils) ConflictingTransactions(transactionID utxo.TransactionID) (conflictingTransactions utxo.TransactionIDs) {
	conflictingTransactions = utxo.NewTransactionIDs()

	conflict, exists := u.ledger.conflictDAG.Conflict(transactionID)
	if !exists {
		return conflictingTransactions
	}

	conflict.ForEachConflictingConflict(func(conflictingConflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) bool {
		conflictingTransactions.Add(conflictingConflict.ID())
		return true
	})

	return conflictingTransactions
}

// TransactionConfirmationState returns the ConfirmationState of the Transaction with the given TransactionID.
func (u *Utils) TransactionConfirmationState(txID utxo.TransactionID) (confirmationState confirmation.State) {
	u.ledger.storage.CachedTransactionMetadata(txID).Consume(func(txMetadata *mempool.TransactionMetadata) {
		confirmationState = txMetadata.ConfirmationState()
	})
	return
}

// OutputConfirmationState returns the ConfirmationState of the Output.
func (u *Utils) OutputConfirmationState(outputID utxo.OutputID) (confirmationState confirmation.State) {
	u.ledger.storage.CachedOutputMetadata(outputID).Consume(func(outputMetadata *mempool.OutputMetadata) {
		confirmationState = outputMetadata.ConfirmationState()
	})
	return
}

func (u *Utils) ConfirmedConsumer(outputID utxo.OutputID) (consumerID utxo.TransactionID) {
	// default to no consumer, i.e. Genesis
	consumerID = utxo.EmptyTransactionID
	u.ledger.storage.CachedConsumers(outputID).Consume(func(consumer *mempool.Consumer) {
		if consumerID != utxo.EmptyTransactionID {
			return
		}
		u.ledger.storage.CachedTransactionMetadata(consumer.TransactionID()).Consume(func(metadata *mempool.TransactionMetadata) {
			if metadata.ConfirmationState().IsAccepted() {
				consumerID = consumer.TransactionID()
			}
		})
	})
	return
}
