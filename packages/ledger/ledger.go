package ledger

import (
	"context"
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/generics/walker"
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/hive.go/types/confirmation"

	"github.com/iotaledger/goshimmer/packages/conflictdag"
	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
)

// region Ledger ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Ledger is an implementation of a "realities-ledger" that manages Outputs according to the principles of the
// quadruple-entry accounting. It acts as a wrapper for the underlying components and exposes the public facing API.
type Ledger struct {
	// Events is a dictionary for Ledger related events.
	Events *Events

	// Storage is a dictionary for storage related API endpoints.
	Storage *Storage

	// Utils is a dictionary for utility methods that simplify the interaction with the Ledger.
	Utils *Utils

	// ConflictDAG is a reference to the ConflictDAG that is used by this Ledger.
	ConflictDAG *conflictdag.ConflictDAG[utxo.TransactionID, utxo.OutputID]

	// dataFlow is a Ledger component that defines the data flow (how the different commands are chained together)
	dataFlow *dataFlow

	// validator is a Ledger component that bundles the API that is used to check the validity of a Transaction
	validator *validator

	// booker is a Ledger component that bundles the booking related API.
	booker *booker

	// options is a dictionary for configuration parameters of the Ledger.
	options *options

	// mutex is a DAGMutex that is used to make the Ledger thread safe.
	mutex *syncutils.DAGMutex[utxo.TransactionID]
}

// New returns a new Ledger from the given options.
func New(options ...Option) (ledger *Ledger) {
	ledger = &Ledger{
		Events:  newEvents(),
		options: newOptions(options...),
		mutex:   syncutils.NewDAGMutex[utxo.TransactionID](),
	}

	ledger.ConflictDAG = conflictdag.New[utxo.TransactionID, utxo.OutputID](append([]conflictdag.Option{
		conflictdag.WithStore(ledger.options.store),
		conflictdag.WithCacheTimeProvider(ledger.options.cacheTimeProvider),
	}, ledger.options.conflictDAGOptions...)...)

	ledger.Storage = newStorage(ledger)
	ledger.validator = newValidator(ledger)
	ledger.booker = newBooker(ledger)
	ledger.dataFlow = newDataFlow(ledger)
	ledger.Utils = newUtils(ledger)

	ledger.ConflictDAG.Events.ConflictAccepted.Attach(event.NewClosure(func(event *conflictdag.ConflictAcceptedEvent[utxo.TransactionID]) {
		ledger.propagateAcceptanceToIncludedTransactions(event.ID)
	}))

	ledger.ConflictDAG.Events.ConflictRejected.Attach(event.NewClosure(func(event *conflictdag.ConflictRejectedEvent[utxo.TransactionID]) {
		ledger.propagatedRejectionToTransactions(event.ID)
	}))

	ledger.Events.TransactionBooked.Attach(event.NewClosure(func(event *TransactionBookedEvent) {
		ledger.processConsumingTransactions(event.Outputs.IDs())
	}))

	ledger.Events.TransactionInvalid.Attach(event.NewClosure(func(event *TransactionInvalidEvent) {
		ledger.PruneTransaction(event.TransactionID, true)
	}))

	return ledger
}

func (l *Ledger) LoadOutputWithMetadatas(outputsWithMetadatas []*OutputWithMetadata) {
	for _, outputWithMetadata := range outputsWithMetadatas {
		newOutputMetadata := NewOutputMetadata(outputWithMetadata.ID())
		newOutputMetadata.SetAccessManaPledgeID(outputWithMetadata.AccessManaPledgeID())
		newOutputMetadata.SetConsensusManaPledgeID(outputWithMetadata.ConsensusManaPledgeID())
		newOutputMetadata.SetConfirmationState(confirmation.Confirmed)

		l.Storage.outputStorage.Store(outputWithMetadata.Output()).Release()
		l.Storage.outputMetadataStorage.Store(newOutputMetadata).Release()
	}
}

func (l *Ledger) LoadEpochDiffs(fullEpochIndex, diffEpochIndex epoch.Index, epochDiffs map[epoch.Index]*EpochDiff) error {
	for ei := fullEpochIndex + 1; ei <= diffEpochIndex; ei++ {
		epochdiff, exists := epochDiffs[ei]
		if !exists {
			return fmt.Errorf("epoch diff not found for epoch")
		}

		for _, spent := range epochdiff.Spent() {
			l.Storage.outputStorage.Delete(spent.ID().Bytes())
			l.Storage.outputMetadataStorage.Delete(spent.ID().Bytes())
		}

		for _, created := range epochdiff.Created() {
			outputMetadata := NewOutputMetadata(created.ID())
			outputMetadata.SetAccessManaPledgeID(created.AccessManaPledgeID())
			outputMetadata.SetConsensusManaPledgeID(created.ConsensusManaPledgeID())
			outputMetadata.SetConfirmationState(confirmation.Confirmed)

			l.Storage.outputStorage.Store(created.Output()).Release()
			l.Storage.outputMetadataStorage.Store(outputMetadata).Release()
		}
	}

	return nil
}

// LoadSnapshot loads a snapshot of the Ledger from the given snapshot.
func (l *Ledger) LoadSnapshot(snapshot *Snapshot) {
	for _, outputWithMetadata := range snapshot.OutputsWithMetadata {
		newOutputMetadata := NewOutputMetadata(outputWithMetadata.ID())
		newOutputMetadata.SetAccessManaPledgeID(outputWithMetadata.AccessManaPledgeID())
		newOutputMetadata.SetConsensusManaPledgeID(outputWithMetadata.ConsensusManaPledgeID())
		newOutputMetadata.SetConfirmationState(confirmation.Confirmed)

		l.Storage.outputStorage.Store(outputWithMetadata.Output()).Release()
		l.Storage.outputMetadataStorage.Store(newOutputMetadata).Release()
	}

	for ei := snapshot.FullEpochIndex + 1; ei <= snapshot.DiffEpochIndex; ei++ {
		epochdiff, exists := snapshot.EpochDiffs[ei]
		if !exists {
			panic("epoch diff not found for epoch")
		}

		for _, spent := range epochdiff.Spent() {
			l.Storage.outputStorage.Delete(spent.ID().Bytes())
			l.Storage.outputMetadataStorage.Delete(spent.ID().Bytes())
		}

		for _, created := range epochdiff.Created() {
			outputMetadata := NewOutputMetadata(created.ID())
			outputMetadata.SetAccessManaPledgeID(created.AccessManaPledgeID())
			outputMetadata.SetConsensusManaPledgeID(created.ConsensusManaPledgeID())
			outputMetadata.SetConfirmationState(confirmation.Confirmed)

			l.Storage.outputStorage.Store(created.Output()).Release()
			l.Storage.outputMetadataStorage.Store(outputMetadata).Release()
		}
	}
}

// TakeSnapshot returns a snapshot of the Ledger state.
func (l *Ledger) TakeSnapshot() (snapshot *Snapshot) {
	snapshot = NewSnapshot([]*OutputWithMetadata{})
	l.Storage.outputMetadataStorage.ForEach(func(key []byte, cachedOutputMetadata *objectstorage.CachedObject[*OutputMetadata]) bool {
		cachedOutputMetadata.Consume(func(outputMetadata *OutputMetadata) {
			if outputMetadata.IsSpent() || !l.Utils.OutputConfirmationState(outputMetadata.ID()).IsAccepted() {
				return
			}

			l.Storage.CachedOutput(outputMetadata.ID()).Consume(func(output utxo.Output) {
				outputWithMetadata := NewOutputWithMetadata(output.ID(), output, outputMetadata.CreationTime(), outputMetadata.ConsensusManaPledgeID(), outputMetadata.AccessManaPledgeID())
				snapshot.OutputsWithMetadata = append(snapshot.OutputsWithMetadata, outputWithMetadata)
			})
		})

		return true
	})

	return snapshot
}

// SetTransactionInclusionTime sets the inclusion timestamp of a Transaction.
func (l *Ledger) SetTransactionInclusionTime(txID utxo.TransactionID, inclusionTime time.Time) {
	l.Storage.CachedTransactionMetadata(txID).Consume(func(txMetadata *TransactionMetadata) {
		updated, previousInclusionTime := txMetadata.SetInclusionTime(inclusionTime)
		if !updated {
			return
		}

		l.Events.TransactionInclusionUpdated.Trigger(&TransactionInclusionUpdatedEvent{
			TransactionID:         txID,
			InclusionTime:         inclusionTime,
			PreviousInclusionTime: previousInclusionTime,
		})

		if previousInclusionTime.IsZero() && l.ConflictDAG.ConfirmationState(txMetadata.ConflictIDs()).IsAccepted() {
			l.triggerAcceptedEvent(txMetadata)
		}
	})
}

// CheckTransaction checks the validity of a Transaction.
func (l *Ledger) CheckTransaction(ctx context.Context, tx utxo.Transaction) (err error) {
	return l.dataFlow.checkTransaction().Run(newDataFlowParams(ctx, tx))
}

// StoreAndProcessTransaction stores and processes the given Transaction.
func (l *Ledger) StoreAndProcessTransaction(ctx context.Context, tx utxo.Transaction) (err error) {
	l.mutex.Lock(tx.ID())
	defer l.mutex.Unlock(tx.ID())

	return l.dataFlow.storeAndProcessTransaction().Run(newDataFlowParams(ctx, tx))
}

// PruneTransaction removes a Transaction from the Ledger (e.g. after it was orphaned or found to be invalid). If the
// pruneFutureCone flag is true, then we do not just remove the named Transaction but also its future cone.
func (l *Ledger) PruneTransaction(txID utxo.TransactionID, pruneFutureCone bool) {
	l.Storage.pruneTransaction(txID, pruneFutureCone)
}

func (l *Ledger) ForEachOutputWithMetadata(consumer func(*OutputWithMetadata)) {
	l.Storage.outputMetadataStorage.ForEach(func(key []byte, cachedOutputMetadata *objectstorage.CachedObject[*OutputMetadata]) bool {
		cachedOutputMetadata.Consume(func(outputMetadata *OutputMetadata) {
			if outputMetadata.IsSpent() || !l.Utils.OutputConfirmationState(outputMetadata.ID()).IsAccepted() {
				return
			}

			l.Storage.CachedOutput(outputMetadata.ID()).Consume(func(output utxo.Output) {
				outputWithMetadata := NewOutputWithMetadata(output.ID(), output, outputMetadata.CreationTime(), outputMetadata.ConsensusManaPledgeID(), outputMetadata.AccessManaPledgeID())
				consumer(outputWithMetadata)
			})
		})

		return true
	})
}

// Shutdown shuts down the stateful elements of the Ledger (the Storage and the ConflictDAG).
func (l *Ledger) Shutdown() {
	l.Storage.Shutdown()
	l.ConflictDAG.Shutdown()
}

// processTransaction tries to book a single Transaction.
func (l *Ledger) processTransaction(tx utxo.Transaction) (err error) {
	l.mutex.Lock(tx.ID())
	defer l.mutex.Unlock(tx.ID())

	return l.dataFlow.processTransaction().Run(newDataFlowParams(context.Background(), tx))
}

// processConsumingTransactions tries to book the transactions approving the given OutputIDs (it is used to propagate
// the booked status).
func (l *Ledger) processConsumingTransactions(outputIDs utxo.OutputIDs) {
	for it := l.Utils.UnprocessedConsumingTransactions(outputIDs).Iterator(); it.HasNext(); {
		txID := it.Next()
		event.Loop.Submit(func() {
			l.Storage.CachedTransaction(txID).Consume(func(tx utxo.Transaction) {
				_ = l.processTransaction(tx)
			})
		})
	}
}

// triggerAcceptedEvent triggers the TransactionAccepted event if the Transaction was accepted.
func (l *Ledger) triggerAcceptedEvent(txMetadata *TransactionMetadata) (triggered bool) {
	if txMetadata.InclusionTime().IsZero() {
		return false
	}

	if !txMetadata.SetConfirmationState(confirmation.Accepted) {
		return false
	}

	for it := txMetadata.OutputIDs().Iterator(); it.HasNext(); {
		l.Storage.CachedOutputMetadata(it.Next()).Consume(func(outputMetadata *OutputMetadata) {
			outputMetadata.SetConfirmationState(confirmation.Accepted)
		})
	}

	l.Events.TransactionAccepted.Trigger(&TransactionAcceptedEvent{
		TransactionID: txMetadata.ID(),
	})

	return true
}

// triggerRejectedEvent triggers the TransactionRejected event if the Transaction was rejected.
func (l *Ledger) triggerRejectedEvent(txMetadata *TransactionMetadata) (triggered bool) {
	if !txMetadata.SetConfirmationState(confirmation.Rejected) {
		return false
	}

	for it := txMetadata.OutputIDs().Iterator(); it.HasNext(); {
		l.Storage.CachedOutputMetadata(it.Next()).Consume(func(outputMetadata *OutputMetadata) {
			outputMetadata.SetConfirmationState(confirmation.Rejected)
		})
	}

	l.Events.TransactionRejected.Trigger(&TransactionRejectedEvent{
		TransactionID: txMetadata.ID(),
	})

	return true
}

// propagateAcceptanceToIncludedTransactions propagates confirmations to the included future cone of the given
// Transaction.
func (l *Ledger) propagateAcceptanceToIncludedTransactions(txID utxo.TransactionID) {
	l.Storage.CachedTransactionMetadata(txID).Consume(func(txMetadata *TransactionMetadata) {
		if !l.triggerAcceptedEvent(txMetadata) {
			return
		}

		l.Utils.WalkConsumingTransactionMetadata(txMetadata.OutputIDs(), func(consumingTxMetadata *TransactionMetadata, walker *walker.Walker[utxo.OutputID]) {
			if l.ConflictDAG.ConfirmationState(consumingTxMetadata.ConflictIDs()).IsAccepted() {
				return
			}

			if !l.triggerAcceptedEvent(consumingTxMetadata) {
				return
			}

			walker.PushAll(consumingTxMetadata.OutputIDs().Slice()...)
		})
	})
}

// propagateConfirmedConflictToIncludedTransactions propagates confirmations to the included future cone of the given
// Transaction.
func (l *Ledger) propagatedRejectionToTransactions(txID utxo.TransactionID) {
	l.Storage.CachedTransactionMetadata(txID).Consume(func(txMetadata *TransactionMetadata) {
		if !l.triggerRejectedEvent(txMetadata) {
			return
		}

		l.Utils.WalkConsumingTransactionMetadata(txMetadata.OutputIDs(), func(consumingTxMetadata *TransactionMetadata, walker *walker.Walker[utxo.OutputID]) {
			if !l.triggerRejectedEvent(consumingTxMetadata) {
				return
			}

			walker.PushAll(consumingTxMetadata.OutputIDs().Slice()...)
		})
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
