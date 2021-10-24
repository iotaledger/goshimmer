package ledgerstate

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"

	"github.com/iotaledger/goshimmer/packages/database"
)

// region Ledgerstate //////////////////////////////////////////////////////////////////////////////////////////////////

// Ledgerstate is a data structure that follows the principles of the quadruple entry accounting.
type Ledgerstate struct {
	Options *Options

	*UTXODAG
	*BranchDAG
	ConfirmationOracle
	sync.RWMutex
}

// New is the constructor for the Ledgerstate.
func New(options ...Option) (ledgerstate *Ledgerstate) {
	ledgerstate = &Ledgerstate{}
	ledgerstate.Configure(options...)

	ledgerstate.UTXODAG = NewUTXODAG(ledgerstate)
	ledgerstate.BranchDAG = NewBranchDAG(ledgerstate)
	ledgerstate.ConfirmationOracle = NewSimpleConfirmationOracle(ledgerstate)

	return ledgerstate
}

// Configure modifies the configuration of the Ledgerstate.
func (l *Ledgerstate) Configure(options ...Option) {
	if l.Options == nil {
		l.Options = &Options{
			Store:              mapdb.NewMapDB(),
			LazyBookingEnabled: true,
		}
	}

	for _, option := range options {
		option(l.Options)
	}
}

// StoreTransaction adds a new Transaction to the ledger state. It returns a boolean that indicates whether the
// Transaction was stored, its SolidityType and an error value that contains the cause for possibly exceptions.
func (l *Ledgerstate) StoreTransaction(transaction *Transaction) (stored bool, solidityType SolidityType, err error) {
	l.RLock()
	defer l.RUnlock()

	return l.UTXODAG.StoreTransaction(transaction)
}

// MergeToMaster merges a Branch (and all of its ancestors) back into the MasterBranch. It reorganizes the BranchDAG on
// top of this Branch and reassigns the related transactions to these new Branches.
func (l *Ledgerstate) MergeToMaster(branchID BranchID) (err error) {
	l.Lock()
	defer l.Unlock()

	branchesToMergeInOrder, err := l.branchesToMergeInOrder(branchID)
	if err != nil {
		return errors.Errorf("failed to determine Branches to merge in order: %w", err)
	}

	for _, currentBranchID := range branchesToMergeInOrder {
		if err = l.mergeSingleBranchToMaster(currentBranchID); err != nil {
			return errors.Errorf("failed to merge %s to MasterBranch: %w", currentBranchID, err)
		}
	}

	return nil
}

// Shutdown marks the Ledgerstate as stopped, so it will not accept any new Transactions.
func (l *Ledgerstate) Shutdown() {
	l.BranchDAG.Shutdown()
	l.UTXODAG.Shutdown()
}

// branchesToMergeInOrder returns the given Branch and all of its ancestors in their correct order for merging (leafs
// last).
func (l *Ledgerstate) branchesToMergeInOrder(branchID BranchID) (branchesInMergeOrder []BranchID, err error) {
	conflictBranchIDs, err := l.BranchDAG.ResolveConflictBranchIDs(NewBranchIDs(branchID))
	if err != nil {
		return nil, errors.Errorf("failed to resolve ConflictBranchIDs of Branch with %s: %w", branchID, err)
	}

	branchWalker := walker.New(false)
	for conflictBranchID := range conflictBranchIDs {
		branchWalker.Push(conflictBranchID)
	}

	for branchWalker.HasNext() {
		currentBranchID := branchWalker.Next().(BranchID)
		branchesInMergeOrder = append(branchesInMergeOrder, currentBranchID)

		l.BranchDAG.Branch(currentBranchID).Consume(func(branch Branch) {
			for parentBranchID := range branch.Parents() {
				if parentBranchID == MasterBranchID {
					continue
				}

				branchWalker.Push(parentBranchID)
			}
		})
	}

	for i := len(branchesInMergeOrder)/2 - 1; i >= 0; i-- {
		opp := len(branchesInMergeOrder) - 1 - i
		branchesInMergeOrder[i], branchesInMergeOrder[opp] = branchesInMergeOrder[opp], branchesInMergeOrder[i]
	}

	return
}

// mergeSingleBranchToMaster merges a single Branch that is at the bottom of the BranchDAG (a child of the root) into
// the MasterBranch. It reorganizes the BranchDAG on top of this Branch and reassigns the related transactions to these
// new Branches.
func (l *Ledgerstate) mergeSingleBranchToMaster(branchID BranchID) (err error) {
	updatedBranches, err := l.BranchDAG.MergeToMaster(branchID)
	if err != nil {
		return errors.Errorf("failed to merge Branch with %s to %s: %w", branchID, MasterBranchID, err)
	}

	transactionWalker := walker.New(false)
	transactionWalker.Push(branchID.TransactionID())

	for transactionWalker.HasNext() {
		currentTransactionID := transactionWalker.Next().(TransactionID)

		l.UTXODAG.CachedTransactionMetadata(currentTransactionID).Consume(func(transactionMetadata *TransactionMetadata) {
			currentBranchID := transactionMetadata.BranchID()
			if currentBranchID == InvalidBranchID {
				return
			}

			updatedBranchID, exists := updatedBranches[currentBranchID]
			if !exists {
				return
			}

			transactionMetadata.SetBranchID(updatedBranchID)

			l.UTXODAG.CachedTransaction(currentTransactionID).Consume(func(transaction *Transaction) {
				for _, output := range transaction.Essence().Outputs() {
					l.UTXODAG.CachedOutputMetadata(output.ID()).Consume(func(outputMetadata *OutputMetadata) {
						outputMetadata.SetBranchID(updatedBranchID)
					})

					l.UTXODAG.CachedConsumers(output.ID(), Solid).Consume(func(consumer *Consumer) {
						transactionWalker.Push(consumer.TransactionID())
					})
				}
			})
		})
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// Option represents the return type of optional parameters that can be handed into the constructor of the Ledgerstate
// to configure its behavior.
type Option func(*Options)

// Options is a container for all configurable parameters of the Ledgerstate.
type Options struct {
	Store              kvstore.KVStore
	CacheTimeProvider  *database.CacheTimeProvider
	LazyBookingEnabled bool
}

// Store is an Option for the Ledgerstate that allows to specify which storage layer is supposed to be used to persist
// data.
func Store(store kvstore.KVStore) Option {
	return func(options *Options) {
		options.Store = store
	}
}

// CacheTimeProvider is an Option for the Tangle that allows to override hard coded cache time.
func CacheTimeProvider(cacheTimeProvider *database.CacheTimeProvider) Option {
	return func(options *Options) {
		options.CacheTimeProvider = cacheTimeProvider
	}
}

// LazyBookingEnabled is an Option for the Ledgerstate that allows to specify if the ledger state should lazy book
// conflicts that look like they have been decided already.
func LazyBookingEnabled(enabled bool) Option {
	return func(options *Options) {
		options.LazyBookingEnabled = enabled
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
