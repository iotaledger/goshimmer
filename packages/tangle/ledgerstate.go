package tangle

import (
	"fmt"
	"time"

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
	UTXODAG   ledgerstate.IUTXODAG

	totalSupply uint64
}

// NewLedgerState is the constructor of the LedgerState component.
func NewLedgerState(tangle *Tangle) (ledgerState *LedgerState) {
	branchDAG := ledgerstate.NewBranchDAG(tangle.Options.Store, tangle.Options.CacheTimeProvider)
	return &LedgerState{
		tangle:    tangle,
		BranchDAG: branchDAG,
		UTXODAG:   ledgerstate.NewUTXODAG(tangle.Options.Store, tangle.Options.CacheTimeProvider, branchDAG),
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
	return l.UTXODAG.CachedTransactionMetadata(transactionID)
}

// Transaction retrieves the Transaction with the given TransactionID from the object storage.
func (l *LedgerState) Transaction(transactionID ledgerstate.TransactionID) *ledgerstate.CachedTransaction {
	return l.UTXODAG.CachedTransaction(transactionID)
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
	l.UTXODAG.CachedTransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
		branchID = transactionMetadata.BranchID()
	})
	return
}

// LoadSnapshot creates a set of outputs in the UTXO-DAG, that are forming the genesis for future transactions.
func (l *LedgerState) LoadSnapshot(snapshot *ledgerstate.Snapshot) (err error) {
	l.UTXODAG.LoadSnapshot(snapshot)
	// add attachment link between txs from snapshot and the genesis message (EmptyMessageID).
	for txID, record := range snapshot.Transactions {
		fmt.Println("... Loading snapshot transaction: ", txID, "#outputs=", len(record.Essence.Outputs()), record.UnspentOutputs)
		attachment, _ := l.tangle.Storage.StoreAttachment(txID, EmptyMessageID)
		if attachment != nil {
			attachment.Release()
		}
		for i, output := range record.Essence.Outputs() {
			if !record.UnspentOutputs[i] {
				continue
			}
			output.Balances().ForEach(func(color ledgerstate.Color, balance uint64) bool {
				l.totalSupply += balance
				return true
			})
		}
	}
	attachment, _ := l.tangle.Storage.StoreAttachment(ledgerstate.GenesisTransactionID, EmptyMessageID)
	if attachment != nil {
		attachment.Release()
	}
	return
}

// SnapshotUTXO returns the UTXO snapshot, which is a list of transactions with unspent outputs.
func (l *LedgerState) SnapshotUTXO() (snapshot *ledgerstate.Snapshot) {
	// The following parameter should be larger than the max allowed timestamp variation, and the required time for confirmation.
	// We can snapshot this far in the past, since global snapshots dont occur frequent and it is ok to ignore the last few minutes.
	minAge := 120 * time.Second
	snapshot = &ledgerstate.Snapshot{
		Transactions: make(map[ledgerstate.TransactionID]ledgerstate.Record),
	}

	startSnapshot := time.Now()
	copyLedgerState := l.Transactions() // consider that this may take quite some time

	for _, transaction := range copyLedgerState {
		// skip unconfirmed transactions
		inclusionState, err := l.TransactionInclusionState(transaction.ID())
		if err != nil || inclusionState != ledgerstate.Confirmed {
			continue
		}
		// skip transactions that are too recent before startSnapshot
		if startSnapshot.Sub(transaction.Essence().Timestamp()) < minAge {
			continue
		}
		unspentOutputs := make([]bool, len(transaction.Essence().Outputs()))
		includeTransaction := false
		for i, output := range transaction.Essence().Outputs() {
			l.CachedOutputMetadata(output.ID()).Consume(func(outputMetadata *ledgerstate.OutputMetadata) {
				if outputMetadata.ConfirmedConsumer() == ledgerstate.GenesisTransactionID { // no consumer yet
					unspentOutputs[i] = true
					includeTransaction = true
				} else {
					tx, exist := copyLedgerState[outputMetadata.ConfirmedConsumer()]
					// ignore consumers that are not confirmed long enough or even in the future.
					if !exist || startSnapshot.Sub(tx.Essence().Timestamp()) < minAge {
						unspentOutputs[i] = true
						includeTransaction = true
					}
				}
			})
		}
		// include only transactions with at least one unspent output
		if includeTransaction {
			snapshot.Transactions[transaction.ID()] = ledgerstate.Record{
				Essence:        transaction.Essence(),
				UnlockBlocks:   transaction.UnlockBlocks(),
				UnspentOutputs: unspentOutputs,
			}
		}
	}

	// TODO ??? due to possible race conditions we could add a check for the consistency of the UTXO snapshot

	return snapshot
}

// ReturnTransaction returns a specific transaction.
func (l *LedgerState) ReturnTransaction(transactionID ledgerstate.TransactionID) (transaction *ledgerstate.Transaction) {
	return l.UTXODAG.Transaction(transactionID)
}

// Transactions returns all the transactions.
func (l *LedgerState) Transactions() (transactions map[ledgerstate.TransactionID]*ledgerstate.Transaction) {
	return l.UTXODAG.Transactions()
}

// CachedOutput returns the Output with the given ID.
func (l *LedgerState) CachedOutput(outputID ledgerstate.OutputID) *ledgerstate.CachedOutput {
	return l.UTXODAG.CachedOutput(outputID)
}

// CachedOutputMetadata returns the OutputMetadata with the given ID.
func (l *LedgerState) CachedOutputMetadata(outputID ledgerstate.OutputID) *ledgerstate.CachedOutputMetadata {
	return l.UTXODAG.CachedOutputMetadata(outputID)
}

// CachedOutputsOnAddress retrieves all the Outputs that are associated with an address.
func (l *LedgerState) CachedOutputsOnAddress(address ledgerstate.Address) (cachedOutputs ledgerstate.CachedOutputs) {
	l.UTXODAG.CachedAddressOutputMapping(address).Consume(func(addressOutputMapping *ledgerstate.AddressOutputMapping) {
		cachedOutputs = append(cachedOutputs, l.CachedOutput(addressOutputMapping.OutputID()))
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
	return l.UTXODAG.CachedConsumers(outputID)
}

// TotalSupply returns the total supply.
func (l *LedgerState) TotalSupply() (totalSupply uint64) {
	return l.totalSupply
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
