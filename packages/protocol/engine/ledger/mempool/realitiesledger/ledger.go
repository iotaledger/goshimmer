package realitiesledger

import (
	"context"
	"time"

	"github.com/iotaledger/goshimmer/packages/core/confirmation"
	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/storage"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/ds/walker"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/module"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/syncutils"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

// region RealitiesLedger ///////////////////////////////////////////////////////////////////////////////////////////////////////

// RealitiesLedger is an implementation of a "realities-ledger" that manages Outputs according to the principles of the
// quadruple-entry accounting. It acts as a wrapper for the underlying components and exposes the public facing API.
type RealitiesLedger struct {
	// Events is a dictionary for RealitiesLedger related events.
	events *mempool.Events

	// chainStorage is used to access storage for the ledger state.
	chainStorage *storage.Storage

	// Storage is a dictionary for storage related API endpoints.
	storage *Storage

	// Utils is a dictionary for utility methods that simplify the interaction with the RealitiesLedger.
	utils *Utils

	// conflictDAG is a reference to the conflictDAG that is used by this RealitiesLedger.
	conflictDAG *conflictdag.ConflictDAG[utxo.TransactionID, utxo.OutputID]

	workerPool *workerpool.WorkerPool

	// dataFlow is a RealitiesLedger component that defines the data flow (how the different commands are chained together)
	dataFlow *dataFlow

	// validator is a RealitiesLedger component that bundles the API that is used to check the validity of a Transaction
	validator *validator

	// booker is a RealitiesLedger component that bundles the booking related API.
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

	// optConflictDAG contains the optionsLedger for the conflictDAG.
	optConflictDAG []options.Option[conflictdag.ConflictDAG[utxo.TransactionID, utxo.OutputID]]

	// mutex is a DAGMutex that is used to make the RealitiesLedger thread safe.
	mutex *syncutils.DAGMutex[utxo.TransactionID]

	module.Module
}

func NewProvider(opts ...options.Option[RealitiesLedger]) module.Provider[*engine.Engine, mempool.MemPool] {
	return module.Provide(func(e *engine.Engine) mempool.MemPool {
		l := New(opts...)

		e.HookConstructed(func() {
			l.Initialize(e.Workers.CreatePool("MemPool", 2), e.Storage)
		})

		return l
	})
}

func New(opts ...options.Option[RealitiesLedger]) *RealitiesLedger {
	return options.Apply(&RealitiesLedger{
		events:                          mempool.NewEvents(),
		optsCacheTimeProvider:           database.NewCacheTimeProvider(0),
		optsVM:                          new(devnetvm.VM),
		optsTransactionCacheTime:        10 * time.Second,
		optTransactionMetadataCacheTime: 10 * time.Second,
		optsOutputCacheTime:             10 * time.Second,
		optsOutputMetadataCacheTime:     10 * time.Second,
		optsConsumerCacheTime:           10 * time.Second,
		mutex:                           syncutils.NewDAGMutex[utxo.TransactionID](),
	}, opts, func(l *RealitiesLedger) {
		l.conflictDAG = conflictdag.New(l.optConflictDAG...)
		l.events.ConflictDAG.LinkTo(l.conflictDAG.Events)

		l.validator = newValidator(l)
		l.booker = newBooker(l)
		l.dataFlow = newDataFlow(l)
		l.utils = newUtils(l)
	}, (*RealitiesLedger).TriggerConstructed)
}

func (l *RealitiesLedger) Initialize(workerPool *workerpool.WorkerPool, storage *storage.Storage) {
	l.chainStorage = storage
	l.workerPool = workerPool

	l.storage = newStorage(l, l.chainStorage.UnspentOutputs)

	asyncOpt := event.WithWorkerPool(l.workerPool)

	// TODO: revisit whether we should make the process of setting conflict and transaction as accepted/rejected atomic
	l.conflictDAG.Events.ConflictAccepted.Hook(func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
		l.propagateAcceptanceToIncludedTransactions(conflict.ID())
	}, asyncOpt)
	l.conflictDAG.Events.ConflictRejected.Hook(l.propagatedRejectionToTransactions, asyncOpt)
	l.events.TransactionBooked.Hook(func(event *mempool.TransactionBookedEvent) {
		l.processConsumingTransactions(event.Outputs.IDs())
	}, asyncOpt)
	l.events.TransactionInvalid.Hook(func(event *mempool.TransactionInvalidEvent) {
		l.PruneTransaction(event.TransactionID, true)
	}, asyncOpt)

	l.TriggerInitialized()
}

func (l *RealitiesLedger) Events() *mempool.Events {
	return l.events
}

func (l *RealitiesLedger) ConflictDAG() *conflictdag.ConflictDAG[utxo.TransactionID, utxo.OutputID] {
	return l.conflictDAG
}

func (l *RealitiesLedger) Storage() mempool.Storage {
	return l.storage
}

func (l *RealitiesLedger) Utils() mempool.Utils {
	return l.utils
}

func (l *RealitiesLedger) VM() vm.VM {
	return l.optsVM
}

var _ mempool.MemPool = new(RealitiesLedger)

// SetTransactionInclusionSlot sets the inclusion timestamp of a Transaction.
func (l *RealitiesLedger) SetTransactionInclusionSlot(id utxo.TransactionID, inclusionSlot slot.Index) {
	l.storage.CachedTransactionMetadata(id).Consume(func(metadata *mempool.TransactionMetadata) {
		if metadata.InclusionSlot() != 0 && inclusionSlot > metadata.InclusionSlot() {
			return
		}

		for it := metadata.OutputIDs().Iterator(); it.HasNext(); {
			l.storage.CachedOutputMetadata(it.Next()).Consume(func(outputMetadata *mempool.OutputMetadata) {
				outputMetadata.SetInclusionSlot(inclusionSlot)
			})
		}

		_, previousInclusionSlot := metadata.SetInclusionSlot(inclusionSlot)

		if previousInclusionSlot != 0 {
			l.events.TransactionInclusionUpdated.Trigger(&mempool.TransactionInclusionUpdatedEvent{
				TransactionID:         id,
				TransactionMetadata:   metadata,
				InclusionSlot:         inclusionSlot,
				PreviousInclusionSlot: previousInclusionSlot,
			})
		}

		l.propagateAcceptanceToIncludedTransactions(metadata.ID())
	})
}

// CheckTransaction checks the validity of a Transaction.
func (l *RealitiesLedger) CheckTransaction(ctx context.Context, tx utxo.Transaction) (err error) {
	return l.dataFlow.checkTransaction().Run(newDataFlowParams(ctx, tx))
}

// StoreAndProcessTransaction stores and processes the given Transaction.
func (l *RealitiesLedger) StoreAndProcessTransaction(ctx context.Context, tx utxo.Transaction) (err error) {
	l.mutex.Lock(tx.ID())
	defer l.mutex.Unlock(tx.ID())

	return l.dataFlow.storeAndProcessTransaction().Run(newDataFlowParams(ctx, tx))
}

// PruneTransaction removes a Transaction from the RealitiesLedger (e.g. after it was orphaned or found to be invalid). If the
// pruneFutureCone flag is true, then we do not just remove the named Transaction but also its future cone.
func (l *RealitiesLedger) PruneTransaction(txID utxo.TransactionID, pruneFutureCone bool) {
	l.storage.pruneTransaction(txID, pruneFutureCone)
}

// Shutdown shuts down the stateful elements of the RealitiesLedger (the Storage and the conflictDAG).
func (l *RealitiesLedger) Shutdown() {
	l.workerPool.Shutdown()
	l.workerPool.PendingTasksCounter.WaitIsZero()
	l.storage.Shutdown()

	l.TriggerStopped()
}

// processTransaction tries to book a single Transaction.
func (l *RealitiesLedger) processTransaction(tx utxo.Transaction) (err error) {
	l.mutex.Lock(tx.ID())
	defer l.mutex.Unlock(tx.ID())

	return l.dataFlow.processTransaction().Run(newDataFlowParams(context.Background(), tx))
}

// processConsumingTransactions tries to book the transactions approving the given OutputIDs (it is used to propagate
// the booked status).
func (l *RealitiesLedger) processConsumingTransactions(outputIDs utxo.OutputIDs) {
	for it := l.utils.UnprocessedConsumingTransactions(outputIDs).Iterator(); it.HasNext(); {
		txID := it.Next()
		l.storage.CachedTransaction(txID).Consume(func(tx utxo.Transaction) {
			_ = l.processTransaction(tx)
		})
	}
}

// triggerAcceptedEvent triggers the TransactionAccepted event if the Transaction was accepted.
func (l *RealitiesLedger) triggerAcceptedEvent(txMetadata *mempool.TransactionMetadata) (triggered bool) {
	l.mutex.Lock(txMetadata.ID())
	defer l.mutex.Unlock(txMetadata.ID())

	if !l.conflictDAG.ConfirmationState(txMetadata.ConflictIDs()).IsAccepted() {
		return false
	}

	if txMetadata.InclusionSlot() == 0 {
		return false
	}

	// check for acceptance monotonicity
	inputsAccepted := true
	l.storage.CachedTransaction(txMetadata.ID()).Consume(func(tx utxo.Transaction) {
		for it := l.utils.ResolveInputs(tx.Inputs()).Iterator(); it.HasNext(); {
			inputID := it.Next()
			l.storage.CachedOutputMetadata(inputID).Consume(func(outputMetadata *mempool.OutputMetadata) {
				if !outputMetadata.ConfirmationState().IsAccepted() {
					inputsAccepted = false
				}
			})
			if !inputsAccepted {
				return
			}
		}
	})
	if !inputsAccepted {
		return false
	}

	// We skip triggering the event if the transaction was already accepted.
	if !txMetadata.SetConfirmationState(confirmation.Accepted) {
		// ... but if the conflict we are propagating is ourselves, we still want to walk the UTXO future cone.
		return txMetadata.ConflictIDs().Has(txMetadata.ID())
	}

	transactionEvent := &mempool.TransactionEvent{
		Metadata:       txMetadata,
		CreatedOutputs: make([]*mempool.OutputWithMetadata, 0),
		SpentOutputs:   make([]*mempool.OutputWithMetadata, 0),
	}

	for it := txMetadata.OutputIDs().Iterator(); it.HasNext(); {
		outputID := it.Next()
		l.storage.CachedOutputMetadata(outputID).Consume(func(outputMetadata *mempool.OutputMetadata) {
			outputMetadata.SetConfirmationState(confirmation.Accepted)
			outputMetadata.SetInclusionSlot(txMetadata.InclusionSlot())
			l.storage.CachedOutput(outputID).Consume(func(output utxo.Output) {
				transactionEvent.CreatedOutputs = append(transactionEvent.CreatedOutputs, mempool.NewOutputWithMetadata(
					outputMetadata.InclusionSlot(),
					outputID,
					output,
					outputMetadata.ConsensusManaPledgeID(),
					outputMetadata.AccessManaPledgeID(),
				))
			})
		})
	}

	l.storage.CachedTransaction(txMetadata.ID()).Consume(func(tx utxo.Transaction) {
		for it := l.utils.ResolveInputs(tx.Inputs()).Iterator(); it.HasNext(); {
			inputID := it.Next()
			l.storage.CachedOutputMetadata(inputID).Consume(func(outputMetadata *mempool.OutputMetadata) {
				l.storage.CachedOutput(inputID).Consume(func(output utxo.Output) {
					transactionEvent.SpentOutputs = append(transactionEvent.SpentOutputs, mempool.NewOutputWithMetadata(
						outputMetadata.InclusionSlot(),
						inputID,
						output,
						outputMetadata.ConsensusManaPledgeID(),
						outputMetadata.AccessManaPledgeID(),
					))
				})
			})
		}
	})

	l.events.TransactionAccepted.Trigger(transactionEvent)

	return true
}

// triggerRejectedEvent triggers the TransactionRejected event if the Transaction was rejected.
func (l *RealitiesLedger) triggerRejectedEvent(txMetadata *mempool.TransactionMetadata) (triggered bool) {
	if !txMetadata.SetConfirmationState(confirmation.Rejected) {
		return false
	}

	for it := txMetadata.OutputIDs().Iterator(); it.HasNext(); {
		l.storage.CachedOutputMetadata(it.Next()).Consume(func(outputMetadata *mempool.OutputMetadata) {
			outputMetadata.SetConfirmationState(confirmation.Rejected)
			l.events.OutputRejected.Trigger(outputMetadata.ID())
		})
	}

	l.events.TransactionRejected.Trigger(txMetadata)

	return true
}

func (l *RealitiesLedger) triggerRejectedEventLocked(txMetadata *mempool.TransactionMetadata) (triggered bool) {
	l.mutex.Lock(txMetadata.ID())
	defer l.mutex.Unlock(txMetadata.ID())

	return l.triggerRejectedEvent(txMetadata)
}

// propagateAcceptanceToIncludedTransactions propagates confirmations to the included future cone of the given
// Transaction.
func (l *RealitiesLedger) propagateAcceptanceToIncludedTransactions(txID utxo.TransactionID) {
	l.storage.CachedTransactionMetadata(txID).Consume(func(txMetadata *mempool.TransactionMetadata) {
		if !l.triggerAcceptedEvent(txMetadata) {
			return
		}

		l.utils.WalkConsumingTransactionMetadata(txMetadata.OutputIDs(), func(consumingTxMetadata *mempool.TransactionMetadata, walker *walker.Walker[utxo.OutputID]) {
			if !l.triggerAcceptedEvent(consumingTxMetadata) {
				return
			}

			walker.PushAll(consumingTxMetadata.OutputIDs().Slice()...)
		})
	})
}

// propagateConfirmedConflictToIncludedTransactions propagates confirmations to the included future cone of the given
// Transaction.
func (l *RealitiesLedger) propagatedRejectionToTransactions(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
	l.storage.CachedTransactionMetadata(conflict.ID()).Consume(func(txMetadata *mempool.TransactionMetadata) {
		if !l.triggerRejectedEventLocked(txMetadata) {
			return
		}

		l.utils.WalkConsumingTransactionMetadata(txMetadata.OutputIDs(), func(consumingTxMetadata *mempool.TransactionMetadata, walker *walker.Walker[utxo.OutputID]) {
			if !l.triggerRejectedEventLocked(consumingTxMetadata) {
				return
			}

			walker.PushAll(consumingTxMetadata.OutputIDs().Slice()...)
		})
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// WithVM is an Option for the RealitiesLedger that allows to configure which VM is supposed to be used to process transactions.
func WithVM(vm vm.VM) (option options.Option[RealitiesLedger]) {
	return func(l *RealitiesLedger) {
		l.optsVM = vm
	}
}

// WithCacheTimeProvider is an Option for the RealitiesLedger that allows to configure which CacheTimeProvider is supposed to
// be used.
func WithCacheTimeProvider(cacheTimeProvider *database.CacheTimeProvider) (option options.Option[RealitiesLedger]) {
	return func(options *RealitiesLedger) {
		options.optsCacheTimeProvider = cacheTimeProvider
	}
}

// WithTransactionCacheTime is an Option for the RealitiesLedger that allows to configure how long Transaction objects stay
// cached after they have been released.
func WithTransactionCacheTime(transactionCacheTime time.Duration) (option options.Option[RealitiesLedger]) {
	return func(options *RealitiesLedger) {
		options.optsTransactionCacheTime = transactionCacheTime
	}
}

// WithTransactionMetadataCacheTime is an Option for the RealitiesLedger that allows to configure how long TransactionMetadata
// objects stay cached after they have been released.
func WithTransactionMetadataCacheTime(transactionMetadataCacheTime time.Duration) (option options.Option[RealitiesLedger]) {
	return func(options *RealitiesLedger) {
		options.optTransactionMetadataCacheTime = transactionMetadataCacheTime
	}
}

// WithOutputCacheTime is an Option for the RealitiesLedger that allows to configure how long Output objects stay cached after
// they have been released.
func WithOutputCacheTime(outputCacheTime time.Duration) (option options.Option[RealitiesLedger]) {
	return func(options *RealitiesLedger) {
		options.optsOutputCacheTime = outputCacheTime
	}
}

// WithOutputMetadataCacheTime is an Option for the RealitiesLedger that allows to configure how long OutputMetadata objects stay
// cached after they have been released.
func WithOutputMetadataCacheTime(outputMetadataCacheTime time.Duration) (option options.Option[RealitiesLedger]) {
	return func(options *RealitiesLedger) {
		options.optsOutputMetadataCacheTime = outputMetadataCacheTime
	}
}

// WithConsumerCacheTime is an Option for the RealitiesLedger that allows to configure how long Consumer objects stay cached
// after they have been released.
func WithConsumerCacheTime(consumerCacheTime time.Duration) (option options.Option[RealitiesLedger]) {
	return func(options *RealitiesLedger) {
		options.optsConsumerCacheTime = consumerCacheTime
	}
}

// WithConflictDAGOptions is an Option for the RealitiesLedger that allows to configure the optionsLedger for the ConflictDAG.
func WithConflictDAGOptions(conflictDAGOptions ...options.Option[conflictdag.ConflictDAG[utxo.TransactionID, utxo.OutputID]]) (option options.Option[RealitiesLedger]) {
	return func(options *RealitiesLedger) {
		options.optConflictDAG = conflictDAGOptions
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
