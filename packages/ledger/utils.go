package ledger

import (
	"github.com/iotaledger/hive.go/generics/lo"
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/generics/walker"

	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
)

type utils struct {
	*Ledger
}

func newUtils(ledger *Ledger) (new *utils) {
	return &utils{
		Ledger: ledger,
	}
}

func (u *utils) resolveInputs(inputs []utxo.Input) (outputIDs utxo.OutputIDs) {
	return utxo.NewOutputIDs(lo.Map(inputs, u.Options.VM.ResolveInput)...)
}

func (u *utils) UnprocessedConsumingTransactions(outputIDs utxo.OutputIDs) (consumingTransactions utxo.TransactionIDs) {
	consumingTransactions = utxo.NewTransactionIDs()
	for it := outputIDs.Iterator(); it.HasNext(); {
		u.Storage.CachedConsumers(it.Next()).Consume(func(consumer *Consumer) {
			if consumer.Processed() {
				return
			}

			consumingTransactions.Add(consumer.TransactionID())
		})
	}

	return consumingTransactions
}

func (u *utils) WalkConsumingTransactionID(entryPoints utxo.OutputIDs, callback func(consumingTxID utxo.TransactionID, walker *walker.Walker[utxo.OutputID])) {
	if entryPoints.Size() == 0 {
		return
	}

	seenTransactions := set.New[utxo.TransactionID](false)
	futureConeWalker := walker.New[utxo.OutputID](false).PushAll(entryPoints.Slice()...)
	for futureConeWalker.HasNext() {
		u.Storage.CachedConsumers(futureConeWalker.Next()).Consume(func(consumer *Consumer) {
			if futureConeWalker.WalkStopped() || !seenTransactions.Add(consumer.TransactionID()) {
				return
			}

			callback(consumer.TransactionID(), futureConeWalker)
		})
	}
}

func (u *utils) WalkConsumingTransactionMetadata(entryPoints utxo.OutputIDs, callback func(txMetadata *TransactionMetadata, walker *walker.Walker[utxo.OutputID])) {
	u.WalkConsumingTransactionID(entryPoints, func(consumingTxID utxo.TransactionID, walker *walker.Walker[utxo.OutputID]) {
		u.Storage.CachedTransactionMetadata(consumingTxID).Consume(func(txMetadata *TransactionMetadata) {
			callback(txMetadata, walker)
		})
	})
}

func (u *utils) WalkConsumingTransactionAndMetadata(entryPoints utxo.OutputIDs, callback func(tx *Transaction, txMetadata *TransactionMetadata, walker *walker.Walker[utxo.OutputID])) {
	u.WalkConsumingTransactionID(entryPoints, func(consumingTxID utxo.TransactionID, walker *walker.Walker[utxo.OutputID]) {
		u.Storage.CachedTransactionMetadata(consumingTxID).Consume(func(txMetadata *TransactionMetadata) {
			u.Storage.CachedTransaction(consumingTxID).Consume(func(tx *Transaction) {
				callback(tx, txMetadata, walker)
			})
		})
	})
}

func (u *utils) WithTransactionAndMetadata(txID utxo.TransactionID, callback func(tx *Transaction, txMetadata *TransactionMetadata)) {
	u.Storage.CachedTransaction(txID).Consume(func(tx *Transaction) {
		u.Storage.CachedTransactionMetadata(txID).Consume(func(txMetadata *TransactionMetadata) {
			callback(tx, txMetadata)
		})
	})
}
