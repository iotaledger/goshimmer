package ledger

import (
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/confirmation"
	"github.com/iotaledger/hive.go/core/cerrors"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/ds/set"
	"github.com/iotaledger/hive.go/ds/walker"
	"github.com/iotaledger/hive.go/lo"
	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/protocol/ledger/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
)

// Utils is a Ledger component that bundles utility related API to simplify common interactions with the Ledger.
type Utils struct {
	// ledger contains a reference to the Ledger that created the Utils.
	ledger *Ledger
}

// newUtils returns a new Utils instance for the given Ledger.
func newUtils(ledger *Ledger) *Utils {
	return &Utils{
		ledger: ledger,
	}
}

func (u *Utils) ConflictIDsInFutureCone(conflictIDs utxo.TransactionIDs) (conflictIDsInFutureCone utxo.TransactionIDs) {
	conflictIDsInFutureCone = utxo.NewTransactionIDs()

	for conflictIDWalker := conflictIDs.Iterator(); conflictIDWalker.HasNext(); {
		conflictID := conflictIDWalker.Next()

		conflictIDsInFutureCone.Add(conflictID)

		if u.ledger.ConflictDAG.ConfirmationState(advancedset.NewAdvancedSet(conflictID)).IsAccepted() {
			u.ledger.Storage.CachedTransactionMetadata(conflictID).Consume(func(txMetadata *TransactionMetadata) {
				u.WalkConsumingTransactionMetadata(txMetadata.OutputIDs(), func(txMetadata *TransactionMetadata, walker *walker.Walker[utxo.OutputID]) {
					conflictIDsInFutureCone.AddAll(txMetadata.ConflictIDs())

					walker.PushAll(txMetadata.OutputIDs().Slice()...)
				})
			})
			continue
		}

		u.ledger.ConflictDAG.Utils.ForEachChildConflictID(conflictID, func(childConflictID utxo.TransactionID) {
			conflictIDWalker.Push(childConflictID)
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
		u.ledger.Storage.CachedConsumers(it.Next()).Consume(func(consumer *Consumer) {
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
		u.ledger.Storage.CachedConsumers(futureConeWalker.Next()).Consume(func(consumer *Consumer) {
			if futureConeWalker.WalkStopped() || !seenTransactions.Add(consumer.TransactionID()) {
				return
			}

			callback(consumer.TransactionID(), futureConeWalker)
		})
	}
}

// WalkConsumingTransactionMetadata walks over the transactions that consume the named OutputIDs and calls the callback
// with their corresponding TransactionMetadata.
func (u *Utils) WalkConsumingTransactionMetadata(entryPoints utxo.OutputIDs, callback func(txMetadata *TransactionMetadata, walker *walker.Walker[utxo.OutputID])) {
	u.WalkConsumingTransactionID(entryPoints, func(consumingTxID utxo.TransactionID, walker *walker.Walker[utxo.OutputID]) {
		u.ledger.Storage.CachedTransactionMetadata(consumingTxID).Consume(func(txMetadata *TransactionMetadata) {
			callback(txMetadata, walker)
		})
	})
}

// WithTransactionAndMetadata walks over the transactions that consume the named OutputIDs and calls the callback
// with their corresponding Transaction and TransactionMetadata.
func (u *Utils) WithTransactionAndMetadata(txID utxo.TransactionID, callback func(tx utxo.Transaction, txMetadata *TransactionMetadata)) {
	u.ledger.Storage.CachedTransaction(txID).Consume(func(tx utxo.Transaction) {
		u.ledger.Storage.CachedTransactionMetadata(txID).Consume(func(txMetadata *TransactionMetadata) {
			callback(tx, txMetadata)
		})
	})
}

// TransactionConflictIDs returns the ConflictIDs of the given TransactionID.
func (u *Utils) TransactionConflictIDs(txID utxo.TransactionID) (conflictIDs *advancedset.AdvancedSet[utxo.TransactionID], err error) {
	conflictIDs = advancedset.NewAdvancedSet[utxo.TransactionID]()
	if !u.ledger.Storage.CachedTransactionMetadata(txID).Consume(func(metadata *TransactionMetadata) {
		conflictIDs = metadata.ConflictIDs()
	}) {
		return nil, errors.WithMessagef(cerrors.ErrFatal, "failed to load TransactionMetadata with %s", txID)
	}

	return conflictIDs, nil
}

func (u *Utils) ReferencedTransactions(tx utxo.Transaction) (transactionIDs utxo.TransactionIDs) {
	transactionIDs = utxo.NewTransactionIDs()
	u.ledger.Storage.CachedOutputs(u.ResolveInputs(tx.Inputs())).Consume(func(output utxo.Output) {
		transactionIDs.Add(output.ID().TransactionID)
	})
	return transactionIDs
}

// ConflictingTransactions returns the TransactionIDs that are conflicting with the given Transaction.
func (u *Utils) ConflictingTransactions(transactionID utxo.TransactionID) (conflictingTransactions utxo.TransactionIDs) {
	conflictingTransactions = utxo.NewTransactionIDs()

	u.ledger.ConflictDAG.Storage.CachedConflict(transactionID).Consume(func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
		for it := conflict.ConflictSetIDs().Iterator(); it.HasNext(); {
			u.ledger.ConflictDAG.Storage.CachedConflictMembers(it.Next()).Consume(func(conflictMember *conflictdag.ConflictMember[utxo.OutputID, utxo.TransactionID]) {
				if conflictMember.ConflictID() == transactionID {
					return
				}

				conflictingTransactions.Add(utxo.TransactionID{Identifier: conflictMember.ConflictID().Identifier})
			})
		}
	})

	return
}

// TransactionConfirmationState returns the ConfirmationState of the Transaction with the given TransactionID.
func (u *Utils) TransactionConfirmationState(txID utxo.TransactionID) (confirmationState confirmation.State) {
	u.ledger.Storage.CachedTransactionMetadata(txID).Consume(func(txMetadata *TransactionMetadata) {
		confirmationState = txMetadata.ConfirmationState()
	})
	return
}

// OutputConfirmationState returns the ConfirmationState of the Output.
func (u *Utils) OutputConfirmationState(outputID utxo.OutputID) (confirmationState confirmation.State) {
	u.ledger.Storage.CachedOutputMetadata(outputID).Consume(func(outputMetadata *OutputMetadata) {
		confirmationState = outputMetadata.ConfirmationState()
	})
	return
}

func (u *Utils) ConfirmedConsumer(outputID utxo.OutputID) (consumerID utxo.TransactionID) {
	// default to no consumer, i.e. Genesis
	consumerID = utxo.EmptyTransactionID
	u.ledger.Storage.CachedConsumers(outputID).Consume(func(consumer *Consumer) {
		if consumerID != utxo.EmptyTransactionID {
			return
		}
		u.ledger.Storage.CachedTransactionMetadata(consumer.TransactionID()).Consume(func(metadata *TransactionMetadata) {
			if metadata.ConfirmationState().IsAccepted() {
				consumerID = consumer.TransactionID()
			}
		})
	})
	return
}
