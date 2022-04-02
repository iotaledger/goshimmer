package ledger

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/generics/dataflow"
	"github.com/iotaledger/hive.go/generics/walker"

	"github.com/iotaledger/goshimmer/packages/refactored/branchdag"
	"github.com/iotaledger/goshimmer/packages/refactored/generics"
	"github.com/iotaledger/goshimmer/packages/refactored/types/utxo"
)

type Booker struct {
	*Ledger
}

func NewBooker(ledger *Ledger) (new *Booker) {
	return &Booker{
		Ledger: ledger,
	}
}

func (b *Booker) checkAlreadyBookedCommand(params *dataFlowParams, next dataflow.Next[*dataFlowParams]) (err error) {
	if params.TransactionMetadata == nil {
		cachedTransactionMetadata := b.CachedTransactionMetadata(params.Transaction.ID())
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

func (b *Booker) bookTransactionCommand(params *dataFlowParams, next dataflow.Next[*dataFlowParams]) (err error) {
	b.bookTransaction(params.TransactionMetadata, params.InputsMetadata, params.Consumers, params.Outputs)

	return next(params)
}

func (b *Booker) bookTransaction(txMetadata *TransactionMetadata, inputsMetadata OutputsMetadata, consumers []*Consumer, outputs Outputs) {
	branchIDs := b.inheritBranchIDs(txMetadata.ID(), inputsMetadata)

	b.storeOutputs(outputs, branchIDs)

	generics.ForEach(consumers, func(consumer *Consumer) { consumer.SetBooked() })

	txMetadata.SetBranchIDs(branchIDs)
	txMetadata.SetOutputIDs(outputs.IDs())
	txMetadata.SetBooked(true)

	b.TransactionBookedEvent.Trigger(&TransactionBookedEvent{
		TransactionID: txMetadata.ID(),
		Outputs:       outputs,
	})
}

func (b *Booker) inheritBranchIDs(txID utxo.TransactionID, inputsMetadata OutputsMetadata) (inheritedBranchIDs branchdag.BranchIDs) {
	conflictingInputIDs, consumersToFork := b.determineConflictDetails(txID, inputsMetadata)
	if conflictingInputIDs.Size() == 0 {
		return b.RemoveConfirmedBranches(inputsMetadata.BranchIDs())
	}

	branchID := branchdag.NewBranchID(txID)
	b.CreateBranch(branchID, b.RemoveConfirmedBranches(inputsMetadata.BranchIDs()), conflictingInputIDs)

	_ = consumersToFork.ForEach(func(transactionID utxo.TransactionID) (err error) {
		b.WithTransactionAndMetadata(transactionID, func(tx *Transaction, txMetadata *TransactionMetadata) {
			b.forkTransaction(tx, txMetadata, conflictingInputIDs)
		})

		return nil
	})

	return branchdag.NewBranchIDs(branchID)
}

func (b *Booker) storeOutputs(outputs Outputs, branchIDs branchdag.BranchIDs) {
	_ = outputs.ForEach(func(output *Output) (err error) {
		outputMetadata := NewOutputMetadata(output.ID())
		outputMetadata.SetBranchIDs(branchIDs)

		b.outputMetadataStorage.Store(outputMetadata).Release()
		b.outputStorage.Store(output).Release()

		return nil
	})
}

func (b *Booker) determineConflictDetails(txID utxo.TransactionID, inputsMetadata OutputsMetadata) (conflictingInputIDs utxo.OutputIDs, consumersToFork utxo.TransactionIDs) {
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

func (b *Booker) forkTransaction(tx *Transaction, txMetadata *TransactionMetadata, outputsSpentByConflictingTx utxo.OutputIDs) {
	b.Lock(txMetadata.ID())

	conflictingInputs := b.resolveInputs(tx.Inputs()).Intersect(outputsSpentByConflictingTx)
	previousParentBranches := txMetadata.BranchIDs()

	forkedBranchID := branchdag.NewBranchID(txMetadata.ID())
	if !b.CreateBranch(forkedBranchID, previousParentBranches, conflictingInputs) {
		b.Unlock(txMetadata.ID())
		return
	}

	b.TransactionForkedEvent.Trigger(&TransactionForkedEvent{
		TransactionID:  txMetadata.ID(),
		ParentBranches: previousParentBranches,
	})

	if !b.updateBranchesAfterFork(txMetadata, forkedBranchID, previousParentBranches) {
		b.Unlock(txMetadata.ID())
		return
	}
	b.Unlock(txMetadata.ID())

	b.propagateForkedBranchToFutureCone(txMetadata.OutputIDs(), forkedBranchID, previousParentBranches)

	return
}

func (b *Booker) propagateForkedBranchToFutureCone(outputIDs utxo.OutputIDs, forkedBranchID branchdag.BranchID, previousParentBranches branchdag.BranchIDs) {
	b.WalkConsumingTransactionMetadata(outputIDs, func(consumingTxMetadata *TransactionMetadata, walker *walker.Walker[utxo.OutputID]) {
		b.Lock(consumingTxMetadata.ID())
		defer b.Unlock(consumingTxMetadata.ID())

		if !b.updateBranchesAfterFork(consumingTxMetadata, forkedBranchID, previousParentBranches) {
			return
		}

		walker.PushAll(consumingTxMetadata.OutputIDs().Slice()...)
	})
}

func (b *Booker) updateBranchesAfterFork(txMetadata *TransactionMetadata, forkedBranchID branchdag.BranchID, previousParents branchdag.BranchIDs) bool {
	if txMetadata.IsConflicting() {
		b.BranchDAG.UpdateParentsAfterFork(branchdag.NewBranchID(txMetadata.ID()), forkedBranchID, previousParents)
		return false
	}

	if txMetadata.BranchIDs().Has(forkedBranchID) {
		return false
	}

	newBranchIDs := txMetadata.BranchIDs().Clone()
	newBranchIDs.DeleteAll(previousParents)
	newBranchIDs.Add(forkedBranchID)
	newBranches := b.RemoveConfirmedBranches(newBranchIDs)

	b.CachedOutputsMetadata(txMetadata.OutputIDs()).Consume(func(outputMetadata *OutputMetadata) {
		outputMetadata.SetBranchIDs(newBranches)
	})

	txMetadata.SetBranchIDs(newBranches)

	b.TransactionBranchIDUpdatedEvent.Trigger(&TransactionBranchIDUpdatedEvent{
		TransactionID:    txMetadata.ID(),
		AddedBranchID:    forkedBranchID,
		RemovedBranchIDs: previousParents,
	})

	return true
}
