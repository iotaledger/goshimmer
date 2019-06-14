package spammer

import (
	"fmt"
	"runtime"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/model/meta_transaction"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/goshimmer/plugins/tangle"
	"github.com/iotaledger/goshimmer/plugins/tipselection"
)

var PLUGIN = node.NewPlugin("Spammer", configure, run)

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

func configure(plugin *node.Plugin) {
	tpsCounter := 0
	solidCounter := 0
	var receivedCounter uint64

	gossip.Events.ReceiveTransaction.Attach(events.NewClosure(func(transaction *meta_transaction.MetaTransaction) {
		atomic.AddUint64(&receivedCounter, 1)
	}))

	go func() {
		for {
			select {
			case <-daemon.ShutdownSignal:
				return

			case <-time.After(1 * time.Second):
				fmt.Println("RECEIVED", receivedCounter, " / TPS "+strconv.Itoa(tpsCounter)+" / SOLID "+strconv.Itoa(solidCounter), " / TIPS ", tipselection.GetTipsCount())
				fmt.Println(runtime.NumGoroutine())

				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				// For info on each, see: https://golang.org/pkg/runtime/#MemStats
				fmt.Printf("Alloc = %v MiB", bToMb(m.Alloc))
				fmt.Printf("\tTotalAlloc = %v MiB", bToMb(m.TotalAlloc))
				fmt.Printf("\tSys = %v MiB", bToMb(m.Sys))
				fmt.Printf("\tNumGC = %v\n", m.NumGC)

				tpsCounter = 0
			}
		}
	}()

	tangle.Events.TransactionSolid.Attach(events.NewClosure(func(transaction *value_transaction.ValueTransaction) {
		solidCounter++
		tpsCounter++
	}))
}

func run(plugin *node.Plugin) {
	daemon.BackgroundWorker(func() {
		for {
			transactionCount := 100000

			for i := 0; i < transactionCount; i++ {
				select {
				case <-daemon.ShutdownSignal:
					return
				default:
					tx := value_transaction.New()
					tx.SetValue(int64(i + 1))
					tx.SetBranchTransactionHash(tipselection.GetRandomTip())
					tx.SetTrunkTransactionHash(tipselection.GetRandomTip())

					gossip.Events.ReceiveTransaction.Trigger(tx.MetaTransaction)
				}
			}

			//time.Sleep(5 * time.Second)
		}
	})
}
