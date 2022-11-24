package ledgerstate

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/types"
	"github.com/iotaledger/hive.go/core/types/confirmation"
	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/storage"
)

type LedgerState struct {
	MemPool          *ledger.Ledger
	StateDiffs       *StateDiffs
	UnspentOutputIDs *UnspentOutputIDs
	consumers        []DiffConsumer
	consumersMutex   sync.RWMutex
}

func New(storageInstance *storage.Storage) (ledgerState *LedgerState) {
	ledgerState = &LedgerState{
		StateDiffs:       NewStateDiffs(storageInstance.LedgerStateDiffs),
		UnspentOutputIDs: NewUnspentOutputIDs(storageInstance.UnspentOutputIDs),
		consumers:        make([]DiffConsumer, 0),
	}

	ledgerState.MemPool.Events.TransactionAccepted.Hook(event.NewClosure(ledgerState.onTransactionAccepted))
	ledgerState.MemPool.Events.TransactionInclusionUpdated.Hook(event.NewClosure(ledgerState.onTransactionInclusionUpdated))
	ledgerState.RegisterConsumer(ledgerState.UnspentOutputIDs)

	return
}

func (l *LedgerState) RegisterConsumer(consumer DiffConsumer) {
	l.consumersMutex.Lock()
	defer l.consumersMutex.Unlock()

	l.consumers = append(l.consumers, consumer)
}

func (l *LedgerState) ImportOutputs(outputs []*OutputWithMetadata) {
	l.consumersMutex.RLock()
	defer l.consumersMutex.RUnlock()

	for _, output := range outputs {
		l.importMemPoolOutput(output)

		for _, consumer := range l.consumers {
			consumer.ProcessCreatedOutput(output)
		}
	}
}

func (l *LedgerState) ApplyStateDiff(targetEpoch epoch.Index) (err error) {
	l.consumersMutex.RLock()
	defer l.consumersMutex.RUnlock()

	for _, consumer := range l.consumers {
		consumer.Begin(targetEpoch)
	}

	if err = l.StateDiffs.StreamCreatedOutputs(targetEpoch, func(output *OutputWithMetadata) {
		l.processConsumers(targetEpoch, output, DiffConsumer.ProcessCreatedOutput, DiffConsumer.ProcessSpentOutput)
	}); err != nil {
		return errors.Errorf("failed to stream created outputs for state diff %d: %w", targetEpoch, err)
	}

	if err = l.StateDiffs.StreamSpentOutputs(targetEpoch, func(output *OutputWithMetadata) {
		l.processConsumers(targetEpoch, output, DiffConsumer.ProcessSpentOutput, DiffConsumer.ProcessCreatedOutput)
	}); err != nil {
		return errors.Errorf("failed to stream created outputs for state diff %d: %w", targetEpoch, err)
	}

	for _, consumer := range l.consumers {
		consumer.Commit()
	}

	return nil
}

func (l *LedgerState) Root() types.Identifier {
	return l.UnspentOutputIDs.Root()
}

func (l *LedgerState) onTransactionAccepted(metadata *ledger.TransactionMetadata) {
	if err := l.StateDiffs.addAcceptedTransaction(metadata); err != nil {
		// TODO: handle error gracefully
		panic(err)
	}
}

func (l *LedgerState) onTransactionInclusionUpdated(event *ledger.TransactionInclusionUpdatedEvent) {
	if l.MemPool.ConflictDAG.ConfirmationState(event.TransactionMetadata.ConflictIDs()).IsAccepted() {
		l.StateDiffs.moveTransactionToOtherEpoch(event.TransactionMetadata, event.PreviousInclusionTime, event.InclusionTime)
	}
}

func (l *LedgerState) importMemPoolOutput(output *OutputWithMetadata) {
	l.MemPool.Storage.CachedOutput(output.ID(), func(id utxo.OutputID) utxo.Output { return output.Output() }).Release()
	l.MemPool.Storage.CachedOutputMetadata(output.ID(), func(outputID utxo.OutputID) *ledger.OutputMetadata {
		newOutputMetadata := ledger.NewOutputMetadata(output.ID())
		newOutputMetadata.SetAccessManaPledgeID(output.AccessManaPledgeID())
		newOutputMetadata.SetConsensusManaPledgeID(output.ConsensusManaPledgeID())
		newOutputMetadata.SetConfirmationState(confirmation.Confirmed)

		return newOutputMetadata
	}).Release()

	l.MemPool.Events.OutputCreated.Trigger(output.ID())
}

func (l *LedgerState) processConsumers(targetEpoch epoch.Index, output *OutputWithMetadata, applyFunc, rollbackFunc func(DiffConsumer, *OutputWithMetadata)) {
	for _, consumer := range l.consumers {
		switch currentEpoch := consumer.LastCommittedEpoch(); {
		case IsApply(currentEpoch, targetEpoch):
			applyFunc(consumer, output)

		case IsRollback(currentEpoch, targetEpoch):
			rollbackFunc(consumer, output)
		}
	}
}
