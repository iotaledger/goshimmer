package ledger

import (
	"context"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/walker"
	"github.com/iotaledger/hive.go/core/syncutils"
	"github.com/iotaledger/hive.go/core/types/confirmation"

	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm"
	"github.com/iotaledger/goshimmer/packages/storage"
	"github.com/iotaledger/goshimmer/packages/storage/models"
)

// region Ledger ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Ledger is an implementation of a "realities-ledger" that manages Outputs according to the principles of the
// quadruple-entry accounting. It acts as a wrapper for the underlying components and exposes the public facing API.
type Ledger struct {
	// Events is a dictionary for Ledger related events.
	Events *Events

	// ChainStorage is used to access storage for the ledger state.
	ChainStorage *storage.Storage

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
func New(chainStorage *storage.Storage, opts ...options.Option[Ledger]) (ledger *Ledger) {
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
		conflictdag.WithStore(chainStorage.UnspentOutputs),
		conflictdag.WithCacheTimeProvider(ledger.optsCacheTimeProvider),
	}, ledger.optConflictDAG...)...)

	ledger.Events.ConflictDAG = ledger.ConflictDAG.Events

	ledger.Storage = newStorage(ledger, chainStorage.UnspentOutputs)
	ledger.validator = newValidator(ledger)
	ledger.booker = newBooker(ledger)
	ledger.dataFlow = newDataFlow(ledger)
	ledger.Utils = newUtils(ledger)

	ledger.ConflictDAG.Events.ConflictAccepted.Attach(event.NewClosure(ledger.propagateAcceptanceToIncludedTransactions))

	ledger.ConflictDAG.Events.ConflictRejected.Attach(event.NewClosure(ledger.propagatedRejectionToTransactions))

	ledger.Events.TransactionBooked.Attach(event.NewClosure(func(event *TransactionBookedEvent) {
		ledger.processConsumingTransactions(event.Outputs.IDs())
	}))

	ledger.Events.TransactionInvalid.Attach(event.NewClosure(func(event *TransactionInvalidEvent) {
		ledger.PruneTransaction(event.TransactionID, true)
	}))

	return ledger
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

		if l.ConflictDAG.ConfirmationState(txMetadata.ConflictIDs()).IsAccepted() {
			l.rollbackTransactionInEpochDiff(txMetadata, previousInclusionTime, inclusionTime)

			if previousInclusionTime.IsZero() {
				l.triggerAcceptedEvent(txMetadata)
			}
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
			inputID := it.Next()
			l.Events.OutputSpent.Trigger(inputID)
			// TODO: inputs should be marked as deleted or spent
			// l.Storage.outputStorage.Delete(lo.PanicOnErr(inputID.Bytes()))
		}
	})

	l.storeTransactionInEpochDiff(txMetadata)

	l.Events.TransactionAccepted.Trigger(txMetadata)

	return true
}

func (l *Ledger) storeTransactionInEpochDiff(txMeta *TransactionMetadata) {
	txEpoch := epoch.IndexFromTime(txMeta.InclusionTime())

	l.Storage.CachedTransaction(txMeta.ID()).Consume(func(tx utxo.Transaction) {
		// Mark every input as a spent output in the epoch diff
		for it := l.Utils.ResolveInputs(tx.Inputs()).Iterator(); it.HasNext(); {
			inputID := it.Next()
			l.Storage.CachedOutput(inputID).Consume(func(output utxo.Output) {
				l.Storage.CachedOutputMetadata(inputID).Consume(func(outputMetadata *OutputMetadata) {
					if err := l.storeOutputInDiff(txEpoch, inputID, output, outputMetadata, l.ChainStorage.LedgerStateDiffs.StoreSpentOutput); err != nil {
						l.Events.Error.Trigger(errors.Errorf("could not store spent output in diff: %w", err))
					}
				})
			})
		}

		// Mark every output as created output in the epoch diff
		for it := txMeta.OutputIDs().Iterator(); it.HasNext(); {
			outputID := it.Next()
			l.Storage.CachedOutput(outputID).Consume(func(output utxo.Output) {
				l.Storage.CachedOutputMetadata(outputID).Consume(func(outputMetadata *OutputMetadata) {
					if err := l.storeOutputInDiff(txEpoch, outputID, output, outputMetadata, l.ChainStorage.LedgerStateDiffs.StoreCreatedOutput); err != nil {
						l.Events.Error.Trigger(errors.Errorf("could not store created output in diff: %w", err))
					}
				})
			})
		}

		if txEpoch > l.ChainStorage.Settings.LatestStateMutationEpoch() {
			if err := l.ChainStorage.Settings.SetLatestStateMutationEpoch(txEpoch); err != nil {
				l.Events.Error.Trigger(errors.Errorf("failed to update latest state mutation epoch: %w", err))
			}
		}
	})
}

func (l *Ledger) rollbackTransactionInEpochDiff(txMeta *TransactionMetadata, previousInclusionTime, inclusionTime time.Time) {
	oldEpoch := epoch.IndexFromTime(previousInclusionTime)
	newEpoch := epoch.IndexFromTime(inclusionTime)

	if oldEpoch == 0 || oldEpoch == newEpoch {
		return
	}

	if oldEpoch <= l.ChainStorage.Settings.LatestCommitment().Index() || newEpoch <= l.ChainStorage.Settings.LatestCommitment().Index() {
		l.Events.Error.Trigger(errors.Errorf("inclusion time of transaction changed for already committed epoch: previous Index %d, new Index %d", oldEpoch, newEpoch))
		return
	}

	l.Storage.CachedTransaction(txMeta.ID()).Consume(func(tx utxo.Transaction) {
		l.ChainStorage.LedgerStateDiffs.DeleteSpentOutputs(oldEpoch, l.Utils.ResolveInputs(tx.Inputs()))
	})

	l.ChainStorage.LedgerStateDiffs.DeleteCreatedOutputs(oldEpoch, txMeta.OutputIDs())
}

func (l *Ledger) storeOutputInDiff(txEpoch epoch.Index, outputID utxo.OutputID, output utxo.Output, outputMetadata *OutputMetadata, storeFunc func(*models.OutputWithMetadata) error) (err error) {
	outputWithMetadata := models.NewOutputWithMetadata(
		txEpoch, outputID, output, outputMetadata.CreationTime(),
		outputMetadata.ConsensusManaPledgeID(),
		outputMetadata.AccessManaPledgeID(),
	)
	return storeFunc(outputWithMetadata)
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

	l.Events.TransactionRejected.Trigger(txMetadata)

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
