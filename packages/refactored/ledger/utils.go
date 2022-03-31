package ledger

import (
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/generics/walker"

	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

type Utils struct {
	*Ledger
}

func NewUtils(ledger *Ledger) (new *Utils) {
	return &Utils{
		Ledger: ledger,
	}
}

func (u *Utils) ConsumingTransactions(outputIDs utxo.OutputIDs, onlyProcessed bool) (consumingTransactions utxo.TransactionIDs) {
	consumingTransactions = utxo.NewTransactionIDs()
	for it := outputIDs.Iterator(); it.HasNext(); {
		u.CachedConsumers(it.Next()).Consume(func(consumer *Consumer) {
			if onlyProcessed && !consumer.Processed() {
				return
			}

			consumingTransactions.Add(consumer.TransactionID())
		})
	}

	return consumingTransactions
}

func (u *Utils) WalkConsumingTransactionID(entryPoints utxo.OutputIDs, callback func(consumingTxID utxo.TransactionID, walker *walker.Walker[utxo.OutputID])) {
	if entryPoints.Size() == 0 {
		return
	}

	seenTransactions := set.New[utxo.TransactionID](false)
	futureConeWalker := walker.New[utxo.OutputID](false).PushAll(entryPoints.Slice()...)
	for futureConeWalker.HasNext() {
		u.CachedConsumers(futureConeWalker.Next()).Consume(func(consumer *Consumer) {
			if futureConeWalker.WalkStopped() || !seenTransactions.Add(consumer.TransactionID()) {
				return
			}

			callback(consumer.TransactionID(), futureConeWalker)
		})
	}
}

func (u *Utils) WalkConsumingTransactionMetadata(entryPoints utxo.OutputIDs, callback func(txMetadata *TransactionMetadata, walker *walker.Walker[utxo.OutputID])) {
	u.WalkConsumingTransactionID(entryPoints, func(consumingTxID utxo.TransactionID, walker *walker.Walker[utxo.OutputID]) {
		u.CachedTransactionMetadata(consumingTxID).Consume(func(txMetadata *TransactionMetadata) {
			callback(txMetadata, walker)
		})
	})
}

func (u *Utils) WalkConsumingTransactionAndMetadata(entryPoints utxo.OutputIDs, callback func(tx *Transaction, txMetadata *TransactionMetadata, walker *walker.Walker[utxo.OutputID])) {
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
