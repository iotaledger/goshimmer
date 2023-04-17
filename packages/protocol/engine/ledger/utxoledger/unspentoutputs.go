package utxoledger

import (
	"context"
	"io"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/confirmation"
	"github.com/iotaledger/goshimmer/packages/core/stream"
	"github.com/iotaledger/goshimmer/packages/core/traits"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/hive.go/ads"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/ds/types"
	"github.com/iotaledger/hive.go/runtime/module"
	"github.com/iotaledger/hive.go/runtime/options"
)

type UnspentOutputs struct {
	ids *ads.Set[utxo.OutputID, *utxo.OutputID]

	memPool               mempool.MemPool
	consumers             map[ledger.UnspentOutputsSubscriber]types.Empty
	consumersMutex        sync.RWMutex
	batchConsumers        map[ledger.UnspentOutputsSubscriber]types.Empty
	batchCreatedOutputIDs utxo.OutputIDs
	batchSpentOutputIDs   utxo.OutputIDs

	traits.BatchCommittable
	module.Module
}

const (
	PrefixUnspentOutputsLatestCommittedIndex byte = iota
	PrefixUnspentOutputsIDs
)

func NewUnspentOutputs(e *engine.Engine) (unspentOutputs *UnspentOutputs) {
	return options.Apply(&UnspentOutputs{
		consumers: make(map[ledger.UnspentOutputsSubscriber]types.Empty),
	}, nil, func(u *UnspentOutputs) {
		e.HookConstructed(func() {
			u.BatchCommittable = traits.NewBatchCommittable(e.Storage.UnspentOutputIDs(), PrefixUnspentOutputsLatestCommittedIndex)
			u.ids = ads.NewSet[utxo.OutputID](e.Storage.UnspentOutputIDs(PrefixUnspentOutputsIDs))
			u.memPool = e.Ledger.MemPool()
		})
	})
}

func (u *UnspentOutputs) Subscribe(consumer ledger.UnspentOutputsSubscriber) {
	u.consumersMutex.Lock()
	defer u.consumersMutex.Unlock()

	u.consumers[consumer] = types.Void
}

func (u *UnspentOutputs) Unsubscribe(consumer ledger.UnspentOutputsSubscriber) {
	u.consumersMutex.Lock()
	defer u.consumersMutex.Unlock()

	delete(u.consumers, consumer)
}

func (u *UnspentOutputs) IDs() *ads.Set[utxo.OutputID, *utxo.OutputID] {
	return u.ids
}

func (u *UnspentOutputs) Begin(newSlot slot.Index) (lastCommittedSlot slot.Index, err error) {
	if lastCommittedSlot, err = u.BeginBatchedStateTransition(newSlot); err != nil {
		return 0, errors.Wrap(err, "failed to begin batched state transition")
	}

	if lastCommittedSlot == newSlot {
		return
	}

	u.batchCreatedOutputIDs = utxo.NewOutputIDs()
	u.batchSpentOutputIDs = utxo.NewOutputIDs()
	u.batchConsumers = make(map[ledger.UnspentOutputsSubscriber]types.Empty)

	if err = u.preparePendingConsumers(lastCommittedSlot, newSlot); err != nil {
		return lastCommittedSlot, errors.Wrap(err, "failed to get pending state diff consumers")
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
		}(consumer.CommitBatchedStateTransition())
	}

	ctx, done := context.WithCancel(context.Background())
	go u.applyBatch(&commitDone, done)

	return ctx
}

func (u *UnspentOutputs) ApplyCreatedOutput(output *mempool.OutputWithMetadata) (err error) {
	var targetConsumers map[ledger.UnspentOutputsSubscriber]types.Empty
	if !u.BatchedStateTransitionStarted() {
		u.ids.Add(output.Output().ID())

		u.importOutputIntoMemPoolStorage(output)

		targetConsumers = u.consumers
	} else {
		if !u.batchSpentOutputIDs.Delete(output.Output().ID()) {
			u.batchCreatedOutputIDs.Add(output.Output().ID())
		}

		targetConsumers = u.batchConsumers
	}

	if err = u.notifyConsumers(targetConsumers, output, ledger.UnspentOutputsSubscriber.ApplyCreatedOutput); err != nil {
		return errors.Wrap(err, "failed to apply created output to consumers")
	}

	return
}

func (u *UnspentOutputs) ApplySpentOutput(output *mempool.OutputWithMetadata) (err error) {
	var targetConsumers map[ledger.UnspentOutputsSubscriber]types.Empty
	if !u.BatchedStateTransitionStarted() {
		panic("cannot apply a spent output without a batched state transition")
	} else {
		if !u.batchCreatedOutputIDs.Delete(output.Output().ID()) {
			u.batchSpentOutputIDs.Add(output.Output().ID())
		}

		targetConsumers = u.batchConsumers
	}

	if err = u.notifyConsumers(targetConsumers, output, ledger.UnspentOutputsSubscriber.ApplySpentOutput); err != nil {
		return errors.Wrap(err, "failed to apply spent output to consumers")
	}

	return
}

func (u *UnspentOutputs) RollbackCreatedOutput(output *mempool.OutputWithMetadata) (err error) {
	return u.ApplySpentOutput(output)
}

func (u *UnspentOutputs) RollbackSpentOutput(output *mempool.OutputWithMetadata) (err error) {
	return u.ApplyCreatedOutput(output)
}

func (u *UnspentOutputs) Export(writer io.WriteSeeker) (err error) {
	if err = stream.WriteCollection(writer, func() (elementsCount uint64, err error) {
		var outputWithMetadata *mempool.OutputWithMetadata
		if iterationErr := u.ids.Stream(func(outputID utxo.OutputID) bool {
			if outputWithMetadata, err = u.outputWithMetadata(outputID); err != nil {
				err = errors.Wrap(err, "failed to load output with metadata")
			} else if err = stream.WriteSerializable(writer, outputWithMetadata); err != nil {
				err = errors.Wrap(err, "failed to write output with metadata")
			} else {
				elementsCount++
			}

			return err == nil
		}); iterationErr != nil {
			return 0, errors.Errorf("failed to stream unspent output IDs: %s", iterationErr)
		}

		return
	}); err != nil {
		return errors.Wrap(err, "failed to export unspent outputs")
	}

	return
}

func (u *UnspentOutputs) Import(reader io.ReadSeeker, targetSlot slot.Index) (err error) {
	outputWithMetadata := new(mempool.OutputWithMetadata)
	if err = stream.ReadCollection(reader, func(i int) (err error) {
		if err = stream.ReadSerializable(reader, outputWithMetadata); err != nil {
			return errors.Wrap(err, "failed to read output with metadata")
		} else if err = u.ApplyCreatedOutput(outputWithMetadata); err != nil {
			return errors.Wrap(err, "failed to apply created output")
		}

		return
	}); err != nil {
		return errors.Wrap(err, "failed to import unspent outputs")
	}

	u.SetLastCommittedSlot(targetSlot)

	u.TriggerInitialized()

	return
}

func (u *UnspentOutputs) Consumers() (consumers []ledger.UnspentOutputsSubscriber) {
	u.consumersMutex.RLock()
	defer u.consumersMutex.RUnlock()

	for consumer := range u.consumers {
		consumers = append(consumers, consumer)
	}

	return consumers
}

func (u *UnspentOutputs) applyBatch(waitForConsumers *sync.WaitGroup, done func()) {
	for it := u.batchCreatedOutputIDs.Iterator(); it.HasNext(); {
		output := it.Next()
		u.ids.Add(output)
	}
	for it := u.batchSpentOutputIDs.Iterator(); it.HasNext(); {
		output := it.Next()
		u.ids.Delete(output)
	}

	waitForConsumers.Wait()

	u.FinalizeBatchedStateTransition()

	done()
}

func (u *UnspentOutputs) preparePendingConsumers(currentSlot, targetSlot slot.Index) (err error) {
	for _, consumer := range u.Consumers() {
		consumerSlot, err := consumer.BeginBatchedStateTransition(targetSlot)
		if err != nil {
			return errors.Wrap(err, "failed to start consumer transaction")
		} else if consumerSlot != currentSlot && consumerSlot != targetSlot {
			return errors.Errorf("consumer in unexpected slot: %d", consumerSlot)
		} else if consumerSlot != targetSlot {
			u.batchConsumers[consumer] = types.Void
		}
	}

	return
}

func (u *UnspentOutputs) notifyConsumers(consumer map[ledger.UnspentOutputsSubscriber]types.Empty, output *mempool.OutputWithMetadata, callback func(self ledger.UnspentOutputsSubscriber, output *mempool.OutputWithMetadata) (err error)) (err error) {
	for consumer := range consumer {
		if err = callback(consumer, output); err != nil {
			return errors.Wrap(err, "failed to apply changes to consumer")
		}
	}

	return
}

func (u *UnspentOutputs) outputWithMetadata(outputID utxo.OutputID) (outputWithMetadata *mempool.OutputWithMetadata, err error) {
	if !u.memPool.Storage().CachedOutput(outputID).Consume(func(output utxo.Output) {
		if !u.memPool.Storage().CachedOutputMetadata(outputID).Consume(func(metadata *mempool.OutputMetadata) {
			outputWithMetadata = mempool.NewOutputWithMetadata(metadata.InclusionSlot(), outputID, output, metadata.ConsensusManaPledgeID(), metadata.AccessManaPledgeID())
		}) {
			err = errors.Wrap(err, "failed to load output metadata")
		}
	}) {
		err = errors.Errorf("failed to load output %s", outputID)
	}

	return
}

func (u *UnspentOutputs) importOutputIntoMemPoolStorage(output *mempool.OutputWithMetadata) {
	u.memPool.Storage().CachedOutput(output.ID(), func(id utxo.OutputID) utxo.Output { return output.Output() }).Release()
	u.memPool.Storage().CachedOutputMetadata(output.ID(), func(outputID utxo.OutputID) *mempool.OutputMetadata {
		newOutputMetadata := mempool.NewOutputMetadata(output.ID())
		newOutputMetadata.SetAccessManaPledgeID(output.AccessManaPledgeID())
		newOutputMetadata.SetConsensusManaPledgeID(output.ConsensusManaPledgeID())
		newOutputMetadata.SetConfirmationState(confirmation.Confirmed)
		newOutputMetadata.SetInclusionSlot(output.Index())

		return newOutputMetadata
	}).Release()

	u.memPool.Events().OutputCreated.Trigger(output.ID())
}

var _ ledger.UnspentOutputs = new(UnspentOutputs)
