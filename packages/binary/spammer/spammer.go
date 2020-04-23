package spammer

import (
	"sync/atomic"
	"time"

	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/messagefactory"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
)

// Spammer spams messages with a static data payload.
type Spammer struct {
	messageFactory *messagefactory.MessageFactory

	processId      int64
	shutdownSignal chan types.Empty
}

// New creates a new spammer.
func New(messageFactory *messagefactory.MessageFactory) *Spammer {
	return &Spammer{
		messageFactory: messageFactory,

		shutdownSignal: make(chan types.Empty),
	}
}

// Start starts the spammer to spam with the given messages per second.
func (spammer *Spammer) Start(mps int) {
	go spammer.run(mps, atomic.AddInt64(&spammer.processId, 1))
}

// Shutdown shuts down the spammer.
func (spammer *Spammer) Shutdown() {
	atomic.AddInt64(&spammer.processId, 1)
}

func (spammer *Spammer) run(mps int, processId int64) {
	currentSentCounter := 0
	start := time.Now()

	for {
		if atomic.LoadInt64(&spammer.processId) != processId {
			return
		}

		spammer.messageFactory.IssuePayload(payload.NewData([]byte("SPAM")))

		currentSentCounter++

		// rate limit to the specified MPS
		if currentSentCounter >= mps {
			duration := time.Since(start)
			if duration < time.Second {
				time.Sleep(time.Second - duration)
			}

			start = time.Now()
			currentSentCounter = 0
		}
	}
}
