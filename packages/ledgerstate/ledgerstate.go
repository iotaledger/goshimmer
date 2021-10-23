package ledgerstate

import (
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

// MergeToMaster merges a confirmed Branch back into the MasterBranch.
func (l *Ledgerstate) MergeToMaster(branchID BranchID) (err error) {
	// lock other writes

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

			transactionMetadata.SetBranchID(updatedBranches[currentBranchID])

			l.UTXODAG.CachedTransaction(currentTransactionID).Consume(func(transaction *Transaction) {
				for _, output := range transaction.Essence().Outputs() {
					l.UTXODAG.CachedOutputMetadata(output.ID()).Consume(func(outputMetadata *OutputMetadata) {
						outputMetadata.SetBranchID(updatedBranches[currentBranchID])
					})

					l.UTXODAG.CachedConsumers(output.ID(), Solid).Consume(func(consumer *Consumer) {
						transactionWalker.Push(consumer.TransactionID())
					})
				}
			})
		})
	}

	return nil
}

// Shutdown marks the Ledgerstate as stopped, so it will not accept any new Transactions.
func (l *Ledgerstate) Shutdown() {
	l.BranchDAG.Shutdown()
	l.UTXODAG.Shutdown()
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
