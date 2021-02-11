package tangle

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/types"
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
	branchDAG := ledgerstate.NewBranchDAG(tangle.Options.Store)
	return &LedgerState{
		tangle:    tangle,
		branchDAG: branchDAG,
		utxoDAG:   ledgerstate.NewUTXODAG(tangle.Options.Store, branchDAG),
	}
}

// Shutdown shuts down the LedgerState and persists its state.
func (l *LedgerState) Shutdown() {
	l.utxoDAG.Shutdown()
	l.branchDAG.Shutdown()
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

// TransactionConflicting returns whether the given transaction is part of a conflict.
func (l *LedgerState) TransactionConflicting(transactionID ledgerstate.TransactionID) bool {
	return l.BranchID(transactionID) == ledgerstate.NewBranchID(transactionID)
}

// TransactionMetadata retrieves the TransactionMetadata with the given TransactionID from the object storage.
func (l *LedgerState) TransactionMetadata(transactionID ledgerstate.TransactionID) (cachedTransactionMetadata *ledgerstate.CachedTransactionMetadata) {
	return l.utxoDAG.TransactionMetadata(transactionID)
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

// ConflictSet returns the list of transactionIDs conflicting with the given transactionID.
func (l *LedgerState) ConflictSet(transactionID ledgerstate.TransactionID) (conflictSet ledgerstate.TransactionIDs) {
	conflictIDs := make(ledgerstate.ConflictIDs)
	conflictSet = make(ledgerstate.TransactionIDs)

	l.branchDAG.Branch(ledgerstate.NewBranchID(transactionID)).Consume(func(branch ledgerstate.Branch) {
		conflictIDs = branch.(*ledgerstate.ConflictBranch).Conflicts()
	})

	for conflictID := range conflictIDs {
		l.branchDAG.ConflictMembers(conflictID).Consume(func(conflictMember *ledgerstate.ConflictMember) {
			conflictSet[ledgerstate.TransactionID(conflictMember.BranchID())] = types.Void
		})
	}

	return
}

// TransactionInclusionState returns the InclusionState of the Transaction with the given TransactionID which can either be
// Pending, Confirmed or Rejected.
func (l *LedgerState) TransactionInclusionState(transactionID ledgerstate.TransactionID) (ledgerstate.InclusionState, error) {
	return l.utxoDAG.InclusionState(transactionID)
}

// BranchInclusionState returns the InclusionState of the Branch with the given BranchID which can either be
// Pending, Confirmed or Rejected.
func (l *LedgerState) BranchInclusionState(branchID ledgerstate.BranchID) (inclusionState ledgerstate.InclusionState) {
	l.branchDAG.Branch(branchID).Consume(func(branch ledgerstate.Branch) {
		inclusionState = branch.InclusionState()
	})
	return
}

// BranchID returns the branchID of the given transactionID.
func (l *LedgerState) BranchID(transactionID ledgerstate.TransactionID) (branchID ledgerstate.BranchID) {
	l.utxoDAG.TransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
		branchID = transactionMetadata.BranchID()
	})
	return
}

// LoadSnapshot creates a set of outputs in the UTXO-DAG, that are forming the genesis for future transactions.
func (l *LedgerState) LoadSnapshot(snapshot map[ledgerstate.TransactionID]map[ledgerstate.Address]*ledgerstate.ColoredBalances) {
	l.utxoDAG.LoadSnapshot(snapshot)
	attachment, _ := l.tangle.Storage.StoreAttachment(ledgerstate.GenesisTransactionID, EmptyMessageID)
	if attachment != nil {
		attachment.Release()
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
