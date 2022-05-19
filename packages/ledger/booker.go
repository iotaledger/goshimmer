package ledger

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/generics/dataflow"
	"github.com/iotaledger/hive.go/generics/lo"
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/generics/walker"

	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
)

// booker is a Ledger component that bundles the booking related API.
type booker struct {
	// ledger contains a reference to the Ledger that created the booker.
	ledger *Ledger
}

// newBooker returns a new booker instance for the given Ledger.
func newBooker(ledger *Ledger) (new *booker) {
	return &booker{
		ledger: ledger,
	}
}

// checkAlreadyBookedCommand is a ChainedCommand that aborts the DataFlow if a Transaction has already been booked.
func (b *booker) checkAlreadyBookedCommand(params *dataFlowParams, next dataflow.Next[*dataFlowParams]) (err error) {
	if params.TransactionMetadata == nil {
		cachedTransactionMetadata := b.ledger.Storage.CachedTransactionMetadata(params.Transaction.ID())
		defer cachedTransactionMetadata.Release()

		transactionMetadata, exists := cachedTransactionMetadata.Unwrap()
		if !exists {
			return errors.Errorf("failed to load metadata of %s: %w", params.Transaction.ID(), cerrors.ErrFatal)
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
	b.bookTransaction(params.Context, params.TransactionMetadata, params.InputsMetadata, params.Consumers, params.Outputs)

	return next(params)
}

// bookTransaction books a Transaction in the Ledger and creates its Outputs.
func (b *booker) bookTransaction(ctx context.Context, txMetadata *TransactionMetadata, inputsMetadata OutputsMetadata, consumers []*Consumer, outputs utxo.Outputs) {
	branchIDs := b.inheritBranchIDs(ctx, txMetadata.ID(), inputsMetadata)

	txMetadata.SetBranchIDs(branchIDs)
	txMetadata.SetOutputIDs(outputs.IDs())

	b.storeOutputs(outputs, branchIDs)

	txMetadata.SetBooked(true)

	lo.ForEach(consumers, func(consumer *Consumer) { consumer.SetBooked() })

	b.ledger.Events.TransactionBooked.Trigger(&TransactionBookedEvent{
		TransactionID: txMetadata.ID(),
		Outputs:       outputs,
		Context:       ctx,
	})
}

// inheritedBranchIDs determines the BranchIDs that a Transaction should inherit when being booked.
func (b *booker) inheritBranchIDs(ctx context.Context, txID utxo.TransactionID, inputsMetadata OutputsMetadata) (inheritedBranchIDs *set.AdvancedSet[utxo.TransactionID]) {
	parentBranchIDs := b.ledger.ConflictDAG.FilterPendingBranches(inputsMetadata.BranchIDs())

	conflictingInputIDs, consumersToFork := b.determineConflictDetails(txID, inputsMetadata)
	if conflictingInputIDs.Size() == 0 {
		return parentBranchIDs
	}

	b.ledger.ConflictDAG.CreateConflict(txID, parentBranchIDs, conflictingInputIDs)

	for it := consumersToFork.Iterator(); it.HasNext(); {
		b.forkTransaction(ctx, it.Next(), conflictingInputIDs)
	}

	return set.NewAdvancedSet(txID)
}

// storeOutputs stores the Outputs in the Ledger.
func (b *booker) storeOutputs(outputs utxo.Outputs, branchIDs *set.AdvancedSet[utxo.TransactionID]) {
	_ = outputs.ForEach(func(output utxo.Output) (err error) {
		outputMetadata := NewOutputMetadata(output.ID())
		outputMetadata.SetBranchIDs(branchIDs)

		b.ledger.Storage.outputMetadataStorage.Store(outputMetadata).Release()
		b.ledger.Storage.outputStorage.Store(output).Release()

		return nil
	})
}

// determineConflictDetails determines whether a Transaction is conflicting and returns the conflict details.
func (b *booker) determineConflictDetails(txID utxo.TransactionID, inputsMetadata OutputsMetadata) (conflictingInputIDs utxo.OutputIDs, consumersToFork utxo.TransactionIDs) {
	conflictingInputIDs = utxo.NewOutputIDs()
	consumersToFork = utxo.NewTransactionIDs()

	_ = inputsMetadata.ForEach(func(outputMetadata *OutputMetadata) error {
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

// forkTransaction forks an existing Transaction.
func (b *booker) forkTransaction(ctx context.Context, txID utxo.TransactionID, outputsSpentByConflictingTx utxo.OutputIDs) {
	b.ledger.Utils.WithTransactionAndMetadata(txID, func(tx utxo.Transaction, txMetadata *TransactionMetadata) {
		b.ledger.mutex.Lock(txID)

		conflictingInputs := b.ledger.Utils.ResolveInputs(tx.Inputs()).Intersect(outputsSpentByConflictingTx)
		parentConflicts := txMetadata.BranchIDs()

		if !b.ledger.ConflictDAG.CreateConflict(txID, parentConflicts, conflictingInputs) {
			b.ledger.ConflictDAG.AddConflictToConflictSets(txID, conflictingInputs)
			b.ledger.mutex.Unlock(txID)
			return
		}

		b.ledger.Events.TransactionForked.Trigger(&TransactionForkedEvent{
			TransactionID:   txID,
			ParentConflicts: parentConflicts,
		})

		b.updateBranchesAfterFork(ctx, txMetadata, txID, parentConflicts)
		b.ledger.mutex.Unlock(txID)

		b.propagateForkedBranchToFutureCone(ctx, txMetadata.OutputIDs(), txID, parentConflicts)
	})
}

// propagateForkedBranchToFutureCone propagates a newly introduced Branch to its future cone.
func (b *booker) propagateForkedBranchToFutureCone(ctx context.Context, outputIDs utxo.OutputIDs, forkedBranchID utxo.TransactionID, previousParentBranches *set.AdvancedSet[utxo.TransactionID]) {
	b.ledger.Utils.WalkConsumingTransactionMetadata(outputIDs, func(consumingTxMetadata *TransactionMetadata, walker *walker.Walker[utxo.OutputID]) {
		b.ledger.mutex.Lock(consumingTxMetadata.ID())
		defer b.ledger.mutex.Unlock(consumingTxMetadata.ID())

		if !b.updateBranchesAfterFork(ctx, consumingTxMetadata, forkedBranchID, previousParentBranches) {
			return
		}

		walker.PushAll(consumingTxMetadata.OutputIDs().Slice()...)
	})
}

// updateBranchesAfterFork updates the BranchIDs of a Transaction after a fork.
func (b *booker) updateBranchesAfterFork(ctx context.Context, txMetadata *TransactionMetadata, forkedBranchID utxo.TransactionID, previousParents *set.AdvancedSet[utxo.TransactionID]) (updated bool) {
	if txMetadata.IsConflicting() {
		b.ledger.ConflictDAG.UpdateConflictParents(txMetadata.ID(), forkedBranchID, previousParents)
		return false
	}

	if txMetadata.BranchIDs().Has(forkedBranchID) {
		return false
	}

	newBranchIDs := txMetadata.BranchIDs().Clone()
	newBranchIDs.DeleteAll(previousParents)
	newBranchIDs.Add(forkedBranchID)
	newBranches := b.ledger.ConflictDAG.FilterPendingBranches(newBranchIDs)

	b.ledger.Storage.CachedOutputsMetadata(txMetadata.OutputIDs()).Consume(func(outputMetadata *OutputMetadata) {
		outputMetadata.SetBranchIDs(newBranches)
	})

	txMetadata.SetBranchIDs(newBranches)

	b.ledger.Events.TransactionBranchIDUpdated.Trigger(&TransactionBranchIDUpdatedEvent{
		TransactionID:    txMetadata.ID(),
		AddedBranchID:    forkedBranchID,
		RemovedBranchIDs: previousParents,
		Context:          ctx,
	})

	return true
}
