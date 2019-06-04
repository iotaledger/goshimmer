package tangle

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/ternary"
	"github.com/iotaledger/goshimmer/packages/transaction"
	"github.com/iotaledger/goshimmer/plugins/gossip"
)

// region plugin module setup //////////////////////////////////////////////////////////////////////////////////////////

func configureMemPool(plugin *node.Plugin) {
	gossip.Events.ReceiveTransaction.Attach(events.NewClosure(func(transaction *transaction.Transaction) {
		memPoolQueue <- &Transaction{rawTransaction: transaction}
	}))
}

func runMemPool(plugin *node.Plugin) {
	plugin.LogInfo("Starting Mempool ...")

	daemon.BackgroundWorker(createMemPoolWorker(plugin))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region internal utility functions ///////////////////////////////////////////////////////////////////////////////////

func createMemPoolWorker(plugin *node.Plugin) func() {
	return func() {
		plugin.LogSuccess("Starting Mempool ... done")

		shuttingDown := false

		for !shuttingDown {
			flushTimer := time.After(MEMPOOL_FLUSH_INTERVAL)

			select {
			case <-daemon.ShutdownSignal:
				plugin.LogInfo("Stopping Mempool ...")

				shuttingDown = true

				continue

			case <-flushTimer:
				// store transactions in database

			case tx := <-memPoolQueue:
				// skip transactions that we have processed already
				if transactionStoredAlready, err := ContainsTransaction(tx.GetHash()); err != nil {
					plugin.LogFailure(err.Error())

					return
				} else if transactionStoredAlready {
					continue
				}

				// store tx in memPool
				memPoolMutex.Lock()
				memPool[tx.GetHash()] = tx
				memPoolMutex.Unlock()

				// update solidity of transactions
				_, err := UpdateSolidity(tx)
				if err != nil {
					plugin.LogFailure(err.Error())

					return
				}

				go func() {
					<-time.After(1 * time.Minute)

					err := tx.Store()
					if err != nil {
						plugin.LogFailure(err.Error())
					}

					memPoolMutex.Lock()
					delete(memPool, tx.GetHash())
					memPoolMutex.Unlock()
				}()
			}
		}

		plugin.LogSuccess("Stopping Mempool ... done")
	}
}

func getTransactionFromMemPool(transactionHash ternary.Trinary) *Transaction {
	memPoolMutex.RLock()
	defer memPoolMutex.RUnlock()

	if cacheEntry, exists := memPool[transactionHash]; exists {
		return cacheEntry
	}

	return nil
}

func memPoolContainsTransaction(transactionHash ternary.Trinary) bool {
	memPoolMutex.RLock()
	defer memPoolMutex.RUnlock()

	_, exists := memPool[transactionHash]

	return exists
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region constants and variables //////////////////////////////////////////////////////////////////////////////////////

var memPoolQueue = make(chan *Transaction, MEM_POOL_QUEUE_SIZE)

var memPool = make(map[ternary.Trinary]*Transaction)

var memPoolMutex sync.RWMutex

const (
	MEM_POOL_QUEUE_SIZE = 1000

	MEMPOOL_FLUSH_INTERVAL = 500 * time.Millisecond
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
