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
	"github.com/iotaledger/goshimmer/packages/core/traits"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
)

type UnspentOutputsConsumer interface {
	ApplyCreatedOutput(output *ledger.OutputWithMetadata) (err error)
	ApplySpentOutput(output *ledger.OutputWithMetadata) (err error)
	RollbackCreatedOutput(output *ledger.OutputWithMetadata) (err error)
	RollbackSpentOutput(output *ledger.OutputWithMetadata) (err error)
	BeginBatchedStateTransition(targetEpoch epoch.Index) (currentEpoch epoch.Index, err error)
	CommitBatchedStateTransition() (ctx context.Context)
}

type UnspentOutputs struct {
	IDs *ads.Set[utxo.OutputID, *utxo.OutputID]

	memPool               *ledger.Ledger
	consumers             map[UnspentOutputsConsumer]types.Empty
	consumersMutex        sync.RWMutex
	batchConsumers        map[UnspentOutputsConsumer]types.Empty
	batchCreatedOutputIDs utxo.OutputIDs
	batchSpentOutputIDs   utxo.OutputIDs

	traits.BatchCommittable
	traits.Initializable
}

const (
	PrefixUnspentOutputsLatestCommittedIndex byte = iota
	PrefixUnspentOutputsIDs
)

func NewUnspentOutputs(store func(optRealm ...byte) kvstore.KVStore, memPool *ledger.Ledger) (unspentOutputs *UnspentOutputs) {
	return &UnspentOutputs{
		Initializable:    traits.NewInitializable(),
		BatchCommittable: traits.NewBatchCommittable(store(), PrefixUnspentOutputsLatestCommittedIndex),
		IDs:              ads.NewSet[utxo.OutputID](store(PrefixUnspentOutputsIDs)),
		memPool:          memPool,
		consumers:        make(map[UnspentOutputsConsumer]types.Empty),
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

func (u *UnspentOutputs) Begin(newEpoch epoch.Index) (lastCommittedEpoch epoch.Index, err error) {
	if lastCommittedEpoch, err = u.BeginBatchedStateTransition(newEpoch); err != nil {
		return 0, errors.Errorf("failed to begin batched state transition: %w", err)
	}

	if lastCommittedEpoch == newEpoch {
		return
	}

	u.batchCreatedOutputIDs = utxo.NewOutputIDs()
	u.batchSpentOutputIDs = utxo.NewOutputIDs()
	u.batchConsumers = make(map[UnspentOutputsConsumer]types.Empty)

	if err = u.preparePendingConsumers(lastCommittedEpoch, newEpoch); err != nil {
		return lastCommittedEpoch, errors.Wrap(err, "failed to get pending state diff consumers")
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

func (u *UnspentOutputs) ApplyCreatedOutput(output *ledger.OutputWithMetadata) (err error) {
	var targetConsumers map[UnspentOutputsConsumer]types.Empty
	if !u.BatchedStateTransitionStarted() {
		u.IDs.Add(output.Output().ID())

		u.importOutputIntoMemPoolStorage(output)

		targetConsumers = u.consumers
	} else {
		if !u.batchSpentOutputIDs.Delete(output.Output().ID()) {
			u.batchCreatedOutputIDs.Add(output.Output().ID())
		}

		targetConsumers = u.batchConsumers
	}

	if err = u.notifyConsumers(targetConsumers, output, UnspentOutputsConsumer.ApplyCreatedOutput); err != nil {
		return errors.Errorf("failed to apply created output to consumers: %w", err)
	}

	return
}

func (u *UnspentOutputs) ApplySpentOutput(output *ledger.OutputWithMetadata) (err error) {
	var targetConsumers map[UnspentOutputsConsumer]types.Empty
	if !u.BatchedStateTransitionStarted() {
		panic("cannot apply a spent output without a batched state transition")
	} else {
		if !u.batchCreatedOutputIDs.Delete(output.Output().ID()) {
			u.batchSpentOutputIDs.Add(output.Output().ID())
		}

		targetConsumers = u.batchConsumers
	}

	if err = u.notifyConsumers(targetConsumers, output, UnspentOutputsConsumer.ApplySpentOutput); err != nil {
		return errors.Errorf("failed to apply spent output to consumers: %w", err)
	}

	return
}

func (u *UnspentOutputs) RollbackCreatedOutput(output *ledger.OutputWithMetadata) (err error) {
	return u.ApplySpentOutput(output)
}

func (u *UnspentOutputs) RollbackSpentOutput(output *ledger.OutputWithMetadata) (err error) {
	return u.ApplyCreatedOutput(output)
}

func (u *UnspentOutputs) Export(writer io.WriteSeeker) (err error) {
	if err = stream.WriteCollection(writer, func() (elementsCount uint64, err error) {
		var outputWithMetadata *ledger.OutputWithMetadata
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

func (u *UnspentOutputs) Import(reader io.ReadSeeker, targetEpoch epoch.Index) (err error) {
	outputWithMetadata := new(ledger.OutputWithMetadata)
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

	u.SetLastCommittedEpoch(targetEpoch)

	u.TriggerInitialized()

	return
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
		output := it.Next()
		u.IDs.Add(output)
	}
	for it := u.batchSpentOutputIDs.Iterator(); it.HasNext(); {
		output := it.Next()
		u.IDs.Delete(output)
	}

	waitForConsumers.Wait()

	u.FinalizeBatchedStateTransition()

	done()
}

func (u *UnspentOutputs) preparePendingConsumers(currentEpoch, targetEpoch epoch.Index) (err error) {
	for _, consumer := range u.Consumers() {
		consumerEpoch, err := consumer.BeginBatchedStateTransition(targetEpoch)
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

func (u *UnspentOutputs) notifyConsumers(consumer map[UnspentOutputsConsumer]types.Empty, output *ledger.OutputWithMetadata, callback func(self UnspentOutputsConsumer, output *ledger.OutputWithMetadata) (err error)) (err error) {
	for consumer := range consumer {
		if err = callback(consumer, output); err != nil {
			return errors.Errorf("failed to apply changes to consumer: %w", err)
		}
	}

	return
}

func (u *UnspentOutputs) outputWithMetadata(outputID utxo.OutputID) (outputWithMetadata *ledger.OutputWithMetadata, err error) {
	if !u.memPool.Storage.CachedOutput(outputID).Consume(func(output utxo.Output) {
		if !u.memPool.Storage.CachedOutputMetadata(outputID).Consume(func(metadata *ledger.OutputMetadata) {
			outputWithMetadata = ledger.NewOutputWithMetadata(metadata.InclusionEpoch(), outputID, output, metadata.ConsensusManaPledgeID(), metadata.AccessManaPledgeID())
		}) {
			err = errors.Errorf("failed to load output metadata: %w", err)
		}
	}) {
		err = errors.Errorf("failed to load output %s", outputID)
	}

	return
}

func (u *UnspentOutputs) importOutputIntoMemPoolStorage(output *ledger.OutputWithMetadata) {
	u.memPool.Storage.CachedOutput(output.ID(), func(id utxo.OutputID) utxo.Output { return output.Output() }).Release()
	u.memPool.Storage.CachedOutputMetadata(output.ID(), func(outputID utxo.OutputID) *ledger.OutputMetadata {
		newOutputMetadata := ledger.NewOutputMetadata(output.ID())
		newOutputMetadata.SetAccessManaPledgeID(output.AccessManaPledgeID())
		newOutputMetadata.SetConsensusManaPledgeID(output.ConsensusManaPledgeID())
		newOutputMetadata.SetConfirmationState(confirmation.Confirmed)
		newOutputMetadata.SetInclusionEpoch(output.Index())

		return newOutputMetadata
	}).Release()

	u.memPool.Events.OutputCreated.Trigger(output.ID())
}
