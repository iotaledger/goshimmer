package utxoledger

import (
	"io"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/realitiesledger"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/module"
	"github.com/iotaledger/hive.go/runtime/options"
)

// UTXOLedger represents a ledger using the realities based mempool.
type UTXOLedger struct {
	events         *ledger.Events
	engine         *engine.Engine
	memPool        mempool.MemPool
	unspentOutputs *UnspentOutputs
	stateDiffs     *StateDiffs
	mutex          sync.RWMutex

	optsMemPoolProvider module.Provider[*engine.Engine, mempool.MemPool]

	module.Module
}

func NewProvider(opts ...options.Option[UTXOLedger]) module.Provider[*engine.Engine, ledger.Ledger] {
	return module.Provide(func(e *engine.Engine) ledger.Ledger {
		return options.Apply(&UTXOLedger{
			events:              ledger.NewEvents(),
			engine:              e,
			stateDiffs:          NewStateDiffs(e),
			unspentOutputs:      NewUnspentOutputs(e),
			optsMemPoolProvider: realitiesledger.NewProvider(),
		}, opts, func(l *UTXOLedger) {
			l.memPool = l.optsMemPoolProvider(e)
			l.events.MemPool.LinkTo(l.memPool.Events())

			e.HookConstructed(func() {
				e.Events.Ledger.LinkTo(l.events)
				e.HookStopped(l.memPool.Shutdown)

				l.HookInitialized(l.unspentOutputs.TriggerInitialized)

				l.HookStopped(lo.Batch(
					e.Events.Ledger.MemPool.TransactionAccepted.Hook(l.onTransactionAccepted).Unhook,
					e.Events.Ledger.MemPool.TransactionInclusionUpdated.Hook(l.onTransactionInclusionUpdated).Unhook,
				))
			})

			e.HookStopped(l.TriggerStopped)
		}, (*UTXOLedger).TriggerConstructed)
	})
}

func (l *UTXOLedger) Events() *ledger.Events {
	return l.events
}

func (l *UTXOLedger) MemPool() mempool.MemPool {
	return l.memPool
}

func (l *UTXOLedger) UnspentOutputs() ledger.UnspentOutputs {
	return l.unspentOutputs
}

func (l *UTXOLedger) StateDiffs() ledger.StateDiffs {
	return l.stateDiffs
}

// ApplyStateDiff applies the state diff of the given slot to the ledger state.
func (l *UTXOLedger) ApplyStateDiff(index slot.Index) (err error) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	lastCommittedSlot, err := l.unspentOutputs.Begin(index)
	if err != nil {
		return errors.Wrap(err, "failed to begin unspent outputs")
	} else if lastCommittedSlot == index {
		return
	}

	if err = l.stateDiffs.StreamCreatedOutputs(index, l.unspentOutputs.ApplyCreatedOutput); err != nil {
		return errors.Wrap(err, "failed to apply created outputs")
	}

	if err = l.stateDiffs.StreamSpentOutputs(index, l.unspentOutputs.ApplySpentOutput); err != nil {
		return errors.Wrap(err, "failed to apply spent outputs")
	}

	<-l.unspentOutputs.Commit().Done()

	return
}

// Import imports the ledger state from the given reader.
func (l *UTXOLedger) Import(reader io.ReadSeeker) (err error) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if err = l.unspentOutputs.Import(reader, l.engine.Storage.Settings.LatestCommitment().Index()); err != nil {
		return errors.Wrap(err, "failed to import unspent outputs")
	}

	importedStateDiffs, err := l.stateDiffs.Import(reader)
	if err != nil {
		return errors.Wrap(err, "failed to import state diffs")
	}

	// Apply state diffs backwards.
	if len(importedStateDiffs) != 0 {
		var stateDiffSlot slot.Index
		for _, stateDiffSlot = range importedStateDiffs {
			if err = l.rollbackStateDiff(stateDiffSlot); err != nil {
				return errors.Wrapf(err, "failed to apply state diff %d", stateDiffSlot)
			}

			if err = l.stateDiffs.Delete(stateDiffSlot); err != nil {
				return errors.Wrapf(err, "failed to delete state diff %d", stateDiffSlot)
			}
		}
		stateDiffSlot-- // we rolled back slot n to get to slot n-1

		targetSlotCommitment, errLoad := l.engine.Storage.Commitments.Load(stateDiffSlot)
		if errLoad != nil {
			return errors.Wrapf(errLoad, "failed to load commitment for target slot %d", stateDiffSlot)
		}

		if err = l.engine.Storage.Settings.SetLatestCommitment(targetSlotCommitment); err != nil {
			return errors.Wrap(err, "failed to set latest commitment")
		}

		if err = l.engine.Storage.Settings.SetLatestStateMutationSlot(stateDiffSlot); err != nil {
			return errors.Wrap(err, "failed to set latest state mutation slot")
		}

		if err = l.engine.Storage.Settings.SetLatestConfirmedSlot(stateDiffSlot); err != nil {
			return errors.Wrap(err, "failed to set latest confirmed slot")
		}
	}

	l.TriggerInitialized()

	return
}

// Export exports the ledger state to the given writer.
func (l *UTXOLedger) Export(writer io.WriteSeeker, targetSlot slot.Index) (err error) {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	if err = l.unspentOutputs.Export(writer); err != nil {
		return errors.Wrap(err, "failed to export unspent outputs")
	}

	if err = l.stateDiffs.Export(writer, targetSlot); err != nil {
		return errors.Wrap(err, "failed to export state diffs")
	}

	return
}

// rollbackStateDiff rolls back the named stateDiff index to get to the previous slot.
func (l *UTXOLedger) rollbackStateDiff(index slot.Index) (err error) {
	targetSlot := index - 1
	lastCommittedSlot, err := l.unspentOutputs.Begin(targetSlot)
	if err != nil {
		return errors.Wrap(err, "failed to begin unspent outputs")
	} else if lastCommittedSlot == targetSlot {
		return
	}

	if err = l.stateDiffs.StreamSpentOutputs(lastCommittedSlot, l.unspentOutputs.RollbackSpentOutput); err != nil {
		return errors.Wrap(err, "failed to apply created outputs")
	}

	if err = l.stateDiffs.StreamCreatedOutputs(lastCommittedSlot, l.unspentOutputs.RollbackCreatedOutput); err != nil {
		return errors.Wrap(err, "failed to apply spent outputs")
	}

	<-l.unspentOutputs.Commit().Done()

	return
}

// onTransactionAccepted is triggered when a transaction is accepted by the memPool.
func (l *UTXOLedger) onTransactionAccepted(transactionEvent *mempool.TransactionEvent) {
	if err := l.stateDiffs.addAcceptedTransaction(transactionEvent.Metadata); err != nil {
		// TODO: handle error gracefully
		panic(err)
	}
}

// onTransactionInclusionUpdated is triggered when a transaction inclusion state is updated.
func (l *UTXOLedger) onTransactionInclusionUpdated(inclusionUpdatedEvent *mempool.TransactionInclusionUpdatedEvent) {
	if l.engine.Ledger.MemPool().ConflictDAG().ConfirmationState(inclusionUpdatedEvent.TransactionMetadata.ConflictIDs()).IsAccepted() {
		l.stateDiffs.moveTransactionToOtherSlot(inclusionUpdatedEvent.TransactionMetadata, inclusionUpdatedEvent.PreviousInclusionSlot, inclusionUpdatedEvent.InclusionSlot)
	}
}

var _ ledger.Ledger = new(UTXOLedger)

func WithMemPoolProvider(provider module.Provider[*engine.Engine, mempool.MemPool]) options.Option[UTXOLedger] {
	return func(u *UTXOLedger) {
		u.optsMemPoolProvider = provider
	}
}
