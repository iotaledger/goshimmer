package tangle

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"golang.org/x/xerrors"
)

// region LedgerState //////////////////////////////////////////////////////////////////////////////////////////////////

// LedgerState is a Tangle component that wraps the components of the ledgerstate package and makes them available at a
// "single point of contact".
type LedgerState struct {
	tangle    *Tangle
	branchDAG *ledgerstate.BranchDAG
	utxoDAG   *ledgerstate.UTXODAG
}

// NewLedgerState is the constructor of the LedgerState component.
func NewLedgerState(tangle *Tangle) (ledgerState *LedgerState) {
	return &LedgerState{
		tangle: tangle,
	}
}

// InheritBranch implements the inheritance rules for Branches in the Tangle. It returns a single inherited Branch
// and automatically creates an AggregatedBranch if necessary.
func (l *LedgerState) InheritBranch(referencedBranchIDs ledgerstate.BranchIDs) (inheritedBranch ledgerstate.BranchID, err error) {
	if referencedBranchIDs.Contains(ledgerstate.InvalidBranchID) {
		inheritedBranch = ledgerstate.InvalidBranchID
		return
	}

	branchIDsContainRejectedBranch, inheritedBranch := l.branchDAG.BranchIDsContainRejectedBranch(referencedBranchIDs)
	if branchIDsContainRejectedBranch {
		return
	}

	cachedAggregatedBranch, _, err := l.branchDAG.AggregateBranches(referencedBranchIDs)
	if err != nil {
		if xerrors.Is(err, ledgerstate.ErrInvalidStateTransition) {
			inheritedBranch = ledgerstate.InvalidBranchID
			return
		}

		err = xerrors.Errorf("failed to aggregate BranchIDs: %w", err)
		return
	}
	cachedAggregatedBranch.Release()

	inheritedBranch = cachedAggregatedBranch.ID()
	return
}

// TransactionValid performs some fast checks of the Transaction and triggers a MessageInvalid event if the checks do
// not pass.
func (l *LedgerState) TransactionValid(transaction *ledgerstate.Transaction, messageID MessageID) (valid bool) {
	valid, err := l.utxoDAG.CheckTransaction(transaction)
	if err != nil {
		l.tangle.Events.MessageInvalid.Trigger(messageID)
	}

	return
}

// BookTransaction books the given Transaction into the underlying LedgerState and returns the target Branch and an
// eventual error.
func (l *LedgerState) BookTransaction(transaction *ledgerstate.Transaction, messageID MessageID) (targetBranch ledgerstate.BranchID, err error) {
	targetBranch, err = l.utxoDAG.BookTransaction(transaction)
	if err != nil {
		if !xerrors.Is(err, ledgerstate.ErrTransactionInvalid) && !xerrors.Is(err, ledgerstate.ErrTransactionNotSolid) {
			err = xerrors.Errorf("failed to book Transaction: %w", err)
			return
		}

		l.tangle.Events.MessageInvalid.Trigger(messageID)

		// non-fatal errors should not bubble up - we trigger a MessageInvalid event instead
		err = nil
		return
	}

	return
}

// TransactionMetadata retrieves the TransactionMetadata with the given TransactionID from the object storage.
func (l *LedgerState) TransactionMetadata(transactionID ledgerstate.TransactionID) (cachedTransactionMetadata *ledgerstate.CachedTransactionMetadata) {
	return l.utxoDAG.TransactionMetadata(transactionID)
}

// ConflictSet returns the list of transactionIDs conflicting with the given transactionID.
func (l *LedgerState) ConflictSet(transactionID ledgerstate.TransactionID) (conflictSet []ledgerstate.TransactionID) {
	conflictIDs := make(ledgerstate.ConflictIDs)
	l.branchDAG.Branch(ledgerstate.NewBranchID(transactionID)).Consume(func(branch ledgerstate.Branch) {
		conflictIDs = branch.(*ledgerstate.ConflictBranch).Conflicts()
	})

	for conflictID := range conflictIDs {
		l.branchDAG.ConflictMembers(conflictID).Consume(func(conflictMember *ledgerstate.ConflictMember) {
			conflictSet = append(conflictSet, ledgerstate.TransactionID(conflictMember.BranchID()))
		})
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
