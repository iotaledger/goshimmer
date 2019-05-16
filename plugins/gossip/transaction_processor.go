package gossip

import (
    "github.com/iotaledger/goshimmer/packages/filter"
    "github.com/iotaledger/goshimmer/packages/transaction"
)

var transactionFilter = filter.NewByteArrayFilter(TRANSACTION_FILTER_SIZE)

func processTransactionData(transactionData []byte) {
    if transactionFilter.Add(transactionData) {
        Events.ReceiveTransaction.Trigger(transaction.FromBytes(transactionData))
    }
}

const (
    TRANSACTION_FILTER_SIZE = 5000
)
