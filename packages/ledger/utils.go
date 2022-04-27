package ledger

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/generics/lo"
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/generics/walker"

	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/ledger/branchdag"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
)

// Utils is a Ledger component that bundles utility related API to simplify common interactions with the Ledger.
type Utils struct {
	// ledger contains a reference to the Ledger that created the Utils.
	ledger *Ledger
}

// newUtils returns a new Utils instance for the given Ledger.
func newUtils(ledger *Ledger) (new *Utils) {
	return &Utils{
		ledger: ledger,
	}
}

// ResolveInputs returns the OutputIDs that were referenced by the given Inputs.
func (u *Utils) ResolveInputs(inputs []utxo.Input) (outputIDs utxo.OutputIDs) {
	return utxo.NewOutputIDs(lo.Map(inputs, u.ledger.options.vm.ResolveInput)...)
}

// UnprocessedConsumingTransactions returns the unprocessed consuming transactions of the named OutputIDs.
func (u *Utils) UnprocessedConsumingTransactions(outputIDs utxo.OutputIDs) (consumingTransactions utxo.TransactionIDs) {
	consumingTransactions = utxo.NewTransactionIDs()
	for it := outputIDs.Iterator(); it.HasNext(); {
		u.ledger.Storage.CachedConsumers(it.Next()).Consume(func(consumer *Consumer) {
			if consumer.IsBooked() {
				return
			}

			consumingTransactions.Add(consumer.TransactionID())
		})
	}

	return consumingTransactions
}

// WalkConsumingTransactionID walks over the TransactionIDs that consume the named OutputIDs.
func (u *Utils) WalkConsumingTransactionID(entryPoints utxo.OutputIDs, callback func(consumingTxID utxo.TransactionID, walker *walker.Walker[utxo.OutputID])) {
	if entryPoints.Size() == 0 {
		return
	}

	seenTransactions := set.New[utxo.TransactionID](false)
	futureConeWalker := walker.New[utxo.OutputID](false).PushAll(entryPoints.Slice()...)
	for futureConeWalker.HasNext() {
		u.ledger.Storage.CachedConsumers(futureConeWalker.Next()).Consume(func(consumer *Consumer) {
			if futureConeWalker.WalkStopped() || !seenTransactions.Add(consumer.TransactionID()) {
				return
			}

			callback(consumer.TransactionID(), futureConeWalker)
		})
	}
}

// WalkConsumingTransactionMetadata walks over the transactions that consume the named OutputIDs and calls the callback
// with their corresponding TransactionMetadata.
func (u *Utils) WalkConsumingTransactionMetadata(entryPoints utxo.OutputIDs, callback func(txMetadata *TransactionMetadata, walker *walker.Walker[utxo.OutputID])) {
	u.WalkConsumingTransactionID(entryPoints, func(consumingTxID utxo.TransactionID, walker *walker.Walker[utxo.OutputID]) {
		u.ledger.Storage.CachedTransactionMetadata(consumingTxID).Consume(func(txMetadata *TransactionMetadata) {
			callback(txMetadata, walker)
		})
	})
}

// WithTransactionAndMetadata walks over the transactions that consume the named OutputIDs and calls the callback
// with their corresponding Transaction and TransactionMetadata.
func (u *Utils) WithTransactionAndMetadata(txID utxo.TransactionID, callback func(tx utxo.Transaction, txMetadata *TransactionMetadata)) {
	u.ledger.Storage.CachedTransaction(txID).Consume(func(tx utxo.Transaction) {
		u.ledger.Storage.CachedTransactionMetadata(txID).Consume(func(txMetadata *TransactionMetadata) {
			callback(tx, txMetadata)
		})
	})
}

// TransactionBranchIDs returns the BranchIDs of the given TransactionID.
func (u *Utils) TransactionBranchIDs(txID utxo.TransactionID) (branchIDs branchdag.BranchIDs, err error) {
	branchIDs = branchdag.NewBranchIDs()
	if !u.ledger.Storage.CachedTransactionMetadata(txID).Consume(func(metadata *TransactionMetadata) {
		branchIDs = metadata.BranchIDs()
	}) {
		return nil, errors.Errorf("failed to load TransactionMetadata with %s: %w", txID, cerrors.ErrFatal)
	}

	return branchIDs, nil
}

func (u *Utils) ReferencedTransactions(tx utxo.Transaction) (transactionIDs utxo.TransactionIDs) {
	transactionIDs = utxo.NewTransactionIDs()
	u.ledger.Storage.CachedOutputs(u.ResolveInputs(tx.Inputs())).Consume(func(output utxo.Output) {
		transactionIDs.Add(output.ID().TransactionID)
	})
	return transactionIDs
}

// ConflictingTransactions returns the TransactionIDs that are conflicting with the given Transaction.
func (u *Utils) ConflictingTransactions(transactionID utxo.TransactionID) (conflictingTransactions utxo.TransactionIDs) {
	conflictingTransactions = utxo.NewTransactionIDs()

	u.ledger.BranchDAG.Storage.CachedBranch(branchdag.NewBranchID(transactionID)).Consume(func(branch *branchdag.Branch) {
		for it := branch.ConflictIDs().Iterator(); it.HasNext(); {
			u.ledger.BranchDAG.Storage.CachedConflictMembers(it.Next()).Consume(func(conflictMember *branchdag.ConflictMember) {
				conflictingTransactions.Add(utxo.TransactionID{conflictMember.BranchID().Identifier})
			})
		}
	})

	return
}

// TransactionGradeOfFinality returns the GradeOfFinality of the Transaction with the given TransactionID.
func (u *Utils) TransactionGradeOfFinality(txID utxo.TransactionID) (gradeOfFinality gof.GradeOfFinality, err error) {
	if !u.ledger.Storage.CachedTransactionMetadata(txID).Consume(func(txMetadata *TransactionMetadata) {
		gradeOfFinality = txMetadata.GradeOfFinality()
	}) {
		return gof.None, errors.Errorf("failed to load TransactionMetadata with %s: %w", txID, cerrors.ErrFatal)
	}

	return
}

// BranchGradeOfFinality returns the GradeOfFinality of the Branch with the given BranchID.
func (u *Utils) BranchGradeOfFinality(branchID branchdag.BranchID) (gradeOfFinality gof.GradeOfFinality, err error) {
	if branchID == branchdag.MasterBranchID {
		return gof.High, nil
	}

	branchGof, gofErr := u.TransactionGradeOfFinality(utxo.TransactionID{branchID.Identifier})
	if gofErr != nil {
		return gof.None, errors.Errorf("failed to retrieve GoF of branch %s: %w", branchID, err)
	}

	return branchGof, nil
}
