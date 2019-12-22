package transactionspammer

import (
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/iotaledger/goshimmer/packages/gossip"
	pb "github.com/iotaledger/goshimmer/packages/gossip/proto"
	"github.com/iotaledger/goshimmer/packages/model/meta_transaction"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/tipselection"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
)

var log = logger.NewLogger("Transaction Spammer")

var spamming = false
var spammingMutex sync.Mutex

var shutdownSignal chan struct{}
var done chan struct{}

var sentCounter = uint(0)

func init() {
	shutdownSignal = make(chan struct{})
	done = make(chan struct{})
}

func Start(tps uint) {
	spammingMutex.Lock()
	spamming = true
	spammingMutex.Unlock()

	daemon.BackgroundWorker("Transaction Spammer", func() {
		start := time.Now()
		totalSentCounter := int64(0)

		for {
			select {
			case <-daemon.ShutdownSignal:
				return

			case <-shutdownSignal:
				done <- struct{}{}
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
				tx.SetTimestamp(uint(time.Now().Unix()))
				if err := tx.DoProofOfWork(meta_transaction.MIN_WEIGHT_MAGNITUDE); err != nil {
					log.Warning("PoW failed", err)
					continue
				}

				mtx := &pb.Transaction{Body: tx.MetaTransaction.GetBytes()}
				b, _ := proto.Marshal(mtx)
				gossip.Events.TransactionReceived.Trigger(&gossip.TransactionReceivedEvent{Body: b, Peer: &local.INSTANCE.Peer})

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
	})
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
