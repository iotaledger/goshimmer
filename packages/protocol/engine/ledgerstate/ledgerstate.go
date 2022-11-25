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

func (l *LedgerState) RegisterConsumer(consumer DiffConsumer) {
	l.consumersMutex.Lock()
	defer l.consumersMutex.Unlock()

	l.consumers = append(l.consumers, consumer)
}

func (l *LedgerState) ImportOutput(output *OutputWithMetadata) {
	l.MemPool.Storage.CachedOutput(output.ID(), func(id utxo.OutputID) utxo.Output { return output.Output() }).Release()
	l.MemPool.Storage.CachedOutputMetadata(output.ID(), func(outputID utxo.OutputID) *ledger.OutputMetadata {
		newOutputMetadata := ledger.NewOutputMetadata(output.ID())
		newOutputMetadata.SetAccessManaPledgeID(output.AccessManaPledgeID())
		newOutputMetadata.SetConsensusManaPledgeID(output.ConsensusManaPledgeID())
		newOutputMetadata.SetConfirmationState(confirmation.Confirmed)

		return newOutputMetadata
	}).Release()

	l.MemPool.Events.OutputCreated.Trigger(output.ID())

	l.consumersMutex.RLock()
	defer l.consumersMutex.RUnlock()

	for _, consumer := range l.consumers {
		consumer.ApplyCreatedOutput(output)
	}
}

func (l *LedgerState) Import(reader io.ReadSeeker) (err error) {
	var nextOutputSize uint64
	if err = binary.Read(reader, binary.LittleEndian, &nextOutputSize); err != nil {
		return errors.Errorf("failed to read size of first output: %w", err)
	}

	for nextOutputSize != 0 {
		outputBytes := make([]byte, nextOutputSize)
		if err = binary.Read(reader, binary.LittleEndian, &outputBytes); err != nil {
			return errors.Errorf("failed to read output: %w", err)
		}

		output := new(OutputWithMetadata)
		if consumedBytes, parseErr := output.FromBytes(outputBytes); parseErr != nil {
			return errors.Errorf("failed to parse output: %w", parseErr)
		} else if consumedBytes != int(nextOutputSize) {
			return errors.Errorf("failed to parse output: consumed bytes (%d) != expected bytes (%d)", consumedBytes, nextOutputSize)
		}
		l.ImportOutput(output)

		if err = binary.Read(reader, binary.LittleEndian, &nextOutputSize); err != nil {
			return errors.Errorf("failed to read size of next output: %w", err)
		}
	}

	return
}

func (l *LedgerState) Export(writer io.WriteSeeker, targetEpoch epoch.Index) (err error) {
	if iterationErr := l.UnspentOutputIDs.Stream(func(outputID utxo.OutputID) bool {
		if !l.MemPool.Storage.CachedOutput(outputID).Consume(func(output utxo.Output) {
			if !l.MemPool.Storage.CachedOutputMetadata(outputID).Consume(func(outputMetadata *ledger.OutputMetadata) {
				if outputBytes, err := NewOutputWithMetadata(epoch.IndexFromTime(outputMetadata.CreationTime()), outputID, output, outputMetadata.ConsensusManaPledgeID(), outputMetadata.AccessManaPledgeID()).Bytes(); err != nil {
					err = errors.Errorf("failed to serialize output: %w", err)
				} else if binary.Write(writer, binary.LittleEndian, uint64(len(outputBytes))) != nil {
					err = errors.Errorf("failed to write output size: %w", err)
				} else if _, err = writer.Write(outputBytes); err != nil {
					err = errors.Errorf("failed to write output: %w", err)
				}
			}) {
				err = errors.Errorf("failed to load output metadata: %w", err)
			}
		}) {
			err = errors.Errorf("failed to load output: %w", err)
		}

		return err == nil
	}); iterationErr != nil {
		return errors.Errorf("failed to stream unspent output IDs: %w", iterationErr)
	} else if err != nil {
		return err
	} else if err = binary.Write(writer, binary.LittleEndian, uint64(0)); err != nil {
		return errors.Errorf("failed to write end marker of outputs: %w", err)
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
