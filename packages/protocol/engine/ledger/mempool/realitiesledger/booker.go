package realitiesledger

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/cerrors"
	"github.com/iotaledger/goshimmer/packages/core/confirmation"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
	"github.com/iotaledger/hive.go/core/dataflow"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/ds/walker"
	"github.com/iotaledger/hive.go/lo"
)

// booker is a RealitiesLedger component that bundles the booking related API.
type booker struct {
	// ledger contains a reference to the RealitiesLedger that created the booker.
	ledger *RealitiesLedger
}

// newBooker returns a new booker instance for the given RealitiesLedger.
func newBooker(ledger *RealitiesLedger) *booker {
	return &booker{
		ledger: ledger,
	}
}

// checkAlreadyBookedCommand is a ChainedCommand that aborts the DataFlow if a Transaction has already been booked.
func (b *booker) checkAlreadyBookedCommand(params *dataFlowParams, next dataflow.Next[*dataFlowParams]) (err error) {
	if params.TransactionMetadata == nil {
		cachedTransactionMetadata := b.ledger.Storage().CachedTransactionMetadata(params.Transaction.ID())
		defer cachedTransactionMetadata.Release()

		transactionMetadata, exists := cachedTransactionMetadata.Unwrap()
		if !exists {
			return errors.WithMessagef(cerrors.ErrFatal, "failed to load metadata of %s", params.Transaction.ID())
		}
		params.TransactionMetadata = transactionMetadata
	}

	if params.TransactionMetadata.IsBooked() {
		return nil
	}

	return next(params)
}

// bookTransactionCommand is a ChainedCommand that books a Transaction.
func (b *booker) bookTransactionCommand(params *dataFlowParams, next dataflow.Next[*dataFlowParams]) (err error) {
	b.bookTransaction(params.Context, params.Transaction, params.TransactionMetadata, params.InputsMetadata, params.Consumers, params.Outputs)

	return next(params)
}

// bookTransaction books a Transaction in the RealitiesLedger and creates its Outputs.
func (b *booker) bookTransaction(ctx context.Context, tx utxo.Transaction, txMetadata *mempool.TransactionMetadata, inputsMetadata *mempool.OutputsMetadata, consumers []*mempool.Consumer, outputs *utxo.Outputs) {
	conflictIDs := b.inheritConflictIDs(ctx, txMetadata.ID(), inputsMetadata)

	txMetadata.SetConflictIDs(conflictIDs)
	txMetadata.SetOutputIDs(outputs.IDs())

	var consensusPledgeID, accessPledgeID identity.ID
	if devnetTx, ok := tx.(*devnetvm.Transaction); ok {
		consensusPledgeID = devnetTx.Essence().ConsensusPledgeID()
		accessPledgeID = devnetTx.Essence().AccessPledgeID()
	}

	b.storeOutputs(outputs, conflictIDs, consensusPledgeID, accessPledgeID)

	if b.ledger.conflictDAG.ConfirmationState(conflictIDs).IsRejected() {
		b.ledger.triggerRejectedEvent(txMetadata)
	}

	txMetadata.SetBooked(true)

	lo.ForEach(consumers, func(consumer *mempool.Consumer) { consumer.SetBooked() })

	b.ledger.Events().TransactionBooked.Trigger(&mempool.TransactionBookedEvent{
		TransactionID: txMetadata.ID(),
		Outputs:       outputs,
		Context:       ctx,
	})
}

// inheritedConflictIDs determines the ConflictIDs that a Transaction should inherit when being booked.
func (b *booker) inheritConflictIDs(ctx context.Context, txID utxo.TransactionID, inputsMetadata *mempool.OutputsMetadata) (inheritedConflictIDs *advancedset.AdvancedSet[utxo.TransactionID]) {
	parentConflictIDs := b.ledger.conflictDAG.UnconfirmedConflicts(inputsMetadata.ConflictIDs())

	conflictingInputIDs, consumersToFork := b.determineConflictDetails(txID, inputsMetadata)
	if conflictingInputIDs.Size() == 0 {
		return parentConflictIDs
	}

	confirmationState := confirmation.Pending
	for it := consumersToFork.Iterator(); it.HasNext(); {
		if b.forkTransaction(ctx, it.Next(), conflictingInputIDs).IsAccepted() {
			confirmationState = confirmation.Rejected
		}
	}

	b.ledger.conflictDAG.CreateConflict(txID, parentConflictIDs, conflictingInputIDs, confirmationState)

	return advancedset.New(txID)
}

// storeOutputs stores the Outputs in the RealitiesLedger.
func (b *booker) storeOutputs(outputs *utxo.Outputs, conflictIDs *advancedset.AdvancedSet[utxo.TransactionID], consensusPledgeID, accessPledgeID identity.ID) {
	_ = outputs.ForEach(func(output utxo.Output) (err error) {
		outputMetadata := mempool.NewOutputMetadata(output.ID())
		outputMetadata.SetConflictIDs(conflictIDs)
		outputMetadata.SetAccessManaPledgeID(accessPledgeID)
		outputMetadata.SetConsensusManaPledgeID(consensusPledgeID)
		b.ledger.Storage().OutputMetadataStorage().Store(outputMetadata).Release()
		b.ledger.Storage().OutputStorage().Store(output).Release()

		b.ledger.Events().OutputCreated.Trigger(output.ID())

		return nil
	})
}

// determineConflictDetails determines whether a Transaction is conflicting and returns the conflict details.
func (b *booker) determineConflictDetails(txID utxo.TransactionID, inputsMetadata *mempool.OutputsMetadata) (conflictingInputIDs utxo.OutputIDs, consumersToFork utxo.TransactionIDs) {
	conflictingInputIDs = utxo.NewOutputIDs()
	consumersToFork = utxo.NewTransactionIDs()

	_ = inputsMetadata.ForEach(func(outputMetadata *mempool.OutputMetadata) error {
		isConflicting, consumerToFork := outputMetadata.RegisterBookedConsumer(txID)
		if isConflicting {
			conflictingInputIDs.Add(outputMetadata.ID())
		}

		if consumerToFork != utxo.EmptyTransactionID {
			consumersToFork.Add(consumerToFork)
		}

		return nil
	})

	return conflictingInputIDs, consumersToFork
}

// forkTransaction forks an existing Transaction and returns the confirmation state of the resulting Branch.
func (b *booker) forkTransaction(ctx context.Context, txID utxo.TransactionID, outputsSpentByConflictingTx utxo.OutputIDs) (confirmationState confirmation.State) {
	b.ledger.Utils().WithTransactionAndMetadata(txID, func(tx utxo.Transaction, txMetadata *mempool.TransactionMetadata) {
		b.ledger.mutex.Lock(txID)

		confirmationState = txMetadata.ConfirmationState()
		conflictingInputs := b.ledger.Utils().ResolveInputs(tx.Inputs()).Intersect(outputsSpentByConflictingTx)
		parentConflicts := txMetadata.ConflictIDs()

		if !b.ledger.conflictDAG.CreateConflict(txID, parentConflicts, conflictingInputs, confirmationState) {
			b.ledger.conflictDAG.UpdateConflictingResources(txID, conflictingInputs)
			b.ledger.mutex.Unlock(txID)
			return
		}

		b.ledger.Events().TransactionForked.Trigger(&mempool.TransactionForkedEvent{
			TransactionID:   txID,
			ParentConflicts: parentConflicts,
		})

		b.updateConflictsAfterFork(ctx, txMetadata, txID, parentConflicts)
		b.ledger.mutex.Unlock(txID)

		if !confirmationState.IsAccepted() {
			b.propagateForkedConflictToFutureCone(ctx, txMetadata.OutputIDs(), txID, parentConflicts)
		}
	})

	return confirmationState
}

// propagateForkedConflictToFutureCone propagates a newly introduced Conflict to its future cone.
func (b *booker) propagateForkedConflictToFutureCone(ctx context.Context, outputIDs utxo.OutputIDs, forkedConflictID utxo.TransactionID, previousParentConflicts *advancedset.AdvancedSet[utxo.TransactionID]) {
	b.ledger.Utils().WalkConsumingTransactionMetadata(outputIDs, func(consumingTxMetadata *mempool.TransactionMetadata, walker *walker.Walker[utxo.OutputID]) {
		b.ledger.mutex.Lock(consumingTxMetadata.ID())
		defer b.ledger.mutex.Unlock(consumingTxMetadata.ID())

		if !b.updateConflictsAfterFork(ctx, consumingTxMetadata, forkedConflictID, previousParentConflicts) {
			return
		}

		walker.PushAll(consumingTxMetadata.OutputIDs().Slice()...)
	})
}

// updateConflictsAfterFork updates the ConflictIDs of a Transaction after a fork.
func (b *booker) updateConflictsAfterFork(ctx context.Context, txMetadata *mempool.TransactionMetadata, forkedConflictID utxo.TransactionID, previousParents *advancedset.AdvancedSet[utxo.TransactionID]) (updated bool) {
	if txMetadata.IsConflicting() {
		b.ledger.conflictDAG.UpdateConflictParents(txMetadata.ID(), previousParents, forkedConflictID)
		return false
	}

	if txMetadata.ConflictIDs().Has(forkedConflictID) {
		return false
	}

	newConflictIDs := txMetadata.ConflictIDs().Clone()
	newConflictIDs.DeleteAll(previousParents)
	newConflictIDs.Add(forkedConflictID)
	newConflicts := b.ledger.conflictDAG.UnconfirmedConflicts(newConflictIDs)

	b.ledger.Storage().CachedOutputsMetadata(txMetadata.OutputIDs()).Consume(func(outputMetadata *mempool.OutputMetadata) {
		outputMetadata.SetConflictIDs(newConflicts)
	})

	txMetadata.SetConflictIDs(newConflicts)

	b.ledger.Events().TransactionConflictIDUpdated.Trigger(&mempool.TransactionConflictIDUpdatedEvent{
		TransactionID:      txMetadata.ID(),
		AddedConflictID:    forkedConflictID,
		RemovedConflictIDs: previousParents,
		Context:            ctx,
	})

	return true
}
