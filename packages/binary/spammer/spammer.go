package spammer

import (
	"sync/atomic"
	"time"

	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
)

// IssuePayloadFunc is a function which issues a payload.
type IssuePayloadFunc = func(payload payload.Payload) (*message.Message, error)

// Spammer spams messages with a static data payload.
type Spammer struct {
	issuePayloadFunc IssuePayloadFunc

	processId      int64
	shutdownSignal chan types.Empty
}

// New creates a new spammer.
func New(issuePayloadFunc IssuePayloadFunc) *Spammer {
	return &Spammer{
		issuePayloadFunc: issuePayloadFunc,
		shutdownSignal:   make(chan types.Empty),
	}
}

// Start starts the spammer to spam with the given messages per time unit.
func (spammer *Spammer) Start(rate int, timeUnit time.Duration) {
	go spammer.run(rate, timeUnit, atomic.AddInt64(&spammer.processId, 1))
}

// Shutdown shuts down the spammer.
func (spammer *Spammer) Shutdown() {
	atomic.AddInt64(&spammer.processId, 1)
}

func (spammer *Spammer) run(rate int, timeUnit time.Duration, processID int64) {
	currentSentCounter := 0
	start := time.Now()

	for {
		if atomic.LoadInt64(&spammer.processId) != processID {
			return
		}

		// we don't care about errors or the actual issued message
		_, _ = spammer.issuePayloadFunc(payload.NewData([]byte("SPAM")))

		currentSentCounter++

		// rate limit to the specified MPS
		if currentSentCounter >= rate {
			duration := time.Since(start)
			if duration < timeUnit {
				time.Sleep(timeUnit - duration)
			}

			start = time.Now()
			currentSentCounter = 0
		}
	}
}
