package ledger

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/lo"
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/generics/walker"

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
func (u *Utils) WithTransactionAndMetadata(txID utxo.TransactionID, callback func(tx *Transaction, txMetadata *TransactionMetadata)) {
	u.ledger.Storage.CachedTransaction(txID).Consume(func(tx *Transaction) {
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
		return nil, errors.New("no TransactionMetadata found")
	}

	return branchIDs, nil
}

// ConflictingTransactions returns the TransactionIDs that are conflicting with the given Transaction.
func (u *Utils) ConflictingTransactions(transaction utxo.Transaction) (conflictingTransactions utxo.TransactionIDs) {
	conflictingTransactions = utxo.NewTransactionIDs()

	for _, input := range transaction.Inputs() {
		u.ledger.Storage.CachedConsumers(u.ledger.options.vm.ResolveInput(input)).Consume(func(consumer *Consumer) {
			if consumer.TransactionID() == transaction.ID() {
				return
			}

			conflictingTransactions.Add(consumer.TransactionID())
		})
	}
	return
}
