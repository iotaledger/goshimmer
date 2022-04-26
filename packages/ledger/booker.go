package ledger

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/generics/dataflow"
	"github.com/iotaledger/hive.go/generics/lo"
	"github.com/iotaledger/hive.go/generics/walker"

	"github.com/iotaledger/goshimmer/packages/ledger/branchdag"
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
func (b *booker) bookTransaction(ctx context.Context, txMetadata *TransactionMetadata, inputsMetadata OutputsMetadata, consumers []*Consumer, outputs Outputs) {
	branchIDs := b.inheritBranchIDs(txMetadata.ID(), inputsMetadata)

	b.storeOutputs(outputs, branchIDs)

	txMetadata.SetBranchIDs(branchIDs)
	txMetadata.SetOutputIDs(outputs.IDs())
	txMetadata.SetBooked(true)

	lo.ForEach(consumers, func(consumer *Consumer) { consumer.SetBooked() })

	b.ledger.Events.TransactionBooked.Trigger(&TransactionBookedEvent{
		TransactionID: txMetadata.ID(),
		Outputs:       outputs,
		Context:       ctx,
	})
}

// inheritedBranchIDs determines the BranchIDs that a Transaction should inherit when being booked.
func (b *booker) inheritBranchIDs(txID utxo.TransactionID, inputsMetadata OutputsMetadata) (inheritedBranchIDs branchdag.BranchIDs) {
	parentBranchIDs := b.ledger.BranchDAG.FilterPendingBranches(inputsMetadata.BranchIDs())

	conflictingInputIDs, consumersToFork := b.determineConflictDetails(txID, inputsMetadata)
	if conflictingInputIDs.Size() == 0 {
		return parentBranchIDs
	}

	forkedBranchID := branchdag.NewBranchID(txID)
	b.ledger.BranchDAG.CreateBranch(forkedBranchID, parentBranchIDs, branchdag.NewConflictIDs(lo.Map(conflictingInputIDs.Slice(), branchdag.NewConflictID)...))

	for it := consumersToFork.Iterator(); it.HasNext(); {
		b.forkTransaction(it.Next(), conflictingInputIDs)
	}

	return branchdag.NewBranchIDs(forkedBranchID)
}

// storeOutputs stores the Outputs in the Ledger.
func (b *booker) storeOutputs(outputs Outputs, branchIDs branchdag.BranchIDs) {
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
func (b *booker) forkTransaction(txID utxo.TransactionID, outputsSpentByConflictingTx utxo.OutputIDs) {
	b.ledger.Utils.WithTransactionAndMetadata(txID, func(tx *Transaction, txMetadata *TransactionMetadata) {
		b.ledger.mutex.Lock(txID)

		conflictingInputs := b.ledger.Utils.ResolveInputs(tx.Inputs()).Intersect(outputsSpentByConflictingTx)
		previousParentBranches := txMetadata.BranchIDs()

		forkedBranchID := branchdag.NewBranchID(txID)
		conflictIDs := branchdag.NewConflictIDs(lo.Map(conflictingInputs.Slice(), branchdag.NewConflictID)...)
		if !b.ledger.BranchDAG.CreateBranch(forkedBranchID, previousParentBranches, conflictIDs) {
			b.ledger.BranchDAG.AddBranchToConflicts(forkedBranchID, conflictIDs)
			b.ledger.mutex.Unlock(txID)
			return
		}

		b.ledger.Events.TransactionForked.Trigger(&TransactionForkedEvent{
			TransactionID:  txID,
			ParentBranches: previousParentBranches,
			ForkedBranchID: forkedBranchID,
		})

		b.updateBranchesAfterFork(txMetadata, forkedBranchID, previousParentBranches)
		b.ledger.mutex.Unlock(txID)

		b.propagateForkedBranchToFutureCone(txMetadata.OutputIDs(), forkedBranchID, previousParentBranches)
	})

	return
}

// propagateForkedBranchToFutureCone propagates a newly introduced Branch to its future cone.
func (b *booker) propagateForkedBranchToFutureCone(outputIDs utxo.OutputIDs, forkedBranchID branchdag.BranchID, previousParentBranches branchdag.BranchIDs) {
	b.ledger.Utils.WalkConsumingTransactionMetadata(outputIDs, func(consumingTxMetadata *TransactionMetadata, walker *walker.Walker[utxo.OutputID]) {
		b.ledger.mutex.Lock(consumingTxMetadata.ID())
		defer b.ledger.mutex.Unlock(consumingTxMetadata.ID())

		if !b.updateBranchesAfterFork(consumingTxMetadata, forkedBranchID, previousParentBranches) {
			return
		}

		walker.PushAll(consumingTxMetadata.OutputIDs().Slice()...)
	})
}

// updateBranchesAfterFork updates the BranchIDs of a Transaction after a fork.
func (b *booker) updateBranchesAfterFork(txMetadata *TransactionMetadata, forkedBranchID branchdag.BranchID, previousParents branchdag.BranchIDs) (updated bool) {
	if txMetadata.IsConflicting() {
		b.ledger.BranchDAG.UpdateBranchParents(branchdag.NewBranchID(txMetadata.ID()), forkedBranchID, previousParents)
		return false
	}

	if txMetadata.BranchIDs().Has(forkedBranchID) {
		return false
	}

	newBranchIDs := txMetadata.BranchIDs().Clone()
	newBranchIDs.DeleteAll(previousParents)
	newBranchIDs.Add(forkedBranchID)
	newBranches := b.ledger.BranchDAG.FilterPendingBranches(newBranchIDs)

	b.ledger.Storage.CachedOutputsMetadata(txMetadata.OutputIDs()).Consume(func(outputMetadata *OutputMetadata) {
		outputMetadata.SetBranchIDs(newBranches)
	})

	txMetadata.SetBranchIDs(newBranches)

	b.ledger.Events.TransactionBranchIDUpdated.Trigger(&TransactionBranchIDUpdatedEvent{
		TransactionID:    txMetadata.ID(),
		AddedBranchID:    forkedBranchID,
		RemovedBranchIDs: previousParents,
	})

	return true
}
