package ledger

import (
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/generics/walker"

	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

type Utils struct {
	*Ledger
}

func (u *Utils) WalkConsumingTransactionID(entryPoints []utxo.OutputID, callback func(consumingTxID utxo.TransactionID, walker *walker.Walker[utxo.OutputID])) {
	if len(entryPoints) == 0 {
		return
	}

	seenTransactions := set.New[utxo.TransactionID](false)
	futureConeWalker := walker.New[utxo.OutputID](false).PushAll(entryPoints...)
	for futureConeWalker.HasNext() {
		u.CachedConsumers(futureConeWalker.Next()).Consume(func(consumer *Consumer) {
			if futureConeWalker.WalkStopped() || !seenTransactions.Add(consumer.TransactionID()) {
				return
			}

			callback(consumer.TransactionID(), futureConeWalker)
		})
	}
}

func (u *Utils) WalkConsumingTransactionMetadata(entryPoints []utxo.OutputID, callback func(txMetadata *TransactionMetadata, walker *walker.Walker[utxo.OutputID])) {
	u.WalkConsumingTransactionID(entryPoints, func(consumingTxID utxo.TransactionID, walker *walker.Walker[utxo.OutputID]) {
		u.CachedTransactionMetadata(consumingTxID).Consume(func(txMetadata *TransactionMetadata) {
			callback(txMetadata, walker)
		})
	})
}

func (u *Utils) WalkConsumingTransactionAndMetadata(entryPoints []utxo.OutputID, callback func(tx *Transaction, txMetadata *TransactionMetadata, walker *walker.Walker[utxo.OutputID])) {
	u.WalkConsumingTransactionID(entryPoints, func(consumingTxID utxo.TransactionID, walker *walker.Walker[utxo.OutputID]) {
		u.CachedTransactionMetadata(consumingTxID).Consume(func(txMetadata *TransactionMetadata) {
			u.CachedTransaction(consumingTxID).Consume(func(tx *Transaction) {
				callback(tx, txMetadata, walker)
			})
		})
	})
}

func (u *Utils) WithTransactionAndMetadata(txID utxo.TransactionID, callback func(tx *Transaction, txMetadata *TransactionMetadata)) {
	u.CachedTransaction(txID).Consume(func(tx *Transaction) {
		u.CachedTransactionMetadata(txID).Consume(func(txMetadata *TransactionMetadata) {
			callback(tx, txMetadata)
		})
	})
}
