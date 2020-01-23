package transactionspammer

import (
	"strings"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/model/meta_transaction"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/tipselection"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
)

const logEveryNTransactions = 5000

var log *logger.Logger

var spamming = false
var spammingMutex sync.Mutex

var shutdownSignal chan struct{}
var done chan struct{}

func init() {
	shutdownSignal = make(chan struct{})
	done = make(chan struct{})
}

var targetAddress = strings.Repeat("SPAMMMMER", 9)

func Start(tps uint64) {
	log = logger.NewLogger("Transaction Spammer")
	spammingMutex.Lock()
	spamming = true
	spammingMutex.Unlock()

	daemon.BackgroundWorker("Transaction Spammer", func(daemonShutdownSignal <-chan struct{}) {
		start := time.Now()

		var totalSentCounter, currentSentCounter uint64

		log.Infof("started spammer...will output sent count every %d transactions", logEveryNTransactions)
		defer log.Infof("spammer stopped, spammed %d transactions", totalSentCounter)
		for {
			select {
			case <-daemonShutdownSignal:
				return

			case <-shutdownSignal:
				done <- struct{}{}
				return

			default:
				currentSentCounter++
				totalSentCounter++

				tx := value_transaction.New()
				tx.SetHead(true)
				tx.SetTail(true)
				tx.SetAddress(targetAddress)
				tx.SetBranchTransactionHash(tipselection.GetRandomTip())
				tx.SetTrunkTransactionHash(tipselection.GetRandomTip())
				tx.SetTimestamp(uint(time.Now().Unix()))
				if err := tx.DoProofOfWork(meta_transaction.MIN_WEIGHT_MAGNITUDE); err != nil {
					log.Warn("PoW failed", err)
					continue
				}

				gossip.Events.TransactionReceived.Trigger(&gossip.TransactionReceivedEvent{Data: tx.GetBytes(), Peer: &local.GetInstance().Peer})

				if totalSentCounter%logEveryNTransactions == 0 {
					log.Infof("spammed %d transactions", totalSentCounter)
				}

				// rate limit to the specified TPS
				if currentSentCounter >= tps {
					duration := time.Since(start)

					if duration < time.Second {
						time.Sleep(time.Second - duration)
					}

					start = time.Now()
					currentSentCounter = 0
				}
			}
		}
	}, shutdown.ShutdownPriorityTangleSpammer)
}

func Stop() {
	spammingMutex.Lock()
	if spamming {
		shutdownSignal <- struct{}{}
		// wait for spammer to be done
		<-done
		spamming = false
	}
	spammingMutex.Unlock()
}
