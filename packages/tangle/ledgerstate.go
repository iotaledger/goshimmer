package tangle

import (
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// region LedgerState //////////////////////////////////////////////////////////////////////////////////////////////////

// LedgerState is a Tangle component that wraps the components of the ledgerstate package and makes them available at a
// "single point of contact".
type LedgerState struct {
	tangle      *Tangle
	totalSupply uint64

	*ledgerstate.LedgerState
}

// NewLedgerState is the constructor of the LedgerState component.
func NewLedgerState(tangle *Tangle) (ledgerState *LedgerState) {
	return &LedgerState{
		tangle: tangle,
		LedgerState: ledgerstate.New(
			ledgerstate.Store(tangle.Options.Store),
			ledgerstate.CacheTimeProvider(tangle.Options.CacheTimeProvider),
		),
	}
}

// InheritBranch implements the inheritance rules for Branches in the Tangle. It returns a single inherited Branch
// and automatically creates an AggregatedBranch if necessary.
func (l *LedgerState) InheritBranch(referencedBranchIDs ledgerstate.BranchIDs) (inheritedBranch ledgerstate.BranchID, err error) {
	if referencedBranchIDs.Contains(ledgerstate.InvalidBranchID) {
		inheritedBranch = ledgerstate.InvalidBranchID
		return
	}

	cachedAggregatedBranch, _, err := l.BranchDAG.AggregateBranches(referencedBranchIDs)
	if err != nil {
		if errors.Is(err, ledgerstate.ErrInvalidStateTransition) {
<<<<<<< HEAD
			return ledgerstate.InvalidBranchID, nil
=======
			// We book under the InvalidBranch, no error.
			inheritedBranch = ledgerstate.InvalidBranchID
			err = nil
			return
>>>>>>> 8c8d7c5e1e5b82c2aa530a4c45954a9e50af6e11
		}

		err = errors.Errorf("failed to aggregate BranchIDs: %w", err)
		return
	}
	cachedAggregatedBranch.Release()

	inheritedBranch = cachedAggregatedBranch.ID()
	return
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

		// skip transactions that are not confirmed
		var isUnconfirmed bool
		l.TransactionMetadata(transaction.ID()).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
			if !l.tangle.ConfirmationOracle.IsBranchConfirmed(transactionMetadata.BranchID()) {
				isUnconfirmed = true
			}
		})

		if isUnconfirmed {
			continue
		}

		// skip transactions that are too recent before startSnapshot
		if startSnapshot.Sub(transaction.Essence().Timestamp()) < minAge {
			continue
		}
		unspentOutputs := make([]bool, len(transaction.Essence().Outputs()))
		includeTransaction := false
		for i, output := range transaction.Essence().Outputs() {
			unspentOutputs[i] = true
			includeTransaction = true

			confirmedConsumerID := l.ConfirmedConsumer(output.ID())
			if confirmedConsumerID != ledgerstate.GenesisTransactionID {
				tx := copyLedgerState[confirmedConsumerID]
				// If the Confirmed Consumer is old enough we consider the output spent
				if startSnapshot.Sub(tx.Essence().Timestamp()) >= minAge {
					unspentOutputs[i] = false
					includeTransaction = false
				}
			}
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

// CachedTransactionMetadata returns the TransactionMetadata with the given ID.
func (l *LedgerState) CachedTransactionMetadata(transactionID ledgerstate.TransactionID) *ledgerstate.CachedTransactionMetadata {
	return l.UTXODAG.CachedTransactionMetadata(transactionID)
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
func (l *LedgerState) Consumers(outputID ledgerstate.OutputID, optionalSolidityType ...ledgerstate.SolidityType) (cachedTransactions ledgerstate.CachedConsumers) {
	return l.UTXODAG.CachedConsumers(outputID, optionalSolidityType...)
}

// ConfirmedConsumer returns the confirmed transactionID consuming the given outputID.
func (l *LedgerState) ConfirmedConsumer(outputID ledgerstate.OutputID) (consumerID ledgerstate.TransactionID) {
	// default to no consumer, i.e. Genesis
	consumerID = ledgerstate.GenesisTransactionID
	l.Consumers(outputID).Consume(func(consumer *ledgerstate.Consumer) {
		if consumerID != ledgerstate.GenesisTransactionID {
			return
		}
		if l.tangle.ConfirmationOracle.IsTransactionConfirmed(consumer.TransactionID()) {
			consumerID = consumer.TransactionID()
		}
	})
	return
}

// TotalSupply returns the total supply.
func (l *LedgerState) TotalSupply() (totalSupply uint64) {
	return l.totalSupply
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
