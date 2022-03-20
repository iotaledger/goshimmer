package ledger

import (
	"github.com/iotaledger/hive.go/generics/dataflow"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/generics/walker"

	. "github.com/iotaledger/goshimmer/packages/refactored/g"
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

	params.OutputsMetadata = KeyBy(cachedOutputsMetadata.Unwrap(), (*OutputMetadata).ID)

	return next(params)
}

func (b *Booker) bookTransaction(txID utxo.TransactionID, txMetadata *TransactionMetadata, inputsMetadata map[utxo.OutputID]*OutputMetadata) (inheritedBranchIDs branchdag.BranchIDs) {
	inheritedBranchIDs = b.RemoveConfirmedBranches(ReduceProperty(Values(inputsMetadata), (*OutputMetadata).BranchIDs, branchdag.BranchIDs.AddAll, branchdag.NewBranchIDs()))

	conflictingInputIDs := Keys(FilterByValue(inputsMetadata, Bind((*OutputMetadata).IsProcessedConsumerDoubleSpend, txID)))
	if len(conflictingInputIDs) != 0 {
		inheritedBranchIDs = branchdag.NewBranchIDs(b.CreateBranch(txID, inheritedBranchIDs, branchdag.NewConflictIDs(Map(conflictingInputIDs, branchdag.NewConflictID)...)))

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
		b.forkConsumer(txID, conflictingInputIDs)
	})
}
