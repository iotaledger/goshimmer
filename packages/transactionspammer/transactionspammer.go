package transactionspammer

import (
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/iotaledger/goshimmer/packages/gossip"
	pb "github.com/iotaledger/goshimmer/packages/gossip/proto"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/plugins/tipselection"
	"github.com/iotaledger/hive.go/daemon"
)

var spamming = false

var startMutex sync.Mutex

var shutdownSignal chan int

var sentCounter = uint(0)

func Start(tps uint) {
	startMutex.Lock()

	if !spamming {
		shutdownSignal = make(chan int, 1)

		func(shutdownSignal chan int) {
			daemon.BackgroundWorker("Transaction Spammer", func() {
				for {
					start := time.Now()
					totalSentCounter := int64(0)

					for {
						select {
						case <-daemon.ShutdownSignal:
							return

						case <-shutdownSignal:
							return

						default:
							sentCounter++
							totalSentCounter++

							tx := value_transaction.New()
							tx.SetHead(true)
							tx.SetTail(true)
							tx.SetValue(totalSentCounter)
							tx.SetBranchTransactionHash(tipselection.GetRandomTip())
							tx.SetTrunkTransactionHash(tipselection.GetRandomTip())

							mtx := &pb.Transaction{Body: tx.MetaTransaction.GetBytes()}
							b, _ := proto.Marshal(mtx)
							gossip.Events.NewTransaction.Trigger(b)

							if sentCounter >= tps {
								duration := time.Since(start)

								if duration < time.Second {
									time.Sleep(time.Second - duration)
								}

								start = time.Now()

								sentCounter = 0
							}
						}
					}
				}
			})
		}(shutdownSignal)

		spamming = true
	}

	startMutex.Unlock()
}

func Stop() {
	startMutex.Lock()

	if spamming {
		close(shutdownSignal)

		spamming = false
	}

	startMutex.Unlock()
}
