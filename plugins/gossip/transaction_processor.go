package gossip

import (
    "github.com/iotaledger/goshimmer/packages/daemon"
    "github.com/iotaledger/goshimmer/packages/events"
    "github.com/iotaledger/goshimmer/packages/filter"
    "github.com/iotaledger/goshimmer/packages/node"
    "github.com/iotaledger/goshimmer/packages/transaction"
)

var transactionFilter = filter.NewByteArrayFilter(TRANSACTION_FILTER_SIZE)

var sendQueue = make(chan *transaction.Transaction, TRANSACTION_SEND_QUEUE_SIZE)

func SendTransaction(transaction *transaction.Transaction) {
    sendQueue <- transaction
}

func SendTransactionToNeighbor(neighbor *Peer, transaction *transaction.Transaction) {
    switch neighbor.AcceptedProtocol.Version {
    case VERSION_1:
        sendTransactionV1(neighbor.AcceptedProtocol, transaction)
    }
}

func processIncomingTransactionData(transactionData []byte) {
    if transactionFilter.Add(transactionData) {
        Events.ReceiveTransaction.Trigger(transaction.FromBytes(transactionData))
    }
}

func configureTransactionProcessor(plugin *node.Plugin) {
    daemon.Events.Shutdown.Attach(events.NewClosure(func() {
        plugin.LogInfo("Stopping Transaction Processor ...")
    }))
}

func runTransactionProcessor(plugin *node.Plugin) {
    plugin.LogInfo("Starting Transaction Processor ...")

    daemon.BackgroundWorker(func() {
        plugin.LogSuccess("Starting Transaction Processor ... done")

        for {
            select {
            case <- daemon.ShutdownSignal:
                plugin.LogSuccess("Stopping Transaction Processor ... done")

                return

            case tx := <-sendQueue:
                for _, neighbor := range GetNeighbors() {
                    SendTransactionToNeighbor(neighbor, tx)
                }
            }
        }
    })
}

const (
    TRANSACTION_FILTER_SIZE = 5000

    TRANSACTION_SEND_QUEUE_SIZE = 500
)
