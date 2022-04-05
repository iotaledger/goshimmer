package ledger

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/generics/dataflow"
	"github.com/iotaledger/hive.go/generics/lo"
	"github.com/iotaledger/hive.go/generics/walker"

	"github.com/iotaledger/goshimmer/packages/ledger/branchdag"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
)

type booker struct {
	*Ledger
}

func newBooker(ledger *Ledger) (new *booker) {
	return &booker{
		Ledger: ledger,
	}
}

func (b *booker) checkAlreadyBookedCommand(params *dataFlowParams, next dataflow.Next[*dataFlowParams]) (err error) {
	if params.TransactionMetadata == nil {
		cachedTransactionMetadata := b.Storage.CachedTransactionMetadata(params.Transaction.ID())
		defer cachedTransactionMetadata.Release()

		transactionMetadata, exists := cachedTransactionMetadata.Unwrap()
		if !exists {
			return errors.Errorf("failed to load metadata of %s: %w", params.Transaction.ID(), cerrors.ErrFatal)
		}

		params.TransactionMetadata = transactionMetadata
	}

	if params.TransactionMetadata.Booked() {
		return nil
	}

	return next(params)
}

func (b *booker) bookTransactionCommand(params *dataFlowParams, next dataflow.Next[*dataFlowParams]) (err error) {
	b.bookTransaction(params.TransactionMetadata, params.InputsMetadata, params.Consumers, params.Outputs)

	return next(params)
}

func (b *booker) bookTransaction(txMetadata *TransactionMetadata, inputsMetadata OutputsMetadata, consumers []*Consumer, outputs Outputs) {
	branchIDs := b.inheritBranchIDs(txMetadata.ID(), inputsMetadata)

	b.storeOutputs(outputs, branchIDs)

	lo.ForEach(consumers, func(consumer *Consumer) { consumer.SetBooked() })

	txMetadata.SetBranchIDs(branchIDs)
	txMetadata.SetOutputIDs(outputs.IDs())
	txMetadata.SetBooked(true)

	b.Events.TransactionBooked.Trigger(&TransactionBookedEvent{
		TransactionID: txMetadata.ID(),
		Outputs:       outputs,
	})
}

func (b *booker) inheritBranchIDs(txID utxo.TransactionID, inputsMetadata OutputsMetadata) (inheritedBranchIDs branchdag.BranchIDs) {
	conflictingInputIDs, consumersToFork := b.determineConflictDetails(txID, inputsMetadata)
	if conflictingInputIDs.Size() == 0 {
		return b.BranchDAG.FilterPendingBranches(inputsMetadata.BranchIDs())
	}

	branchID := branchdag.NewBranchID(txID)
	b.BranchDAG.CreateBranch(branchID, b.BranchDAG.FilterPendingBranches(inputsMetadata.BranchIDs()), branchdag.NewConflictIDs(lo.Map(conflictingInputIDs.Slice(), branchdag.NewConflictID)...))

	_ = consumersToFork.ForEach(func(transactionID utxo.TransactionID) (err error) {
		b.utils.WithTransactionAndMetadata(transactionID, func(tx *Transaction, txMetadata *TransactionMetadata) {
			b.forkTransaction(tx, txMetadata, conflictingInputIDs)
		})

		return nil
	})

	return branchdag.NewBranchIDs(branchID)
}

func (b *booker) storeOutputs(outputs Outputs, branchIDs branchdag.BranchIDs) {
	_ = outputs.ForEach(func(output *Output) (err error) {
		outputMetadata := NewOutputMetadata(output.ID())
		outputMetadata.SetBranchIDs(branchIDs)

		b.Storage.outputMetadataStorage.Store(outputMetadata).Release()
		b.Storage.outputStorage.Store(output).Release()

		return nil
	})
}

func (b *booker) determineConflictDetails(txID utxo.TransactionID, inputsMetadata OutputsMetadata) (conflictingInputIDs utxo.OutputIDs, consumersToFork utxo.TransactionIDs) {
	conflictingInputIDs = utxo.NewOutputIDs()
	consumersToFork = utxo.NewTransactionIDs()

	_ = inputsMetadata.ForEach(func(outputMetadata *OutputMetadata) error {
		isConflicting, consumerToFork := outputMetadata.RegisterProcessedConsumer(txID)
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

func (b *booker) forkTransaction(tx *Transaction, txMetadata *TransactionMetadata, outputsSpentByConflictingTx utxo.OutputIDs) {
	b.mutex.Lock(txMetadata.ID())

	conflictingInputs := b.utils.resolveInputs(tx.Inputs()).Intersect(outputsSpentByConflictingTx)
	previousParentBranches := txMetadata.BranchIDs()

	forkedBranchID := branchdag.NewBranchID(txMetadata.ID())
	conflictIDs := branchdag.NewConflictIDs(lo.Map(conflictingInputs.Slice(), branchdag.NewConflictID)...)
	if !b.BranchDAG.CreateBranch(forkedBranchID, previousParentBranches, conflictIDs) {
		b.BranchDAG.AddBranchToConflicts(forkedBranchID, conflictIDs)
		b.mutex.Unlock(txMetadata.ID())
		return
	}

	b.Events.TransactionForked.Trigger(&TransactionForkedEvent{
		TransactionID:  txMetadata.ID(),
		ParentBranches: previousParentBranches,
	})

	if !b.updateBranchesAfterFork(txMetadata, forkedBranchID, previousParentBranches) {
		b.mutex.Unlock(txMetadata.ID())
		return
	}
	b.mutex.Unlock(txMetadata.ID())

	b.propagateForkedBranchToFutureCone(txMetadata.OutputIDs(), forkedBranchID, previousParentBranches)

	return
}

func (b *booker) propagateForkedBranchToFutureCone(outputIDs utxo.OutputIDs, forkedBranchID branchdag.BranchID, previousParentBranches branchdag.BranchIDs) {
	b.utils.WalkConsumingTransactionMetadata(outputIDs, func(consumingTxMetadata *TransactionMetadata, walker *walker.Walker[utxo.OutputID]) {
		b.mutex.Lock(consumingTxMetadata.ID())
		defer b.mutex.Unlock(consumingTxMetadata.ID())

		if !b.updateBranchesAfterFork(consumingTxMetadata, forkedBranchID, previousParentBranches) {
			return
		}

		walker.PushAll(consumingTxMetadata.OutputIDs().Slice()...)
	})
}

func (b *booker) updateBranchesAfterFork(txMetadata *TransactionMetadata, forkedBranchID branchdag.BranchID, previousParents branchdag.BranchIDs) bool {
	if txMetadata.IsConflicting() {
		b.BranchDAG.UpdateBranchParents(branchdag.NewBranchID(txMetadata.ID()), forkedBranchID, previousParents)
		return false
	}

	if txMetadata.BranchIDs().Has(forkedBranchID) {
		return false
	}

	newBranchIDs := txMetadata.BranchIDs().Clone()
	newBranchIDs.DeleteAll(previousParents)
	newBranchIDs.Add(forkedBranchID)
	newBranches := b.BranchDAG.FilterPendingBranches(newBranchIDs)

	b.Storage.CachedOutputsMetadata(txMetadata.OutputIDs()).Consume(func(outputMetadata *OutputMetadata) {
		outputMetadata.SetBranchIDs(newBranches)
	})

	txMetadata.SetBranchIDs(newBranches)

	b.Events.TransactionBranchIDUpdated.Trigger(&TransactionBranchIDUpdatedEvent{
		TransactionID:    txMetadata.ID(),
		AddedBranchID:    forkedBranchID,
		RemovedBranchIDs: previousParents,
	})

	return true
}
