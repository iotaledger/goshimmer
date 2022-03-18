package ledger

import (
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/generics/walker"

	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

type Utils struct {
	*Ledger
}

func (u *Utils) WalkConsumingTransactionID(callback func(consumingTxID utxo.TransactionID, walker *walker.Walker[utxo.OutputID]), entryPoints []utxo.OutputID) {
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

func (u *Utils) WalkConsumingTransactionMetadata(callback func(txMetadata *TransactionMetadata, walker *walker.Walker[utxo.OutputID]), entryPoints []utxo.OutputID) {
	u.WalkConsumingTransactionID(func(consumingTxID utxo.TransactionID, walker *walker.Walker[utxo.OutputID]) {
		u.CachedTransactionMetadata(consumingTxID).Consume(func(txMetadata *TransactionMetadata) {
			callback(txMetadata, walker)
		})
	}, entryPoints)
}
