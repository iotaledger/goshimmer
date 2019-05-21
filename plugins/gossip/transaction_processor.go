package gossip

import (
    "github.com/iotaledger/goshimmer/packages/filter"
    "github.com/iotaledger/goshimmer/packages/node"
    "github.com/iotaledger/goshimmer/packages/transaction"
)

var transactionFilter = filter.NewByteArrayFilter(TRANSACTION_FILTER_SIZE)

func processIncomingTransactionData(transactionData []byte) {
    if transactionFilter.Add(transactionData) {
        Events.ReceiveTransaction.Trigger(transaction.FromBytes(transactionData))
    }
}



func configureTransactionProcessor(plugin *node.Plugin) {
}

func runTransactionProcessor(plugin *node.Plugin) {
}

const (
    TRANSACTION_FILTER_SIZE = 5000
)
