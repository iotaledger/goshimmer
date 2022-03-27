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

func NewBooker(ledger *Ledger) (new *Booker) {
	return &Booker{
		Ledger: ledger,
	}
}

func (b *Booker) bookTransactionCommand(params *params, next dataflow.Next[*params]) (err error) {
	cachedOutputsMetadata := b.bookTransaction(params.Transaction.ID(), params.TransactionMetadata, params.InputsMetadata, params.Outputs)
	defer cachedOutputsMetadata.Release()

	params.OutputsMetadata = lo.KeyBy(cachedOutputsMetadata.Unwrap(), (*OutputMetadata).ID)

	return next(params)
}

func (b *Booker) bookTransaction(txID utxo.TransactionID, txMetadata *TransactionMetadata, inputsMetadata map[utxo.OutputID]*OutputMetadata, outputs []*Output) (cachedOutputsMetadata objectstorage.CachedObjects[*OutputMetadata]) {
	inheritedBranchIDs := b.inheritBranchIDsFromInputs(txID, inputsMetadata)

	txMetadata.SetBranchIDs(inheritedBranchIDs)

	return b.bookOutputs(outputs, inheritedBranchIDs)
}

func (b *Booker) inheritBranchIDsFromInputs(txID utxo.TransactionID, inputsMetadata map[utxo.OutputID]*OutputMetadata) (inheritedBranchIDs branchdag.BranchIDs) {
	pendingParentBranchIDs := b.RemoveConfirmedBranches(lo.ReduceProperty(lo.Values(inputsMetadata), (*OutputMetadata).BranchIDs, branchdag.BranchIDs.AddAll, branchdag.NewBranchIDs()))

	conflictingInputIDs, consumersToFork := b.determineConflictDetails(txID, inputsMetadata)
	if len(conflictingInputIDs) == 0 {
		return pendingParentBranchIDs
	}

	forkedBranchID := b.CreateBranch(txID, pendingParentBranchIDs, conflictingInputIDs)

	for consumerToFork := range consumersToFork {
		b.WithTransactionAndMetadata(consumerToFork, func(tx *Transaction, txMetadata *TransactionMetadata) {
			b.forkTransaction(tx, txMetadata, conflictingInputIDs)
		})
	}

	return branchdag.NewBranchIDs(forkedBranchID)
}

func (b *Booker) determineConflictDetails(txID utxo.TransactionID, inputsMetadata map[utxo.OutputID]*OutputMetadata) (conflictingInputIDs map[utxo.OutputID]bool, consumersToFork map[utxo.TransactionID]bool) {
	conflictingInputIDs = make(map[utxo.OutputID]bool)
	consumersToFork = make(map[utxo.TransactionID]bool)

	for outputID, outputMetadata := range inputsMetadata {
		isConflicting, consumerToFork := outputMetadata.RegisterProcessedConsumer(txID)
		if isConflicting {
			conflictingInputIDs[outputID] = true
		}

		if consumerToFork != utxo.EmptyTransactionID {
			consumersToFork[consumerToFork] = true
		}
	}

	return conflictingInputIDs, consumersToFork
}

func (b *Booker) bookOutputs(outputs []*Output, branchIDs branchdag.BranchIDs) (cachedOutputsMetadata objectstorage.CachedObjects[*OutputMetadata]) {
	cachedOutputsMetadata = make(objectstorage.CachedObjects[*OutputMetadata], len(outputs))
	for index, output := range outputs {
		outputMetadata := NewOutputMetadata(output.ID())
		outputMetadata.SetBranchIDs(branchIDs)

		cachedOutputsMetadata[index] = b.outputMetadataStorage.Store(outputMetadata)
		b.outputStorage.Store(output).Release()
	}

	return cachedOutputsMetadata
}

func (b *Booker) forkTransaction(tx utxo.Transaction, txMetadata *TransactionMetadata, outputsSpentByConflictingTx map[utxo.OutputID]bool) {
	b.Lock(txMetadata.ID())
	defer b.Unlock(txMetadata.ID())

	conflictingInputs := lo.Filter(b.outputIDsFromInputs(tx.Inputs()), filter.MapHasKey(outputsSpentByConflictingTx))
	previousParentBranches := txMetadata.BranchIDs()

	// TODO: RETURN
	forkedBranchID := b.CreateBranch(txMetadata.ID(), previousParentBranches, branchdag.NewConflictIDs(lo.Map(conflictingInputs, branchdag.NewConflictID)...))

	if !b.updateBranchesAfterFork(txMetadata, forkedBranchID, previousParentBranches) {
		return
	}

	b.TransactionForkedEvent.Trigger()
	// trigger forked event

	b.propagateForkedBranchToFutureCone(txMetadata, forkedBranchID, previousParentBranches)

	return
}

func (b *Booker) propagateForkedBranchToFutureCone(txMetadata *TransactionMetadata, forkedBranchID branchdag.BranchID, previousParentBranches branchdag.BranchIDs) {
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
