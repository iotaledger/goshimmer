package ledger

import (
	"context"
	"time"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/walker"
	"github.com/iotaledger/hive.go/core/syncutils"
	"github.com/iotaledger/hive.go/core/types/confirmation"

	"github.com/iotaledger/goshimmer/packages/core/chainstorage"
	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm"
)

// region Ledger ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Ledger is an implementation of a "realities-ledger" that manages Outputs according to the principles of the
// quadruple-entry accounting. It acts as a wrapper for the underlying components and exposes the public facing API.
type Ledger struct {
	// Events is a dictionary for Ledger related events.
	Events *Events

	// ChainStorage is used to access storage for the ledger state.
	ChainStorage *chainstorage.ChainStorage

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

	// optsVM contains the virtual machine that is used to execute Transactions.
	optsVM vm.VM

	// optsCacheTimeProvider contains the CacheTimeProvider that overrides the local cache times.
	optsCacheTimeProvider *database.CacheTimeProvider

	// optsTransactionCacheTime contains the duration that Transaction objects stay cached after they have been
	// released.
	optsTransactionCacheTime time.Duration

	// optTransactionMetadataCacheTime contains the duration that TransactionMetadata objects stay cached after they
	// have been released.
	optTransactionMetadataCacheTime time.Duration

	// optsOutputCacheTime contains the duration that Output objects stay cached after they have been released.
	optsOutputCacheTime time.Duration

	// optsOutputMetadataCacheTime contains the duration that OutputMetadata objects stay cached after they have been
	// released.
	optsOutputMetadataCacheTime time.Duration

	// optsConsumerCacheTime contains the duration that Consumer objects stay cached after they have been released.
	optsConsumerCacheTime time.Duration

	// optConflictDAG contains the optionsLedger for the ConflictDAG.
	optConflictDAG []conflictdag.Option

	// mutex is a DAGMutex that is used to make the Ledger thread safe.
	mutex *syncutils.DAGMutex[utxo.TransactionID]
}

// New returns a new Ledger from the given optionsLedger.
func New(chainStorage *chainstorage.ChainStorage, opts ...options.Option[Ledger]) (ledger *Ledger) {
	ledger = options.Apply(&Ledger{
		Events:       NewEvents(),
		ChainStorage: chainStorage,

		optsCacheTimeProvider:           database.NewCacheTimeProvider(0),
		optsVM:                          NewMockedVM(),
		optsTransactionCacheTime:        10 * time.Second,
		optTransactionMetadataCacheTime: 10 * time.Second,
		optsOutputCacheTime:             10 * time.Second,
		optsOutputMetadataCacheTime:     10 * time.Second,
		optsConsumerCacheTime:           10 * time.Second,
		mutex:                           syncutils.NewDAGMutex[utxo.TransactionID](),
	}, opts)

	ledger.ConflictDAG = conflictdag.New[utxo.TransactionID, utxo.OutputID](append([]conflictdag.Option{
		conflictdag.WithStore(chainStorage.LedgerstateStorage()),
		conflictdag.WithCacheTimeProvider(ledger.optsCacheTimeProvider),
	}, ledger.optConflictDAG...)...)

	ledger.Events.ConflictDAG = ledger.ConflictDAG.Events

	ledger.Storage = newStorage(ledger, chainStorage.LedgerstateStorage())
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

// LoadOutputsWithMetadata loads OutputWithMetadata from a snapshot file to the storage.
func (l *Ledger) LoadOutputsWithMetadata(outputsWithMetadata []*OutputWithMetadata) {
	for _, outputWithMetadata := range outputsWithMetadata {
		newOutputMetadata := NewOutputMetadata(outputWithMetadata.ID())
		newOutputMetadata.SetAccessManaPledgeID(outputWithMetadata.AccessManaPledgeID())
		newOutputMetadata.SetConsensusManaPledgeID(outputWithMetadata.ConsensusManaPledgeID())
		newOutputMetadata.SetConfirmationState(confirmation.Confirmed)

		l.Storage.outputStorage.Store(outputWithMetadata.Output()).Release()
		l.Storage.outputMetadataStorage.Store(newOutputMetadata).Release()

		l.Events.OutputCreated.Trigger(outputWithMetadata.ID())
	}
}

// LoadEpochDiff loads EpochDiff from a snapshot file to the storage.
func (l *Ledger) LoadEpochDiff(epochDiff *EpochDiff) error {
	for _, spent := range epochDiff.Spent() {
		l.Storage.outputStorage.Delete(lo.PanicOnErr(spent.ID().Bytes()))
		l.Storage.outputMetadataStorage.Delete(lo.PanicOnErr(spent.ID().Bytes()))

		l.Events.OutputSpent.Trigger(spent.ID())
	}

	for _, created := range epochDiff.Created() {
		outputMetadata := NewOutputMetadata(created.ID())
		outputMetadata.SetAccessManaPledgeID(created.AccessManaPledgeID())
		outputMetadata.SetConsensusManaPledgeID(created.ConsensusManaPledgeID())
		outputMetadata.SetConfirmationState(confirmation.Confirmed)

		l.Storage.outputStorage.Store(created.Output()).Release()
		l.Storage.outputMetadataStorage.Store(outputMetadata).Release()

		l.Events.OutputCreated.Trigger(created.ID())
	}

	return nil
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

	l.Storage.CachedTransaction(txMetadata.ID()).Consume(func(tx utxo.Transaction) {
		for it := l.Utils.ResolveInputs(tx.Inputs()).Iterator(); it.HasNext(); {
			l.Events.OutputSpent.Trigger(it.Next())
		}
	})

	l.storeEpochDiff(txMetadata)

	l.Events.TransactionAccepted.Trigger(&TransactionAcceptedEvent{
		TransactionMetadata: txMetadata,
	})

	return true
}

func (l *Ledger) storeTransactionInEpochDiff(txMeta *TransactionMetadata) {
	/*
		1) check inputs against epochdiff mapped outputs -> mark them as spent in storage
			-> remove them from the map
		2) create new outputs and add them to map & epochdiff storage
		3)
	*/

	txEpoch := epoch.IndexFromTime(txMeta.InclusionTime())
	diffStorage := l.ChainStorage.LedgerDiffStorage(txEpoch)
	diffStorage
	diffStorage.Store(txMeta)

	l.Storage.CachedTransaction(txMeta.ID()).Consume(func(tx utxo.Transaction) {
		for it := l.Utils.ResolveInputs(tx.Inputs()).Iterator(); it.HasNext(); {
			input := it.Next()
		}
	})
}

// triggerRejectedEvent triggers the TransactionRejected event if the Transaction was rejected.
func (l *Ledger) triggerRejectedEvent(txMetadata *TransactionMetadata) (triggered bool) {
	if !txMetadata.SetConfirmationState(confirmation.Rejected) {
		return false
	}

	for it := txMetadata.OutputIDs().Iterator(); it.HasNext(); {
		l.Storage.CachedOutputMetadata(it.Next()).Consume(func(outputMetadata *OutputMetadata) {
			outputMetadata.SetConfirmationState(confirmation.Rejected)
			l.Events.OutputRejected.Trigger(outputMetadata.ID())
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

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// WithVM is an Option for the Ledger that allows to configure which VM is supposed to be used to process transactions.
func WithVM(vm vm.VM) (option options.Option[Ledger]) {
	return func(l *Ledger) {
		l.optsVM = vm
	}
}

// WithCacheTimeProvider is an Option for the Ledger that allows to configure which CacheTimeProvider is supposed to
// be used.
func WithCacheTimeProvider(cacheTimeProvider *database.CacheTimeProvider) (option options.Option[Ledger]) {
	return func(options *Ledger) {
		options.optsCacheTimeProvider = cacheTimeProvider
	}
}

// WithTransactionCacheTime is an Option for the Ledger that allows to configure how long Transaction objects stay
// cached after they have been released.
func WithTransactionCacheTime(transactionCacheTime time.Duration) (option options.Option[Ledger]) {
	return func(options *Ledger) {
		options.optsTransactionCacheTime = transactionCacheTime
	}
}

// WithTransactionMetadataCacheTime is an Option for the Ledger that allows to configure how long TransactionMetadata
// objects stay cached after they have been released.
func WithTransactionMetadataCacheTime(transactionMetadataCacheTime time.Duration) (option options.Option[Ledger]) {
	return func(options *Ledger) {
		options.optTransactionMetadataCacheTime = transactionMetadataCacheTime
	}
}

// WithOutputCacheTime is an Option for the Ledger that allows to configure how long Output objects stay cached after
// they have been released.
func WithOutputCacheTime(outputCacheTime time.Duration) (option options.Option[Ledger]) {
	return func(options *Ledger) {
		options.optsOutputCacheTime = outputCacheTime
	}
}

// WithOutputMetadataCacheTime is an Option for the Ledger that allows to configure how long OutputMetadata objects stay
// cached after they have been released.
func WithOutputMetadataCacheTime(outputMetadataCacheTime time.Duration) (option options.Option[Ledger]) {
	return func(options *Ledger) {
		options.optsOutputMetadataCacheTime = outputMetadataCacheTime
	}
}

// WithConsumerCacheTime is an Option for the Ledger that allows to configure how long Consumer objects stay cached
// after they have been released.
func WithConsumerCacheTime(consumerCacheTime time.Duration) (option options.Option[Ledger]) {
	return func(options *Ledger) {
		options.optsConsumerCacheTime = consumerCacheTime
	}
}

// WithConflictDAGOptions is an Option for the Ledger that allows to configure the optionsLedger for the ConflictDAG
func WithConflictDAGOptions(conflictDAGOptions ...conflictdag.Option) (option options.Option[Ledger]) {
	return func(options *Ledger) {
		options.optConflictDAG = conflictDAGOptions
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
