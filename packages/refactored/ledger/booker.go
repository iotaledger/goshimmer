package ledger

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/generics/dataflow"
	"github.com/iotaledger/hive.go/generics/walker"

	"github.com/iotaledger/goshimmer/packages/refactored/branchdag"
	"github.com/iotaledger/goshimmer/packages/refactored/generics"
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

	b.TransactionBookedEvent.Trigger(txMetadata.ID())
}

func (b *Booker) inheritBranchIDs(txID TransactionID, inputsMetadata OutputsMetadata) (inheritedBranchIDs branchdag.BranchIDs) {
	conflictingInputIDs, consumersToFork := b.determineConflictDetails(txID, inputsMetadata)
	if conflictingInputIDs.Size() == 0 {
		return b.RemoveConfirmedBranches(inputsMetadata.BranchIDs())
	}

	b.CreateBranch(txID, b.RemoveConfirmedBranches(inputsMetadata.BranchIDs()), conflictingInputIDs)

	_ = consumersToFork.ForEach(func(transactionID TransactionID) (err error) {
		b.WithTransactionAndMetadata(transactionID, func(tx *Transaction, txMetadata *TransactionMetadata) {
			b.forkTransaction(tx, txMetadata, conflictingInputIDs)
		})

		return nil
	})

	return branchdag.NewBranchIDs(txID)
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

func (b *Booker) determineConflictDetails(txID TransactionID, inputsMetadata OutputsMetadata) (conflictingInputIDs OutputIDs, consumersToFork TransactionIDs) {
	conflictingInputIDs = NewOutputIDs()
	consumersToFork = NewTransactionIDs()

	_ = inputsMetadata.ForEach(func(outputMetadata *OutputMetadata) error {
		isConflicting, consumerToFork := outputMetadata.RegisterProcessedConsumer(txID)
		if isConflicting {
			conflictingInputIDs.Add(outputMetadata.ID())
		}

		if consumerToFork != EmptyTransactionID {
			consumersToFork.Add(consumerToFork)
		}

		return nil
	})

	return conflictingInputIDs, consumersToFork
}

func (b *Booker) forkTransaction(tx *Transaction, txMetadata *TransactionMetadata, outputsSpentByConflictingTx OutputIDs) {
	b.Lock(txMetadata.ID())

	conflictingInputs := b.resolveInputs(tx.Inputs()).Intersect(outputsSpentByConflictingTx)
	previousParentBranches := txMetadata.BranchIDs()

	if !b.CreateBranch(txMetadata.ID(), previousParentBranches, conflictingInputs) || !b.updateBranchesAfterFork(txMetadata, txMetadata.ID(), previousParentBranches) {
		b.Unlock(txMetadata.ID())
		return
	}
	b.Unlock(txMetadata.ID())

	// b.TransactionForkedEvent.Trigger()
	// trigger forked event

	b.propagateForkedBranchToFutureCone(txMetadata, txMetadata.ID(), previousParentBranches)

	return
}

func (b *Booker) propagateForkedBranchToFutureCone(txMetadata *TransactionMetadata, forkedBranchID branchdag.BranchID, previousParentBranches branchdag.BranchIDs) {
	b.WalkConsumingTransactionMetadata(txMetadata.OutputIDs(), func(consumingTxMetadata *TransactionMetadata, walker *walker.Walker[OutputID]) {
		b.Lock(consumingTxMetadata.ID())
		defer b.Unlock(consumingTxMetadata.ID())

		if !b.updateBranchesAfterFork(consumingTxMetadata, forkedBranchID, previousParentBranches) {
			return
		}

		// Trigger propagated event

		walker.PushAll(consumingTxMetadata.OutputIDs().Slice()...)
	})
}

func (b *Booker) updateBranchesAfterFork(txMetadata *TransactionMetadata, forkedBranchID branchdag.BranchID, previousParents branchdag.BranchIDs) bool {
	if txMetadata.IsConflicting() {
		b.BranchDAG.UpdateParentsAfterFork(txMetadata.ID(), forkedBranchID, previousParents)
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

	return true
}
