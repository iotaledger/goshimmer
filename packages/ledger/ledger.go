package ledger

import (
	"context"
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/ledger/branchdag"
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

	// BranchDAG is a reference to the BranchDAG that is used by this Ledger.
	BranchDAG *branchdag.BranchDAG

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

	ledger.BranchDAG = branchdag.New(branchdag.WithStore(ledger.options.store), branchdag.WithCacheTimeProvider(ledger.options.cacheTimeProvider))
	ledger.Storage = newStorage(ledger)
	ledger.validator = newValidator(ledger)
	ledger.booker = newBooker(ledger)
	ledger.dataFlow = newDataFlow(ledger)
	ledger.Utils = newUtils(ledger)

	ledger.BranchDAG.Events.BranchConfirmed.Attach(event.NewClosure(func(events *branchdag.BranchConfirmedEvent) {
		fmt.Println("CONFIRM", events.BranchID.TransactionID())
		ledger.Storage.CachedTransactionMetadata(events.BranchID.TransactionID()).Consume(func(txMetadata *TransactionMetadata) {
			ledger.triggerConfirmedEvent(txMetadata, false)
		})
	}))

	ledger.Events.TransactionBooked.Attach(event.NewClosure(func(event *TransactionBookedEvent) {
		ledger.processConsumingTransactions(event.Outputs.IDs())
	}))

	ledger.Events.TransactionInvalid.Attach(event.NewClosure(func(event *TransactionInvalidEvent) {
		ledger.PruneTransaction(event.TransactionID, true)
	}))

	return ledger
}

// LoadSnapshot loads a snapshot of the Ledger from the given snapshot.
func (l *Ledger) LoadSnapshot(snapshot *Snapshot) {
	for _, output := range snapshot.Outputs {
		l.Storage.CachedOutput(output.ID(), func(outputID utxo.OutputID) *Output {
			return NewOutput(output)
		})

		l.Storage.CachedOutputMetadata(output.ID(), func(outputID utxo.OutputID) *OutputMetadata {
			outputMetadata := NewOutputMetadata(output.ID())
			outputMetadata.SetBranchIDs(branchdag.NewBranchIDs(branchdag.MasterBranchID))
			outputMetadata.SetGradeOfFinality(gof.High)

			return outputMetadata
		})
	}
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

		if previousInclusionTime.IsZero() && l.BranchDAG.InclusionState(txMetadata.BranchIDs()) == branchdag.Confirmed {
			l.triggerConfirmedEvent(txMetadata, false)
		}
	})
}

// CheckTransaction checks the validity of a Transaction.
func (l *Ledger) CheckTransaction(ctx context.Context, tx utxo.Transaction) (err error) {
	return l.dataFlow.checkTransaction().Run(newDataFlowParams(ctx, NewTransaction(tx)))
}

// StoreAndProcessTransaction stores and processes the given Transaction.
func (l *Ledger) StoreAndProcessTransaction(ctx context.Context, tx utxo.Transaction) (err error) {
	l.mutex.Lock(tx.ID())
	defer l.mutex.Unlock(tx.ID())

	return l.dataFlow.storeAndProcessTransaction().Run(newDataFlowParams(ctx, NewTransaction(tx)))
}

// PruneTransaction removes a Transaction from the Ledger (e.g. after it was orphaned or found to be invalid). If the
// pruneFutureCone flag is true, then we do not just remove the named Transaction but also its future cone.
func (l *Ledger) PruneTransaction(txID utxo.TransactionID, pruneFutureCone bool) {
	l.Storage.pruneTransaction(txID, pruneFutureCone)
}

// Shutdown shuts down the stateful elements of the Ledger (the Storage and the BranchDAG).
func (l *Ledger) Shutdown() {
	l.Storage.Shutdown()
	l.BranchDAG.Shutdown()
}

// processTransaction tries to book a single Transaction.
func (l *Ledger) processTransaction(tx *Transaction) (err error) {
	l.mutex.Lock(tx.ID())
	defer l.mutex.Unlock(tx.ID())

	return l.dataFlow.processTransaction().Run(newDataFlowParams(context.Background(), tx))
}

// processConsumingTransactions tries to book the transactions approving the given OutputIDs (it is used to propagate
// the booked status).
func (l *Ledger) processConsumingTransactions(outputIDs utxo.OutputIDs) {
	for it := l.Utils.UnprocessedConsumingTransactions(outputIDs).Iterator(); it.HasNext(); {
		go l.Storage.CachedTransaction(it.Next()).Consume(func(tx *Transaction) {
			_ = l.processTransaction(tx)
		})
	}
}

// triggerConfirmedEvent triggers the TransactionConfirmed event if the Transaction was confirmed.
func (l *Ledger) triggerConfirmedEvent(txMetadata *TransactionMetadata, checkInclusion bool) {
	if checkInclusion && txMetadata.InclusionTime().IsZero() {
		return
	}

	if !txMetadata.SetGradeOfFinality(gof.High) {
		return
	}

	fmt.Println("UPDATE")

	for it := txMetadata.OutputIDs().Iterator(); it.HasNext(); {
		l.Storage.CachedOutputMetadata(it.Next()).Consume(func(outputMetadata *OutputMetadata) {
			outputMetadata.SetGradeOfFinality(gof.High)
		})
	}

	l.Events.TransactionConfirmed.Trigger(&TransactionConfirmedEvent{
		TransactionID: txMetadata.ID(),
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
