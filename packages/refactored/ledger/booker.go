package ledger

import (
	"github.com/iotaledger/hive.go/generics/dataflow"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/generics/walker"
	"github.com/iotaledger/hive.go/types"

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

		b.forkConsumers(conflictingInputIDs)
	}

	txMetadata.SetBranchIDs(inheritedBranchIDs)

	return inheritedBranchIDs
}

func (b *Booker) bookOutputs(txID utxo.TransactionID, outputs []utxo.Output, branchIDs branchdag.BranchIDs) (cachedOutputsMetadata objectstorage.CachedObjects[*OutputMetadata]) {
	cachedOutputsMetadata = make(objectstorage.CachedObjects[*OutputMetadata], len(outputs))

	for index, output := range outputs {
		output.SetID(utxo.NewOutputID(txID, uint16(index), output.Bytes()))

		outputMetadata := NewOutputMetadata(output.ID())
		outputMetadata.SetBranchIDs(branchIDs)

		cachedOutputsMetadata[index] = b.outputMetadataStorage.Store(outputMetadata)
		b.outputStorage.Store(output).Release()
	}

	return cachedOutputsMetadata
}

func (b *Booker) forkConsumers(conflictingInputIDs []utxo.OutputID) {
	b.WalkConsumingTransactionID(conflictingInputIDs, func(txID utxo.TransactionID, _ *walker.Walker[utxo.OutputID]) {
		b.forkConsumer(txID, lo.KeyBy(conflictingInputIDs, lo.Identity[utxo.OutputID]))
	})
}

func (b *Booker) forkConsumer(txID utxo.TransactionID, conflictingInputIDs map[utxo.OutputID]utxo.OutputID) {
	b.Lock(txID)
	defer b.Unlock(txID)

	b.WithTransactionAndMetadata(txID, func(tx utxo.Transaction, txMetadata *TransactionMetadata) {
		conflictingOutputIDs := lo.Filter(b.outputIDsFromInputs(tx.Inputs()), filter.Contains[utxo.OutputID](conflictingInputIDs))

		forkedBranchID := b.CreateBranch(txID, txMetadata.BranchIDs(), branchdag.NewConflictIDs(lo.Map(conflictingOutputIDs, branchdag.NewConflictID)...))

		// We don't need to propagate updates if the branch did already exist.
		// Though CreateBranch needs to be called so that conflict sets and conflict membership are properly updated.
		if txMetadata.BranchIDs().Is(forkedBranchID) {
			return
		}

		// Because we are forking the transaction, automatically all the outputs and the transaction itself need to go
		// into the newly forked branch (own branch) and override all other existing branches. These are now mapped via
		// the BranchDAG (parents of forked branch).
		forkedBranchIDs := branchdag.NewBranchIDs(forkedBranchID)
		outputIds := u.createdOutputIDsOfTransaction(transactionID)
		for _, outputID := range outputIds {
			if !u.CachedOutputMetadata(outputID).Consume(func(outputMetadata *OutputMetadata) {
				outputMetadata.SetBranchIDs(forkedBranchIDs)
			}) {
				panic("failed to load OutputMetadata")
			}
		}

		transactionMetadata.SetBranchIDs(forkedBranchIDs)
		u.Events().TransactionBranchIDUpdatedByFork.Trigger(&TransactionBranchIDUpdatedByForkEvent{
			TransactionID:  transactionID,
			ForkedBranchID: forkedBranchID,
		})

		u.walkFutureCone(outputIds, func(transactionID TransactionID) (updatedOutputs []OutputID) {
			return u.propagateBranch(transactionID, forkedBranchID)
		}, types.True)
	})
}
