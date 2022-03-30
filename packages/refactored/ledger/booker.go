package ledger

import (
	"fmt"

	"github.com/iotaledger/hive.go/generics/dataflow"
	"github.com/iotaledger/hive.go/generics/objectstorage"
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

func (b *Booker) bookTransactionCommand(params *params, next dataflow.Next[*params]) (err error) {
	cachedOutputsMetadata := b.bookTransaction(params.Transaction.ID(), params.TransactionMetadata, params.InputsMetadata, params.Outputs)
	defer cachedOutputsMetadata.Release()
	params.OutputsMetadata = NewOutputsMetadata(cachedOutputsMetadata.Unwrap()...)

	generics.ForEach(params.Consumers, func(consumer *Consumer) {
		consumer.SetBooked()
	})
	params.TransactionMetadata.SetBooked(true)

	b.TransactionBookedEvent.Trigger(params.Transaction.ID())

	return next(params)
}

func (b *Booker) bookTransaction(txID TransactionID, txMetadata *TransactionMetadata, inputsMetadata OutputsMetadata, outputs Outputs) (cachedOutputsMetadata objectstorage.CachedObjects[*OutputMetadata]) {
	inheritedBranchIDs := b.inheritBranchIDsFromInputs(txID, inputsMetadata)

	txMetadata.SetBranchIDs(inheritedBranchIDs)

	return b.bookOutputs(txMetadata, outputs, inheritedBranchIDs)
}

func (b *Booker) inheritBranchIDsFromInputs(txID TransactionID, inputsMetadata OutputsMetadata) (inheritedBranchIDs branchdag.BranchIDs) {
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

func (b *Booker) bookOutputs(txMetadata *TransactionMetadata, outputs Outputs, branchIDs branchdag.BranchIDs) (cachedOutputsMetadata objectstorage.CachedObjects[*OutputMetadata]) {
	cachedOutputsMetadata = make(objectstorage.CachedObjects[*OutputMetadata], 0)

	_ = outputs.ForEach(func(output *Output) (err error) {
		outputMetadata := NewOutputMetadata(output.ID())
		outputMetadata.SetBranchIDs(branchIDs)

		cachedOutputsMetadata = append(cachedOutputsMetadata, b.outputMetadataStorage.Store(outputMetadata))
		b.outputStorage.Store(output).Release()
		return nil
	})

	txMetadata.SetOutputIDs(outputs.IDs())

	return cachedOutputsMetadata
}

func (b *Booker) forkTransaction(tx *Transaction, txMetadata *TransactionMetadata, outputsSpentByConflictingTx OutputIDs) {
	b.Lock(txMetadata.ID())

	conflictingInputs := b.resolveInputs(tx.Inputs()).Intersect(outputsSpentByConflictingTx)
	previousParentBranches := txMetadata.BranchIDs()

	fmt.Println(txMetadata.ID(), previousParentBranches)

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
		fmt.Println("conflicting")
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
