package tangle

import (
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// region LedgerState //////////////////////////////////////////////////////////////////////////////////////////////////

// LedgerState is a Tangle component that wraps the components of the ledgerstate package and makes them available at a
// "single point of contact".
type LedgerState struct {
	tangle      *Tangle
	totalSupply uint64

	*ledgerstate.Ledgerstate
}

// NewLedgerState is the constructor of the LedgerState component.
func NewLedgerState(tangle *Tangle) (ledgerState *LedgerState) {
	return &LedgerState{
		tangle: tangle,
		Ledgerstate: ledgerstate.New(
			ledgerstate.Store(tangle.Options.Store),
			ledgerstate.CacheTimeProvider(tangle.Options.CacheTimeProvider),
		),
	}
}

// Setup sets up the behavior of the component by making it attach to the relevant events of other components.
func (l *LedgerState) Setup() {
	l.tangle.ConfirmationOracle.Events().BranchConfirmed.Attach(events.NewClosure(func(branchID ledgerstate.BranchID) {
		if l.tangle.Options.LedgerState.MergeBranches {
			l.SetBranchConfirmed(branchID)
		}
	}))
}

// Shutdown shuts down the LedgerState and persists its state.
func (l *LedgerState) Shutdown() {
	l.UTXODAG.Shutdown()
	l.BranchDAG.Shutdown()
}

// TransactionValid performs some fast checks of the Transaction and triggers a MessageInvalid event if the checks do
// not pass.
func (l *LedgerState) TransactionValid(transaction *ledgerstate.Transaction, messageID MessageID) (err error) {
	if err = l.UTXODAG.CheckTransaction(transaction); err != nil {
		l.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
			messageMetadata.SetObjectivelyInvalid(true)
		})
		l.tangle.Events.MessageInvalid.Trigger(&MessageInvalidEvent{MessageID: messageID, Error: err})

		return errors.Errorf("invalid transaction in message with %s: %w", messageID, err)
	}

	return nil
}

// TransactionConflicting returns whether the given transaction is part of a conflict.
func (l *LedgerState) TransactionConflicting(transactionID ledgerstate.TransactionID) bool {
	branchIDs := l.BranchIDs(transactionID)
	return len(branchIDs) == 1 && branchIDs.Contains(ledgerstate.NewBranchID(transactionID))
}

// TransactionMetadata retrieves the TransactionMetadata with the given TransactionID from the object storage.
func (l *LedgerState) TransactionMetadata(transactionID ledgerstate.TransactionID) (cachedTransactionMetadata *objectstorage.CachedObject[*ledgerstate.TransactionMetadata]) {
	return l.UTXODAG.CachedTransactionMetadata(transactionID)
}

// Transaction retrieves the Transaction with the given TransactionID from the object storage.
func (l *LedgerState) Transaction(transactionID ledgerstate.TransactionID) *objectstorage.CachedObject[*ledgerstate.Transaction] {
	return l.UTXODAG.CachedTransaction(transactionID)
}

// BookTransaction books the given Transaction into the underlying LedgerState and returns the target Branch and an
// eventual error.
func (l *LedgerState) BookTransaction(transaction *ledgerstate.Transaction, messageID MessageID) (targetBranches ledgerstate.BranchIDs, err error) {
	targetBranches, err = l.UTXODAG.BookTransaction(transaction)
	if err != nil {
		err = errors.Errorf("failed to book Transaction: %w", err)

		l.tangle.Storage.MessageMetadata(messageID).Consume(func(messagemetadata *MessageMetadata) {
			messagemetadata.SetObjectivelyInvalid(true)
		})
		l.tangle.Events.MessageInvalid.Trigger(&MessageInvalidEvent{MessageID: messageID, Error: err})

		return
	}

	return
}

// ConflictSet returns the list of transactionIDs conflicting with the given transactionID.
func (l *LedgerState) ConflictSet(transactionID ledgerstate.TransactionID) (conflictSet ledgerstate.TransactionIDs) {
	conflictIDs := make(ledgerstate.ConflictIDs)
	conflictSet = make(ledgerstate.TransactionIDs)

	l.BranchDAG.Branch(ledgerstate.NewBranchID(transactionID)).Consume(func(branch *ledgerstate.Branch) {
		conflictIDs = branch.Conflicts()
	})

	for conflictID := range conflictIDs {
		l.BranchDAG.ConflictMembers(conflictID).Consume(func(conflictMember *ledgerstate.ConflictMember) {
			conflictSet[ledgerstate.TransactionID(conflictMember.BranchID())] = types.Void
		})
	}

	return
}

// BranchIDs returns the branchIDs of the given transactionID.
func (l *LedgerState) BranchIDs(transactionID ledgerstate.TransactionID) (branchIDs ledgerstate.BranchIDs) {
	l.UTXODAG.CachedTransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
		branchIDs = transactionMetadata.BranchIDs()
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
	// We can snapshot this far in the past, since global snapshots don't occur frequent, and it is ok to ignore the last few minutes.
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
			for branchID := range transactionMetadata.BranchIDs() {
				if !l.tangle.ConfirmationOracle.IsBranchConfirmed(branchID) {
					isUnconfirmed = true
					break
				}
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
				tx, exists := copyLedgerState[confirmedConsumerID]
				// If the Confirmed Consumer is old enough we consider the output spent
				if exists && startSnapshot.Sub(tx.Essence().Timestamp()) >= minAge {
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
func (l *LedgerState) CachedOutput(outputID ledgerstate.OutputID) *objectstorage.CachedObject[ledgerstate.Output] {
	return l.UTXODAG.CachedOutput(outputID)
}

// CachedOutputMetadata returns the OutputMetadata with the given ID.
func (l *LedgerState) CachedOutputMetadata(outputID ledgerstate.OutputID) *objectstorage.CachedObject[*ledgerstate.OutputMetadata] {
	return l.UTXODAG.CachedOutputMetadata(outputID)
}

// CachedTransactionMetadata returns the TransactionMetadata with the given ID.
func (l *LedgerState) CachedTransactionMetadata(transactionID ledgerstate.TransactionID) *objectstorage.CachedObject[*ledgerstate.TransactionMetadata] {
	return l.UTXODAG.CachedTransactionMetadata(transactionID)
}

// CachedOutputsOnAddress retrieves all the Outputs that are associated with an address.
func (l *LedgerState) CachedOutputsOnAddress(address ledgerstate.Address) (cachedOutputs objectstorage.CachedObjects[ledgerstate.Output]) {
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
func (l *LedgerState) ConsumedOutputs(transaction *ledgerstate.Transaction) (cachedInputs objectstorage.CachedObjects[ledgerstate.Output]) {
	return l.UTXODAG.ConsumedOutputs(transaction)
}

// Consumers returns the (cached) consumers of the given outputID.
func (l *LedgerState) Consumers(outputID ledgerstate.OutputID) (cachedTransactions objectstorage.CachedObjects[*ledgerstate.Consumer]) {
	return l.UTXODAG.CachedConsumers(outputID)
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
