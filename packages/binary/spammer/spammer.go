package spammer

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/binary/identity"
	"github.com/iotaledger/goshimmer/packages/binary/tangle"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction/payload/data"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/tipselector"
)

type Spammer struct {
	tangle      *tangle.Tangle
	tipSelector *tipselector.TipSelector

	running        bool
	startStopMutex sync.Mutex
	shutdownSignal chan types.Empty
}

func New(tangle *tangle.Tangle, tipSelector *tipselector.TipSelector) *Spammer {
	return &Spammer{
		tangle:      tangle,
		tipSelector: tipSelector,
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

func (spammer *Spammer) Shutdown() {
	spammer.startStopMutex.Lock()
	defer spammer.startStopMutex.Unlock()

	if !spammer.running {
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
			spammer.tangle.AttachTransaction(
				transaction.New(trunkTransactionId, branchTransactionId, identity.Generate(), data.New([]byte("SPAM"))),
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
