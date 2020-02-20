package spammer

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/binary/identity"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction/payload/data"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/tipselector"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/transactionparser"
)

type Spammer struct {
	transactionParser *transactionparser.TransactionParser
	tipSelector       *tipselector.TipSelector

	running        bool
	startStopMutex sync.Mutex
	shutdownSignal chan types.Empty
}

func New(transactionParser *transactionparser.TransactionParser, tipSelector *tipselector.TipSelector) *Spammer {
	return &Spammer{
		shutdownSignal:    make(chan types.Empty),
		transactionParser: transactionParser,
		tipSelector:       tipSelector,
	}
}

func (spammer *Spammer) Start(tps int) {
	spammer.startStopMutex.Lock()
	defer spammer.startStopMutex.Unlock()

	if !spammer.running {
		spammer.running = true

		go spammer.run(tps)
	}
}

func (spammer *Spammer) Burst(transactions int) {
	spammer.startStopMutex.Lock()
	defer spammer.startStopMutex.Unlock()

	if !spammer.running {
		spammer.running = true

		go spammer.sendBurst(transactions)
	}
}

func (spammer *Spammer) Shutdown() {
	spammer.startStopMutex.Lock()
	defer spammer.startStopMutex.Unlock()

	if spammer.running {
		spammer.running = false

		spammer.shutdownSignal <- types.Void
	}
}

func (spammer *Spammer) run(tps int) {
	currentSentCounter := 0
	start := time.Now()

	for {

		select {
		case <-spammer.shutdownSignal:
			return

		default:
			trunkTransactionId, branchTransactionId := spammer.tipSelector.GetTips()
			spammer.transactionParser.Parse(
				transaction.New(trunkTransactionId, branchTransactionId, identity.Generate(), data.New([]byte("SPAM"))).GetBytes(),
			)

			currentSentCounter++

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
}

func (spammer *Spammer) sendBurst(transactions int) {
	spammingIdentity := identity.Generate()

	previousTransactionId := transaction.EmptyId

	burstBuffer := make([][]byte, transactions)
	for i := 0; i < transactions; i++ {
		select {
		case <-spammer.shutdownSignal:
			return

		default:
			spamTransaction := transaction.New(previousTransactionId, previousTransactionId, spammingIdentity, data.New([]byte("SPAM")))
			previousTransactionId = spamTransaction.GetId()
			burstBuffer[i] = spamTransaction.GetBytes()
		}
	}

	for i := 0; i < transactions; i++ {
		select {
		case <-spammer.shutdownSignal:
			return

		default:
			spammer.transactionParser.Parse(burstBuffer[i])
		}
	}
}
