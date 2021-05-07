package tangle

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// region LedgerState //////////////////////////////////////////////////////////////////////////////////////////////////

// LedgerState is a Tangle component that wraps the components of the ledgerstate package and makes them available at a
// "single point of contact".
type LedgerState struct {
	tangle    *Tangle
	BranchDAG *ledgerstate.BranchDAG
	UTXODAG   *ledgerstate.UTXODAG
}

// NewLedgerState is the constructor of the LedgerState component.
func NewLedgerState(tangle *Tangle) (ledgerState *LedgerState) {
	branchDAG := ledgerstate.NewBranchDAG(tangle.Options.Store)
	return &LedgerState{
		tangle:    tangle,
		BranchDAG: branchDAG,
		UTXODAG:   ledgerstate.NewUTXODAG(tangle.Options.Store, branchDAG),
	}
}

// Shutdown shuts down the LedgerState and persists its state.
func (l *LedgerState) Shutdown() {
	l.UTXODAG.Shutdown()
	l.BranchDAG.Shutdown()
}

// InheritBranch implements the inheritance rules for Branches in the Tangle. It returns a single inherited Branch
// and automatically creates an AggregatedBranch if necessary.
func (l *LedgerState) InheritBranch(referencedBranchIDs ledgerstate.BranchIDs) (inheritedBranch ledgerstate.BranchID, err error) {
	if referencedBranchIDs.Contains(ledgerstate.InvalidBranchID) {
		inheritedBranch = ledgerstate.InvalidBranchID
		return
	}

	branchIDsContainRejectedBranch, inheritedBranch := l.BranchDAG.BranchIDsContainRejectedBranch(referencedBranchIDs)
	if branchIDsContainRejectedBranch {
		return
	}

	cachedAggregatedBranch, _, err := l.BranchDAG.AggregateBranches(referencedBranchIDs)
	if err != nil {
		if errors.Is(err, ledgerstate.ErrInvalidStateTransition) {
			inheritedBranch = ledgerstate.InvalidBranchID
			err = nil
			return
		}

		err = errors.Errorf("failed to aggregate BranchIDs: %w", err)
		return
	}
	cachedAggregatedBranch.Release()

	inheritedBranch = cachedAggregatedBranch.ID()
	return
}

// TransactionValid performs some fast checks of the Transaction and triggers a MessageInvalid event if the checks do
// not pass.
func (l *LedgerState) TransactionValid(transaction *ledgerstate.Transaction, messageID MessageID) (err error) {
	if err = l.UTXODAG.CheckTransaction(transaction); err != nil {
		l.tangle.Storage.MessageMetadata(messageID).Consume(func(messagemetadata *MessageMetadata) {
			messagemetadata.SetInvalid(true)
		})
		l.tangle.Events.MessageInvalid.Trigger(messageID)

		return errors.Errorf("invalid transaction in message with %s: %w", messageID, err)
	}

	return nil
}

// TransactionConflicting returns whether the given transaction is part of a conflict.
func (l *LedgerState) TransactionConflicting(transactionID ledgerstate.TransactionID) bool {
	return l.BranchID(transactionID) == ledgerstate.NewBranchID(transactionID)
}

// TransactionMetadata retrieves the TransactionMetadata with the given TransactionID from the object storage.
func (l *LedgerState) TransactionMetadata(transactionID ledgerstate.TransactionID) (cachedTransactionMetadata *ledgerstate.CachedTransactionMetadata) {
	return l.UTXODAG.TransactionMetadata(transactionID)
}

// Transaction retrieves the Transaction with the given TransactionID from the object storage.
func (l *LedgerState) Transaction(transactionID ledgerstate.TransactionID) *ledgerstate.CachedTransaction {
	return l.UTXODAG.Transaction(transactionID)
}

// BookTransaction books the given Transaction into the underlying LedgerState and returns the target Branch and an
// eventual error.
func (l *LedgerState) BookTransaction(transaction *ledgerstate.Transaction, messageID MessageID) (targetBranch ledgerstate.BranchID, err error) {
	targetBranch, err = l.UTXODAG.BookTransaction(transaction)
	if err != nil {
		if !errors.Is(err, ledgerstate.ErrTransactionInvalid) && !errors.Is(err, ledgerstate.ErrTransactionNotSolid) {
			err = errors.Errorf("failed to book Transaction: %w", err)
			return
		}

		l.tangle.Storage.MessageMetadata(messageID).Consume(func(messagemetadata *MessageMetadata) {
			messagemetadata.SetInvalid(true)
		})
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

	l.BranchDAG.Branch(ledgerstate.NewBranchID(transactionID)).Consume(func(branch ledgerstate.Branch) {
		conflictIDs = branch.(*ledgerstate.ConflictBranch).Conflicts()
	})

	for conflictID := range conflictIDs {
		l.BranchDAG.ConflictMembers(conflictID).Consume(func(conflictMember *ledgerstate.ConflictMember) {
			conflictSet[ledgerstate.TransactionID(conflictMember.BranchID())] = types.Void
		})
	}

	return
}

// TransactionInclusionState returns the InclusionState of the Transaction with the given TransactionID which can either be
// Pending, Confirmed or Rejected.
func (l *LedgerState) TransactionInclusionState(transactionID ledgerstate.TransactionID) (ledgerstate.InclusionState, error) {
	return l.UTXODAG.InclusionState(transactionID)
}

// BranchInclusionState returns the InclusionState of the Branch with the given BranchID which can either be
// Pending, Confirmed or Rejected.
func (l *LedgerState) BranchInclusionState(branchID ledgerstate.BranchID) (inclusionState ledgerstate.InclusionState) {
	l.BranchDAG.Branch(branchID).Consume(func(branch ledgerstate.Branch) {
		inclusionState = branch.InclusionState()
	})
	return
}

// BranchID returns the branchID of the given transactionID.
func (l *LedgerState) BranchID(transactionID ledgerstate.TransactionID) (branchID ledgerstate.BranchID) {
	l.UTXODAG.TransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
		branchID = transactionMetadata.BranchID()
	})
	return
}

// LoadSnapshot creates a set of outputs in the UTXO-DAG, that are forming the genesis for future transactions.
func (l *LedgerState) LoadSnapshot(snapshot *ledgerstate.Snapshot) {
	l.UTXODAG.LoadSnapshot(snapshot)
	for txID := range snapshot.Transactions {
		attachment, _ := l.tangle.Storage.StoreAttachment(txID, EmptyMessageID)
		if attachment != nil {
			attachment.Release()
		}
	}
	attachment, _ := l.tangle.Storage.StoreAttachment(ledgerstate.GenesisTransactionID, EmptyMessageID)
	if attachment != nil {
		attachment.Release()
	}
}

// Output returns the Output with the given ID.
func (l *LedgerState) Output(outputID ledgerstate.OutputID) *ledgerstate.CachedOutput {
	return l.UTXODAG.Output(outputID)
}

// OutputMetadata returns the OutputMetadata with the given ID.
func (l *LedgerState) OutputMetadata(outputID ledgerstate.OutputID) *ledgerstate.CachedOutputMetadata {
	return l.UTXODAG.OutputMetadata(outputID)
}

// OutputsOnAddress retrieves all the Outputs that are associated with an address.
func (l *LedgerState) OutputsOnAddress(address ledgerstate.Address) (cachedOutputs ledgerstate.CachedOutputs) {
	l.UTXODAG.AddressOutputMapping(address).Consume(func(addressOutputMapping *ledgerstate.AddressOutputMapping) {
		cachedOutputs = append(cachedOutputs, l.Output(addressOutputMapping.OutputID()))
	})
	return
}

// CheckTransaction contains fast checks that have to be performed before booking a Transaction.
func (l *LedgerState) CheckTransaction(transaction *ledgerstate.Transaction) (err error) {
	return l.UTXODAG.CheckTransaction(transaction)
}

// ConsumedOutputs returns the consumed (cached)Outputs of the given Transaction.
func (l *LedgerState) ConsumedOutputs(transaction *ledgerstate.Transaction) (cachedInputs ledgerstate.CachedOutputs) {
	return l.UTXODAG.ConsumedOutputs(transaction)
}

// Consumers returns the (cached) consumers of the given outputID.
func (l *LedgerState) Consumers(outputID ledgerstate.OutputID) (cachedTransactions ledgerstate.CachedConsumers) {
	return l.UTXODAG.Consumers(outputID)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
