package ledger

import (
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/dataflow"
	"github.com/iotaledger/hive.go/generics/walker"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/refactored/generics"
	"github.com/iotaledger/goshimmer/packages/refactored/ledger/branchdag"
	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

type Booker struct {
	*Ledger
}

func (b *Booker) bookTransactionCommand(params *params, next dataflow.Next[*params]) (err error) {
	// check the double spends first because at this step we also create the consumers
	conflictingInputIDs := generics.Keys(generics.FilterByValue(params.InputsMetadata, b.doubleSpendRegistered(params.Transaction.ID())))

	inheritedBranchIDs, err := b.ResolvePendingBranchIDs(generics.Reduce(generics.Values(params.InputsMetadata), b.accumulateBranchIDs, branchdag.NewBranchIDs()))
	if err != nil {
		return errors.Errorf("failed to resolve pending branches: %w", err)
	}

	if len(conflictingInputIDs) != 0 {
		b.WalkConsumingTransactionID(conflictingInputIDs, func(txID utxo.TransactionID, _ *walker.Walker[utxo.OutputID]) {
			b.forkConsumer(txID, conflictingInputIDs)
			return
		})

		newBranchID := branchdag.NewBranchID(params.Transaction.ID())
		inheritedBranchIDs = branchdag.NewBranchIDs(newBranchID)
		cachedBranch, _, err := b.CreateBranch(newBranchID, inheritedBranchIDs, conflictingInputIDs)
		if err != nil {
			panic(fmt.Errorf("failed to create Branch when booking Transaction with %s: %w", params.Transaction.ID(), err))
		}
		cachedBranch.Release()
	}

	b.bookTransaction(params.Transaction, params.TransactionMetadata, params.InputsMetadata, inheritedBranchIDs)

	return next(params)
}

func (b *Booker) accumulateBranchIDs(accumulator branchdag.BranchIDs, inputMetadata *OutputMetadata) (result branchdag.BranchIDs) {
	return accumulator.AddAll(inputMetadata.BranchIDs())
}

func (b *Booker) doubleSpendRegistered(txID utxo.TransactionID) func(*OutputMetadata) bool {
	return func(outputMetadata *OutputMetadata) (conflicting bool) {
		outputMetadata.RegisterConsumer(txID)

		b.bookConsumers(inputsMetadata, transaction.ID(), types.True)

		return false
	}
}

// bookNonConflictingTransaction is an internal utility function that books the Transaction into the Branch that is
// determined by aggregating the Branches of the consumed Inputs.
func (u *Booker) bookTransaction(transaction utxo.Transaction, transactionMetadata *TransactionMetadata, inputsMetadata map[utxo.OutputID]*OutputMetadata, branchIDs branchdag.BranchIDs) (targetBranchIDs branchdag.BranchIDs) {
	u.bookOutputs(transaction, branchIDs)

	transactionMetadata.SetBranchIDs(branchIDs)
	transactionMetadata.SetSolid(true)

	return branchIDs
}

//
