package spammer

import (
	"sync/atomic"
	"time"

	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/goshimmer/plugins/syncbeaconfollower"
	"golang.org/x/xerrors"
)

// IssuePayloadFunc is a function which issues a payload.
type IssuePayloadFunc = func(payload payload.Payload, t ...*tangle.Tangle) (*tangle.Message, error)

// Spammer spams messages with a static data payload.
type Spammer struct {
	issuePayloadFunc IssuePayloadFunc
	processID        int64
}

// New creates a new spammer.
func New(issuePayloadFunc IssuePayloadFunc) *Spammer {
	return &Spammer{
		issuePayloadFunc: issuePayloadFunc,
	}
}

// Start starts the spammer to spam with the given messages per time unit.
func (spammer *Spammer) Start(rate int, timeUnit time.Duration) {
	go spammer.run(rate, timeUnit, atomic.AddInt64(&spammer.processID, 1))
}

// Shutdown shuts down the spammer.
func (spammer *Spammer) Shutdown() {
	atomic.AddInt64(&spammer.processID, 1)
}

func (spammer *Spammer) run(rate int, timeUnit time.Duration, processID int64) {
	// emit messages every msgInterval interval
	msgInterval := time.Duration(timeUnit.Nanoseconds() / int64(rate))
	start := time.Now()

	for {
		if atomic.LoadInt64(&spammer.processID) != processID {
			return
		}

		// we don't care about errors or the actual issued message
		_, err := spammer.issuePayloadFunc(payload.NewGenericDataPayload([]byte("SPAM")))
		if xerrors.Is(err, syncbeaconfollower.ErrNodeNotSynchronized) {
			// can't issue msg because node not in sync
			return
		}

		currentInterval := time.Since(start)
		if currentInterval < msgInterval {
			//there is still time, sleep until msgInterval
			time.Sleep(msgInterval - currentInterval)
		}
		// when currentInterval > msgInterval, the node can't issue msgs as fast as requested, will do as fast as it can
		start = time.Now()
	}
}
