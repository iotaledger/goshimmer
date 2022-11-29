package ledgerstate

import (
	"context"
	"io"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/types"
	"github.com/iotaledger/hive.go/core/types/confirmation"

	"github.com/iotaledger/goshimmer/packages/core/ads"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/stream"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
)

type UnspentOutputs struct {
	IDs     *ads.Set[utxo.OutputID, *utxo.OutputID]
	Storage *ledger.Storage

	lastCommittedEpoch      epoch.Index
	lastCommittedEpochMutex sync.RWMutex
	batchedConsumers        []DiffConsumer
	batchedEpochIndex       epoch.Index
	batchedEpochMutex       sync.RWMutex
	batchedCreatedOutputIDs utxo.OutputIDs
	batchedSpentOutputIDs   utxo.OutputIDs
	consumers               []DiffConsumer
	consumersMutex          sync.RWMutex
}

func NewUnspentOutputs(unspentOutputIDsStore kvstore.KVStore, storage *ledger.Storage) (unspentOutputs *UnspentOutputs) {
	return &UnspentOutputs{
		IDs:       ads.NewSet[utxo.OutputID](unspentOutputIDsStore),
		Storage:   storage,
		consumers: make([]DiffConsumer, 0),
	}
}

func (u *UnspentOutputs) RegisterConsumer(consumer DiffConsumer) {
	u.consumersMutex.Lock()
	defer u.consumersMutex.Unlock()

	u.consumers = append(u.consumers, consumer)
}

func (u *UnspentOutputs) Begin(committedEpoch epoch.Index) (direction int, err error) {
	u.setBatchedEpoch(committedEpoch)
	u.batchedCreatedOutputIDs = utxo.NewOutputIDs()
	u.batchedSpentOutputIDs = utxo.NewOutputIDs()

	if u.batchedConsumers, direction, err = u.pendingStateDiffConsumers(committedEpoch); err != nil {
		err = errors.Wrap(err, "failed to get pending state diff consumers")
	}

	return
}

func (u *UnspentOutputs) pendingStateDiffConsumers(targetEpoch epoch.Index) (pendingConsumers []DiffConsumer, direction int, err error) {
	for _, consumer := range u.consumers {
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

func (u *UnspentOutputs) Commit() (ctx context.Context) {
	var commitDone sync.WaitGroup

	ctx, done := context.WithCancel(context.Background())
	go func() {
		for it := u.batchedCreatedOutputIDs.Iterator(); it.HasNext(); {
			u.IDs.Add(it.Next())
		}
		for it := u.batchedSpentOutputIDs.Iterator(); it.HasNext(); {
			u.IDs.Delete(it.Next())
		}

		commitDone.Wait()

		u.setLastCommittedEpoch(u.batchedEpoch())
		u.setBatchedEpoch(0)

		done()
	}()

	for _, consumer := range u.batchedConsumers {
		commitDone.Add(1)

		go func(ctx context.Context) {
			select {
			case <-ctx.Done():
				commitDone.Done()
			}
		}(consumer.Commit())
	}

	return ctx
}

func (u *UnspentOutputs) ApplyCreatedOutput(output *OutputWithMetadata) (err error) {
	if u.batchedEpoch() == 0 {
		u.IDs.Add(output.Output().ID())

		if err = u.dispatchConsumers(u.consumers, output, DiffConsumer.ApplyCreatedOutput); err != nil {
			return errors.Errorf("failed to apply created output to batched consumers: %w", err)
		}
	} else if !u.batchedSpentOutputIDs.Delete(output.Output().ID()) {
		u.batchedCreatedOutputIDs.Add(output.Output().ID())

		if err = u.dispatchConsumers(u.batchedConsumers, output, DiffConsumer.ApplyCreatedOutput); err != nil {
			return errors.Errorf("failed to apply created output to batched consumers: %w", err)
		}
	}

	return
}

func (l *UnspentOutputs) Root() types.Identifier {
	return l.IDs.Root()
}

func (u *UnspentOutputs) dispatchConsumers(consumer []DiffConsumer, output *OutputWithMetadata, callback func(self DiffConsumer, output *OutputWithMetadata) (err error)) (err error) {
	for _, consumer := range consumer {
		if err = callback(consumer, output); err != nil {
			return errors.Errorf("failed to apply changes to consumer: %w", err)
		}
	}

	return
}

func (u *UnspentOutputs) ApplySpentOutput(output *OutputWithMetadata) (err error) {
	if u.batchedEpoch() == 0 {
		u.IDs.Delete(output.Output().ID())
	} else if !u.batchedCreatedOutputIDs.Delete(output.Output().ID()) {
		u.batchedSpentOutputIDs.Add(output.Output().ID())
	}

	return
}

func (u *UnspentOutputs) RollbackCreatedOutput(output *OutputWithMetadata) (err error) {
	return u.ApplySpentOutput(output)
}

func (u *UnspentOutputs) RollbackSpentOutput(output *OutputWithMetadata) (err error) {
	return u.ApplyCreatedOutput(output)
}

func (u *UnspentOutputs) LastCommittedEpoch() epoch.Index {
	u.lastCommittedEpochMutex.RLock()
	defer u.lastCommittedEpochMutex.RUnlock()

	return u.lastCommittedEpoch
}

func (u *UnspentOutputs) Export(writer io.WriteSeeker, targetEpoch epoch.Index) (err error) {
	if err = stream.WriteCollection(writer, func() (elementsCount uint64, err error) {
		var outputWithMetadata *OutputWithMetadata
		if iterationErr := u.IDs.Stream(func(outputID utxo.OutputID) bool {
			if outputWithMetadata, err = u.outputWithMetadata(outputID); err != nil {
				err = errors.Errorf("failed to load output with metadata: %w", err)
			} else if err = outputWithMetadata.Export(writer); err != nil {
				err = errors.Errorf("failed to export output with metadata: %w", err)
			} else {
				elementsCount++
			}

			return err == nil
		}); iterationErr != nil {
			return 0, errors.Errorf("failed to stream unspent output IDs: %s", iterationErr)
		}

		return
	}); err != nil {
		return errors.Errorf("failed to export unspent outputs: %w", err)
	}

	return
}

func (u *UnspentOutputs) Import(reader io.ReadSeeker) (err error) {
	outputWithMetadata := new(OutputWithMetadata)
	if err = stream.ReadCollection(reader, func(i int) (err error) {
		if err = outputWithMetadata.Import(reader); err != nil {
			return errors.Errorf("failed to import output with metadata: %w", err)
		} else if err = u.ApplyCreatedOutput(outputWithMetadata); err != nil {
			return errors.Errorf("failed to apply created output: %w", err)
		}

		return
	}); err != nil {
		return errors.Errorf("failed to import unspent outputs: %w", err)
	}

	return
}

func (u *UnspentOutputs) ImportOutput(output *OutputWithMetadata) {
	u.Storage.CachedOutput(output.ID(), func(id utxo.OutputID) utxo.Output { return output.Output() }).Release()
	u.Storage.CachedOutputMetadata(output.ID(), func(outputID utxo.OutputID) *ledger.OutputMetadata {
		newOutputMetadata := ledger.NewOutputMetadata(output.ID())
		newOutputMetadata.SetAccessManaPledgeID(output.AccessManaPledgeID())
		newOutputMetadata.SetConsensusManaPledgeID(output.ConsensusManaPledgeID())
		newOutputMetadata.SetConfirmationState(confirmation.Confirmed)

		return newOutputMetadata
	}).Release()

	// TODO: FIX TRIGGER
	//  u.MemPool.Events.OutputCreated.Trigger(output.ID())
}

func (u *UnspentOutputs) outputWithMetadata(outputID utxo.OutputID) (outputWithMetadata *OutputWithMetadata, err error) {
	if !u.Storage.CachedOutput(outputID).Consume(func(output utxo.Output) {
		if !u.Storage.CachedOutputMetadata(outputID).Consume(func(metadata *ledger.OutputMetadata) {
			outputWithMetadata = NewOutputWithMetadata(epoch.IndexFromTime(metadata.CreationTime()), outputID, output, metadata.ConsensusManaPledgeID(), metadata.AccessManaPledgeID())
		}) {
			err = errors.Errorf("failed to load output metadata: %w", err)
		}
	}) {
		err = errors.Errorf("failed to load output: %w", err)
	}

	return
}

func (u *UnspentOutputs) setLastCommittedEpoch(index epoch.Index) {
	u.lastCommittedEpochMutex.Lock()
	defer u.lastCommittedEpochMutex.Unlock()

	u.lastCommittedEpoch = index
}

func (u *UnspentOutputs) batchedEpoch() epoch.Index {
	u.batchedEpochMutex.RLock()
	defer u.batchedEpochMutex.RUnlock()

	return u.batchedEpochIndex
}

func (u *UnspentOutputs) setBatchedEpoch(index epoch.Index) {
	u.batchedEpochMutex.Lock()
	defer u.batchedEpochMutex.Unlock()

	if index != 0 && u.batchedEpochIndex != 0 {
		panic("a batch is already in progress")
	}

	u.batchedEpochIndex = index
}
