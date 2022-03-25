package ledger

import (
	"github.com/iotaledger/hive.go/generics/dataflow"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/generics/walker"

	lo "github.com/iotaledger/goshimmer/packages/refactored/generics"
	"github.com/iotaledger/goshimmer/packages/refactored/generics/filter"
	"github.com/iotaledger/goshimmer/packages/refactored/ledger/branchdag"
	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

type Booker struct {
	*Ledger
}

func (b *Booker) bookTransactionCommand(params *params, next dataflow.Next[*params]) (err error) {
	inheritedBranchIDs := b.bookTransaction(params.Transaction.ID(), params.TransactionMetadata, params.InputsMetadata)

	cachedOutputsMetadata := b.bookOutputs(params.Transaction.ID(), params.Outputs, inheritedBranchIDs)
	defer cachedOutputsMetadata.Release()

	params.OutputsMetadata = lo.KeyBy(cachedOutputsMetadata.Unwrap(), (*OutputMetadata).ID)

	return next(params)
}

func (b *Booker) bookTransaction(txID utxo.TransactionID, txMetadata *TransactionMetadata, inputsMetadata map[utxo.OutputID]*OutputMetadata) (inheritedBranchIDs branchdag.BranchIDs) {
	inheritedBranchIDs = b.RemoveConfirmedBranches(lo.ReduceProperty(lo.Values(inputsMetadata), (*OutputMetadata).BranchIDs, branchdag.BranchIDs.AddAll, branchdag.NewBranchIDs()))

	conflictingInputsMetadata := lo.FilterByValue(inputsMetadata, lo.Bind(txID, (*OutputMetadata).IsProcessedConsumerDoubleSpend))
	conflictingInputIDs := lo.Keys(conflictingInputsMetadata)
	if len(conflictingInputIDs) != 0 {
		inheritedBranchIDs = branchdag.NewBranchIDs(b.CreateBranch(txID, inheritedBranchIDs, branchdag.NewConflictIDs(lo.Map(conflictingInputIDs, branchdag.NewConflictID)...)))

		b.WalkConsumingTransactionAndMetadata(conflictingInputIDs, func(tx utxo.Transaction, txMetadata *TransactionMetadata, _ *walker.Walker[utxo.OutputID]) {
			b.forkTransactionFutureCone(tx, txMetadata, lo.KeyBy(conflictingInputIDs, lo.Identity[utxo.OutputID]))
		})
	}

	txMetadata.SetBranchIDs(inheritedBranchIDs)

	return inheritedBranchIDs
}

func (b *Booker) bookOutputs(txID utxo.TransactionID, outputs []*Output, branchIDs branchdag.BranchIDs) (cachedOutputsMetadata objectstorage.CachedObjects[*OutputMetadata]) {
	cachedOutputsMetadata = make(objectstorage.CachedObjects[*OutputMetadata], len(outputs))
	for index, output := range outputs {
		outputMetadata := NewOutputMetadata(output.ID())
		outputMetadata.SetBranchIDs(branchIDs)

		cachedOutputsMetadata[index] = b.outputMetadataStorage.Store(outputMetadata)
		b.outputStorage.Store(output).Release()
	}

	return cachedOutputsMetadata
}

func (b *Booker) forkTransactionFutureCone(tx utxo.Transaction, txMetadata *TransactionMetadata, outputsSpentByConflictingTx map[utxo.OutputID]utxo.OutputID) {
	forkedBranchID, previousParentBranches, success := b.forkTransaction(tx, txMetadata, outputsSpentByConflictingTx)
	if !success {
		return
	}

	// trigger forked event

	b.forkFutureCone(txMetadata, forkedBranchID, previousParentBranches)
}

func (b *Booker) forkTransaction(tx utxo.Transaction, txMetadata *TransactionMetadata, outputsSpentByConflictingTx map[utxo.OutputID]utxo.OutputID) (forkedBranchID branchdag.BranchID, previousParentBranches branchdag.BranchIDs, success bool) {
	b.Lock(txMetadata.ID())
	defer b.Unlock(txMetadata.ID())

	conflictingInputs := lo.Filter(b.outputIDsFromInputs(tx.Inputs()), filter.MapHasKey(outputsSpentByConflictingTx))
	conflictIDs := branchdag.NewConflictIDs(lo.Map(conflictingInputs, branchdag.NewConflictID)...)

	previousParentBranches = txMetadata.BranchIDs()
	forkedBranchID = b.CreateBranch(txMetadata.ID(), previousParentBranches, conflictIDs)
	success = b.updateBranchesAfterFork(txMetadata, forkedBranchID, previousParentBranches)

	return
}

func (b *Booker) forkFutureCone(txMetadata *TransactionMetadata, forkedBranchID branchdag.BranchID, previousParentBranches branchdag.BranchIDs) {
	b.WalkConsumingTransactionMetadata(txMetadata.OutputIDs(), func(consumingTxMetadata *TransactionMetadata, walker *walker.Walker[utxo.OutputID]) {
		b.Lock(consumingTxMetadata.ID())
		defer b.Unlock(consumingTxMetadata.ID())

		if !b.updateBranchesAfterFork(consumingTxMetadata, forkedBranchID, previousParentBranches) {
			return
		}

		// Trigger propagated event

		walker.PushAll(consumingTxMetadata.OutputIDs()...)
	})
}

func (b *Booker) updateBranchesAfterFork(txMetadata *TransactionMetadata, forkedBranchID branchdag.BranchID, previousParents branchdag.BranchIDs) bool {
	if txMetadata.BranchIDs().Contains(forkedBranchID) {
		return false
	}

	newBranches := b.RemoveConfirmedBranches(txMetadata.BranchIDs().Subtract(previousParents).Add(forkedBranchID))
	objectstorage.CachedObjects[*OutputMetadata](lo.Map(txMetadata.OutputIDs(), b.CachedOutputMetadata)).Consume(func(outputMetadata *OutputMetadata) {
		outputMetadata.SetBranchIDs(newBranches)
	})
	txMetadata.SetBranchIDs(newBranches)

	return true
}
