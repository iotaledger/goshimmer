package spammer

import (
	"math/rand"
	"sync/atomic"
	"time"

	"golang.org/x/xerrors"

	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

// IssuePayloadFunc is a function which issues a payload.
type IssuePayloadFunc = func(payload payload.Payload) (*tangle.Message, error)

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

// Start starts the spammer to spam with the given messages per time unit,
// according to a inter message issuing function (IMIF)
func (spammer *Spammer) Start(rate int, timeUnit time.Duration, imif string) {
	go spammer.run(rate, timeUnit, atomic.AddInt64(&spammer.processID, 1), imif)
}

// Shutdown shuts down the spammer.
func (spammer *Spammer) Shutdown() {
	atomic.AddInt64(&spammer.processID, 1)
}

func (spammer *Spammer) run(rate int, timeUnit time.Duration, processID int64, imif string) {
	// emit messages every msgInterval interval, when IMIF is other than exponential
	msgInterval := time.Duration(timeUnit.Nanoseconds() / int64(rate))

	for {
		start := time.Now()

		if atomic.LoadInt64(&spammer.processID) != processID {
			return
		}

		// we don't care about errors or the actual issued message
		_, err := spammer.issuePayloadFunc(payload.NewGenericDataPayload([]byte("SPAM")))
		if xerrors.Is(err, tangle.ErrNotSynced) {
			// can't issue msg because node not in sync
			return
		}

		currentInterval := time.Since(start)

		if imif == "poisson" {
			// emit messages modeled with Poisson point process, whose time intervals are exponential variables with mean 1/rate
			msgInterval = time.Duration(float64(timeUnit.Nanoseconds()) * rand.ExpFloat64() / float64(rate))
		}

		if currentInterval < msgInterval {
			// there is still time, sleep until msgInterval
			time.Sleep(msgInterval - currentInterval)
		}
		// when currentInterval > msgInterval, the node can't issue msgs as fast as requested, will do as fast as it can
	}
}
