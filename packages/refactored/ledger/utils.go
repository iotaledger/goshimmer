package ledger

import (
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/generics/walker"

	utxo2 "github.com/iotaledger/goshimmer/packages/refactored/types/utxo"
)

type Utils struct {
	*Ledger
}

func NewUtils(ledger *Ledger) (new *Utils) {
	return &Utils{
		Ledger: ledger,
	}
}

func (u *Utils) UnprocessedConsumingTransactions(outputIDs utxo2.OutputIDs) (consumingTransactions utxo2.TransactionIDs) {
	consumingTransactions = utxo2.NewTransactionIDs()
	for it := outputIDs.Iterator(); it.HasNext(); {
		u.CachedConsumers(it.Next()).Consume(func(consumer *Consumer) {
			if consumer.Processed() {
				return
			}

			consumingTransactions.Add(consumer.TransactionID())
		})
	}

	return consumingTransactions
}

func (u *Utils) WalkConsumingTransactionID(entryPoints utxo2.OutputIDs, callback func(consumingTxID utxo2.TransactionID, walker *walker.Walker[utxo2.OutputID])) {
	if entryPoints.Size() == 0 {
		return
	}

	seenTransactions := set.New[utxo2.TransactionID](false)
	futureConeWalker := walker.New[utxo2.OutputID](false).PushAll(entryPoints.Slice()...)
	for futureConeWalker.HasNext() {
		u.CachedConsumers(futureConeWalker.Next()).Consume(func(consumer *Consumer) {
			if futureConeWalker.WalkStopped() || !seenTransactions.Add(consumer.TransactionID()) {
				return
			}

			callback(consumer.TransactionID(), futureConeWalker)
		})
	}
}

func (u *Utils) WalkConsumingTransactionMetadata(entryPoints utxo2.OutputIDs, callback func(txMetadata *TransactionMetadata, walker *walker.Walker[utxo2.OutputID])) {
	u.WalkConsumingTransactionID(entryPoints, func(consumingTxID utxo2.TransactionID, walker *walker.Walker[utxo2.OutputID]) {
		u.CachedTransactionMetadata(consumingTxID).Consume(func(txMetadata *TransactionMetadata) {
			callback(txMetadata, walker)
		})
	})
}

func (u *Utils) WalkConsumingTransactionAndMetadata(entryPoints utxo2.OutputIDs, callback func(tx *Transaction, txMetadata *TransactionMetadata, walker *walker.Walker[utxo2.OutputID])) {
	u.WalkConsumingTransactionID(entryPoints, func(consumingTxID utxo2.TransactionID, walker *walker.Walker[utxo2.OutputID]) {
		u.CachedTransactionMetadata(consumingTxID).Consume(func(txMetadata *TransactionMetadata) {
			u.CachedTransaction(consumingTxID).Consume(func(tx *Transaction) {
				callback(tx, txMetadata, walker)
			})
		})
	})
}

func (u *Utils) WithTransactionAndMetadata(txID utxo2.TransactionID, callback func(tx *Transaction, txMetadata *TransactionMetadata)) {
	u.CachedTransaction(txID).Consume(func(tx *Transaction) {
		u.CachedTransactionMetadata(txID).Consume(func(txMetadata *TransactionMetadata) {
			callback(tx, txMetadata)
		})
	})
}
