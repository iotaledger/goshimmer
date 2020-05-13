package utxodag

import (
	"container/list"

	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tangle"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
)

// UTXODAG represents the DAG of funds that are flowing from the genesis, to the addresses that have balance now, that
// is embedded as another layer in the message tangle.
type UTXODAG struct {
	workerPool async.WorkerPool
}

func (utxoDAG *UTXODAG) solidifyTransactionWorker(cachedTransaction *transaction.CachedTransaction, cachedTransactionMetadata *tangle.CachedTransactionMetadata, cachedAttachment *tangle.CachedAttachment) {
	// initialize the stack
	solidificationStack := list.New()
	solidificationStack.PushBack([3]interface{}{cachedTransaction, cachedTransactionMetadata, cachedAttachment})

	// process payloads that are supposed to be checked for solidity recursively
	for solidificationStack.Len() > 0 {
		// execute logic inside a func, so we can use defer to release the objects
		func() {
			// retrieve cached objects
			currentCachedTransaction, currentCachedTransactionMetadata, currentCachedAttachment := utxoDAG.popElementsFromSolidificationStack(solidificationStack)
			defer currentCachedTransaction.Release()
			defer currentCachedTransactionMetadata.Release()
			defer currentCachedAttachment.Release()

			// unwrap cached objects
			currentTransaction := currentCachedTransaction.Unwrap()
			currentTransactionMetadata := currentCachedTransactionMetadata.Unwrap()
			currentAttachment := currentCachedAttachment.Unwrap()

			// abort if any of the retrieved models is nil or payload is not solid or it was set as solid already
			if currentTransaction == nil || currentTransactionMetadata == nil || currentAttachment == nil {
				return
			}

			/*
				// abort if the transaction is not solid or invalid
				if transactionSolid, err := utxoDAG.isTransactionSolid(currentTransaction, currentTransactionMetadata); !transactionSolid || err != nil {
					if err != nil {
						// TODO: TRIGGER INVALID TX + REMOVE TXS THAT APPROVE IT
						fmt.Println(err, currentTransaction)
					}

					return
				}
			*/

			// ... and schedule check of approvers
			utxoDAG.ForEachConsumers(currentTransaction, func(cachedTransaction *transaction.CachedTransaction, transactionMetadata *tangle.CachedTransactionMetadata, cachedAttachment *tangle.CachedAttachment) {
				solidificationStack.PushBack([3]interface{}{cachedTransaction, transactionMetadata, cachedAttachment})
			})

			// book transaction
			if err := utxoDAG.bookTransaction(cachedTransaction.Retain(), cachedTransactionMetadata.Retain(), cachedAttachment.Retain()); err != nil {
				utxoDAG.Events.Error.Trigger(err)
			}
		}()
	}
}

func (utxoDAG *UTXODAG) popElementsFromSolidificationStack(stack *list.List) (*transaction.CachedTransaction, *tangle.CachedTransactionMetadata, *tangle.CachedAttachment) {
	currentSolidificationEntry := stack.Front()
	cachedTransaction := currentSolidificationEntry.Value.([3]interface{})[0].(*transaction.CachedTransaction)
	cachedTransactionMetadata := currentSolidificationEntry.Value.([3]interface{})[1].(*tangle.CachedTransactionMetadata)
	cachedAttachment := currentSolidificationEntry.Value.([3]interface{})[2].(*tangle.CachedAttachment)
	stack.Remove(currentSolidificationEntry)

	return cachedTransaction, cachedTransactionMetadata, cachedAttachment
}

// ForEachConsumers iterates through the transactions that are consuming outputs of the given transactions
func (utxoDAG *UTXODAG) ForEachConsumers(currentTransaction *transaction.Transaction, consume func(cachedTransaction *transaction.CachedTransaction, transactionMetadata *tangle.CachedTransactionMetadata, cachedAttachment *tangle.CachedAttachment)) {
	seenTransactions := make(map[transaction.ID]types.Empty)
	currentTransaction.Outputs().ForEach(func(address address.Address, balances []*balance.Balance) bool {
		utxoDAG.GetConsumers(transaction.NewOutputID(address, currentTransaction.ID())).Consume(func(consumer *tangle.Consumer) {
			if _, transactionSeen := seenTransactions[consumer.TransactionID()]; !transactionSeen {
				seenTransactions[consumer.TransactionID()] = types.Void

				cachedTransaction := utxoDAG.Transaction(consumer.TransactionID())
				cachedTransactionMetadata := utxoDAG.TransactionMetadata(consumer.TransactionID())
				for _, cachedAttachment := range utxoDAG.Attachments(consumer.TransactionID()) {
					consume(cachedTransaction, cachedTransactionMetadata, cachedAttachment)
				}
			}
		})

		return true
	})
}
