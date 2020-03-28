package spammer

import (
	"sync/atomic"
	"time"

	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/messagefactory"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
)

type Spammer struct {
	messageFactory *messagefactory.MessageFactory

	processId      int64
	shutdownSignal chan types.Empty
}

func New(messageFactory *messagefactory.MessageFactory) *Spammer {
	return &Spammer{
		messageFactory: messageFactory,

		shutdownSignal: make(chan types.Empty),
	}
}

func (spammer *Spammer) Start(tps int) {
	go spammer.run(tps, atomic.AddInt64(&spammer.processId, 1))
}

func (spammer *Spammer) Shutdown() {
	atomic.AddInt64(&spammer.processId, 1)
}

func (spammer *Spammer) run(tps int, processId int64) {
	currentSentCounter := 0
	start := time.Now()

	for {
		if atomic.LoadInt64(&spammer.processId) != processId {
			return
		}

		spammer.messageFactory.IssuePayload(payload.NewData([]byte("SPAM")))

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
