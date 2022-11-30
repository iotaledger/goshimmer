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

type UnspentOutputsConsumer interface {
	Begin(committedEpoch epoch.Index) (currentEpoch epoch.Index, err error)
	Commit() (ctx context.Context)
	ApplyCreatedOutput(output *OutputWithMetadata) (err error)
	ApplySpentOutput(output *OutputWithMetadata) (err error)
	RollbackCreatedOutput(output *OutputWithMetadata) (err error)
	RollbackSpentOutput(output *OutputWithMetadata) (err error)
}

type UnspentOutputs struct {
	IDs     *ads.Set[utxo.OutputID, *utxo.OutputID]
	Storage *ledger.Storage

	lastCommittedEpochIndex      epoch.Index
	lastCommittedEpochIndexMutex sync.RWMutex
	batchConsumers               map[UnspentOutputsConsumer]types.Empty
	batchEpoch                   epoch.Index
	batchEpochMutex              sync.RWMutex
	batchCreatedOutputIDs        utxo.OutputIDs
	batchSpentOutputIDs          utxo.OutputIDs
	consumers                    map[UnspentOutputsConsumer]types.Empty
	consumersMutex               sync.RWMutex
}

func NewUnspentOutputs(unspentOutputIDsStore kvstore.KVStore, storage *ledger.Storage) (unspentOutputs *UnspentOutputs) {
	return &UnspentOutputs{
		IDs:       ads.NewSet[utxo.OutputID](unspentOutputIDsStore),
		Storage:   storage,
		consumers: make(map[UnspentOutputsConsumer]types.Empty),
	}
}

func (u *UnspentOutputs) Subscribe(consumer UnspentOutputsConsumer) {
	u.consumersMutex.Lock()
	defer u.consumersMutex.Unlock()

	u.consumers[consumer] = types.Void
}

func (u *UnspentOutputs) Unsubscribe(consumer UnspentOutputsConsumer) {
	u.consumersMutex.Lock()
	defer u.consumersMutex.Unlock()

	delete(u.consumers, consumer)
}

func (u *UnspentOutputs) Root() types.Identifier {
	return u.IDs.Root()
}

func (u *UnspentOutputs) Begin(newEpoch epoch.Index) (currentEpoch epoch.Index, err error) {
	if currentEpoch, err = u.setBatchedEpoch(newEpoch); err != nil {
		return 0, errors.Errorf("failed to set batched epoch: %w", err)
	}

	if currentEpoch == newEpoch {
		return
	}

	u.batchCreatedOutputIDs = utxo.NewOutputIDs()
	u.batchSpentOutputIDs = utxo.NewOutputIDs()
	u.batchConsumers = make(map[UnspentOutputsConsumer]types.Empty)

	if err = u.preparePendingConsumers(currentEpoch, newEpoch); err != nil {
		return currentEpoch, errors.Wrap(err, "failed to get pending state diff consumers")
	}

	return
}

func (u *UnspentOutputs) Commit() (ctx context.Context) {
	var commitDone sync.WaitGroup
	commitDone.Add(len(u.batchConsumers))

	for consumer := range u.batchConsumers {
		go func(ctx context.Context) {
			<-ctx.Done()
			commitDone.Done()
		}(consumer.Commit())
	}

	ctx, done := context.WithCancel(context.Background())
	go u.applyBatch(&commitDone, done)

	return ctx
}

func (u *UnspentOutputs) ApplyCreatedOutput(output *OutputWithMetadata) (err error) {
	if u.batchedEpoch() == 0 {
		u.IDs.Add(output.Output().ID())

		if err = u.notifyConsumers(u.consumers, output, UnspentOutputsConsumer.ApplyCreatedOutput); err != nil {
			return errors.Errorf("failed to apply created output to consumers: %w", err)
		}
	} else {
		if !u.batchSpentOutputIDs.Delete(output.Output().ID()) {
			u.batchCreatedOutputIDs.Add(output.Output().ID())
		}

		if err = u.notifyConsumers(u.batchConsumers, output, UnspentOutputsConsumer.ApplyCreatedOutput); err != nil {
			return errors.Errorf("failed to apply created output to batched consumers: %w", err)
		}
	}

	return
}

func (u *UnspentOutputs) ApplySpentOutput(output *OutputWithMetadata) (err error) {
	if u.batchedEpoch() == 0 {
		u.IDs.Delete(output.Output().ID())

		if err = u.notifyConsumers(u.consumers, output, UnspentOutputsConsumer.ApplySpentOutput); err != nil {
			return errors.Errorf("failed to apply spent output to consumers: %w", err)
		}
	} else {
		if !u.batchCreatedOutputIDs.Delete(output.Output().ID()) {
			u.batchSpentOutputIDs.Add(output.Output().ID())
		}

		if err = u.notifyConsumers(u.batchConsumers, output, UnspentOutputsConsumer.ApplySpentOutput); err != nil {
			return errors.Errorf("failed to apply spent output to batched consumers: %w", err)
		}
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
	u.lastCommittedEpochIndexMutex.RLock()
	defer u.lastCommittedEpochIndexMutex.RUnlock()

	return u.lastCommittedEpochIndex
}

func (u *UnspentOutputs) Export(writer io.WriteSeeker) (err error) {
	if err = stream.WriteCollection(writer, func() (elementsCount uint64, err error) {
		var outputWithMetadata *OutputWithMetadata
		if iterationErr := u.IDs.Stream(func(outputID utxo.OutputID) bool {
			if outputWithMetadata, err = u.outputWithMetadata(outputID); err != nil {
				err = errors.Errorf("failed to load output with metadata: %w", err)
			} else if err = stream.WriteSerializable(writer, outputWithMetadata); err != nil {
				err = errors.Errorf("failed to write output with metadata: %w", err)
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
		if err = stream.ReadSerializable(reader, outputWithMetadata); err != nil {
			return errors.Errorf("failed to read output with metadata: %w", err)
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

func (u *UnspentOutputs) Consumers() (consumers []UnspentOutputsConsumer) {
	u.consumersMutex.RLock()
	defer u.consumersMutex.RUnlock()

	for consumer := range u.consumers {
		consumers = append(consumers, consumer)
	}

	return consumers
}

func (u *UnspentOutputs) applyBatch(waitForConsumers *sync.WaitGroup, done func()) {
	for it := u.batchCreatedOutputIDs.Iterator(); it.HasNext(); {
		u.IDs.Add(it.Next())
	}
	for it := u.batchSpentOutputIDs.Iterator(); it.HasNext(); {
		u.IDs.Delete(it.Next())
	}

	waitForConsumers.Wait()

	u.setLastCommittedEpoch(u.batchedEpoch())
	u.setBatchedEpoch(0)

	done()
}

func (u *UnspentOutputs) preparePendingConsumers(currentEpoch, targetEpoch epoch.Index) (err error) {
	for _, consumer := range u.Consumers() {
		consumerEpoch, err := consumer.Begin(targetEpoch)
		if err != nil {
			return errors.Errorf("failed to start consumer transaction: %w", err)
		} else if consumerEpoch != currentEpoch && consumerEpoch != targetEpoch {
			return errors.Errorf("consumer in unexpected epoch: %d", consumerEpoch)
		} else if consumerEpoch != targetEpoch {
			u.batchConsumers[consumer] = types.Void
		}
	}

	return
}

func (u *UnspentOutputs) notifyConsumers(consumer map[UnspentOutputsConsumer]types.Empty, output *OutputWithMetadata, callback func(self UnspentOutputsConsumer, output *OutputWithMetadata) (err error)) (err error) {
	for consumer := range consumer {
		if err = callback(consumer, output); err != nil {
			return errors.Errorf("failed to apply changes to consumer: %w", err)
		}
	}

	return
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

func (u *UnspentOutputs) lastCommittedEpoch() (lastCommittedEpoch epoch.Index) {
	u.lastCommittedEpochIndexMutex.RLock()
	defer u.lastCommittedEpochIndexMutex.RUnlock()

	return u.lastCommittedEpochIndex
}

func (u *UnspentOutputs) setLastCommittedEpoch(index epoch.Index) {
	u.lastCommittedEpochIndexMutex.Lock()
	defer u.lastCommittedEpochIndexMutex.Unlock()

	u.lastCommittedEpochIndex = index
}


func (u *UnspentOutputs) batchedEpoch() epoch.Index {
	u.batchEpochMutex.RLock()
	defer u.batchEpochMutex.RUnlock()

	return u.batchEpoch
}

func (u *UnspentOutputs) setBatchedEpoch(newEpoch epoch.Index) (currentEpoch epoch.Index, err error) {
	u.batchEpochMutex.Lock()
	defer u.batchEpochMutex.Unlock()

	if newEpoch != 0 && u.batchEpoch != 0 {
		return 0, errors.New("batch is already in progress")
	} else if (newEpoch - currentEpoch).Abs() > 1 {
		return 0, errors.New("batches can only be applied in order")
	} else if currentEpoch = u.lastCommittedEpoch(); currentEpoch != newEpoch {
		u.batchEpoch = newEpoch
	}

	return
}
