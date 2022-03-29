package ledger

import (
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/generics/walker"
)

type Utils struct {
	*Ledger
}

func NewUtils(ledger *Ledger) (new *Utils) {
	return &Utils{
		Ledger: ledger,
	}
}

func (u *Utils) WalkConsumingTransactionID(entryPoints OutputIDs, callback func(consumingTxID TransactionID, walker *walker.Walker[OutputID])) {
	if entryPoints.Size() == 0 {
		return
	}

	seenTransactions := set.New[TransactionID](false)
	futureConeWalker := walker.New[OutputID](false).PushAll(entryPoints.Slice()...)
	for futureConeWalker.HasNext() {
		u.CachedConsumers(futureConeWalker.Next()).Consume(func(consumer *Consumer) {
			if futureConeWalker.WalkStopped() || !seenTransactions.Add(consumer.TransactionID()) {
				return
			}

			callback(consumer.TransactionID(), futureConeWalker)
		})
	}
}

func (u *Utils) WalkConsumingTransactionMetadata(entryPoints OutputIDs, callback func(txMetadata *TransactionMetadata, walker *walker.Walker[OutputID])) {
	u.WalkConsumingTransactionID(entryPoints, func(consumingTxID TransactionID, walker *walker.Walker[OutputID]) {
		u.CachedTransactionMetadata(consumingTxID).Consume(func(txMetadata *TransactionMetadata) {
			callback(txMetadata, walker)
		})
	})
}

func (u *Utils) WalkConsumingTransactionAndMetadata(entryPoints OutputIDs, callback func(tx *Transaction, txMetadata *TransactionMetadata, walker *walker.Walker[OutputID])) {
	u.WalkConsumingTransactionID(entryPoints, func(consumingTxID TransactionID, walker *walker.Walker[OutputID]) {
		u.CachedTransactionMetadata(consumingTxID).Consume(func(txMetadata *TransactionMetadata) {
			u.CachedTransaction(consumingTxID).Consume(func(tx *Transaction) {
				callback(tx, txMetadata, walker)
			})
		})
	})
}

func (u *Utils) WithTransactionAndMetadata(txID TransactionID, callback func(tx *Transaction, txMetadata *TransactionMetadata)) {
	u.CachedTransaction(txID).Consume(func(tx *Transaction) {
		u.CachedTransactionMetadata(txID).Consume(func(txMetadata *TransactionMetadata) {
			callback(tx, txMetadata)
		})
	})
}
