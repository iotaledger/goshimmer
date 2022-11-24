package ledgerstate

import (
	"encoding/binary"
	"io"
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
			consumer.ApplyCreatedOutput(output)
		}
	}
}

func (l *LedgerState) ApplyStateDiff(targetEpoch epoch.Index) (err error) {
	l.consumersMutex.RLock()
	defer l.consumersMutex.RUnlock()

	consumers, direction, err := l.pendingStateDiffConsumers(targetEpoch)
	if err != nil {
		return errors.Errorf("failed to determine pending consumers: %w", err)
	}

	var streamOutputsFunctions []func(epoch.Index, func(*OutputWithMetadata)) error
	var applyOutputsFunctions []func(DiffConsumer, *OutputWithMetadata)
	switch {
	case direction > 0:
		streamOutputsFunctions = append(streamOutputsFunctions, l.StateDiffs.StreamCreatedOutputs, l.StateDiffs.StreamSpentOutputs)
		applyOutputsFunctions = append(applyOutputsFunctions, DiffConsumer.ApplyCreatedOutput, DiffConsumer.ApplySpentOutput)
	case direction < 0:
		streamOutputsFunctions = append(streamOutputsFunctions, l.StateDiffs.StreamSpentOutputs, l.StateDiffs.StreamCreatedOutputs)
		applyOutputsFunctions = append(applyOutputsFunctions, DiffConsumer.RollbackSpentOutput, DiffConsumer.RollbackCreatedOutput)
	}

	for _, consumer := range consumers {
		consumer.Begin(targetEpoch)
	}

	for i, streamOutputs := range streamOutputsFunctions {
		if err = streamOutputs(targetEpoch, func(output *OutputWithMetadata) {
			for _, consumer := range consumers {
				applyOutputsFunctions[i](consumer, output)
			}
		}); err != nil {
			return errors.Errorf("failed to stream outputs for state diff %d: %w", targetEpoch, err)
		}
	}

	for _, consumer := range consumers {
		consumer.Commit()
	}

	return nil
}

func (l *LedgerState) Root() types.Identifier {
	return l.UnspentOutputIDs.Root()
}

func (c *LedgerState) WriteTo(writer io.WriteSeeker) (err error) {
	writer.Seek(8, io.SeekCurrent)

	c.UnspentOutputIDs.Str
	settingsBytes, err := c.settingsModel.Bytes()
	if err != nil {
		return errors.Errorf("failed to convert settings to bytes: %w", err)
	}

	if err = binary.Write(writer, binary.LittleEndian, uint32(len(settingsBytes))); err != nil {
		return errors.Errorf("failed to write settings length: %w", err)
	}

	if err = binary.Write(writer, binary.LittleEndian, settingsBytes); err != nil {
		return errors.Errorf("failed to write settings: %w", err)
	}

	return nil
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

func (l *LedgerState) pendingStateDiffConsumers(targetEpoch epoch.Index) (pendingConsumers []DiffConsumer, direction int, err error) {
	for _, consumer := range l.consumers {
		switch currentEpoch := consumer.LastCommittedEpoch(); {
		case IsApply(currentEpoch, targetEpoch):
			if direction++; direction <= 0 {
				return nil, 0, errors.New("tried to mix apply and rollback consumers")
			}
		case IsRollback(currentEpoch, targetEpoch):
			if direction--; direction >= 0 {
				return nil, 0, errors.New("tried to mix apply and rollback consumers")
			}
		default:
			continue
		}

		pendingConsumers = append(pendingConsumers, consumer)
	}

	return
}
