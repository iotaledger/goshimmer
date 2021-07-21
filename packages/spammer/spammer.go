package spammer

import (
	"math/rand"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/typeutils"

	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

// IssuePayloadFunc is a function which issues a payload.
type IssuePayloadFunc = func(payload payload.Payload, parentsCount ...int) (*tangle.Message, error)

// Spammer spams messages with a static data payload.
type Spammer struct {
	issuePayloadFunc IssuePayloadFunc
	log              *logger.Logger
	running          typeutils.AtomicBool
	wg               sync.WaitGroup
}

// New creates a new spammer.
func New(issuePayloadFunc IssuePayloadFunc, log *logger.Logger) *Spammer {
	return &Spammer{
		issuePayloadFunc: issuePayloadFunc,
		log:              log,
	}
}

// Start starts the spammer to spam with the given messages per time unit,
// according to a inter message issuing function (IMIF)
func (s *Spammer) Start(rate int, timeUnit time.Duration, imif string) {
	// only start if not yet running
	if s.running.SetToIf(false, true) {
		s.wg.Add(1)
		go s.run(rate, timeUnit, imif)
	}
}

// Shutdown shuts down the spammer.
func (s *Spammer) Shutdown() {
	s.running.SetTo(false)
	s.wg.Wait()
}

func (s *Spammer) run(rate int, timeUnit time.Duration, imif string) {
	defer s.wg.Done()

	// emit messages every msgInterval interval, when IMIF is other than exponential
	msgInterval := time.Duration(timeUnit.Nanoseconds() / int64(rate))
	for {
		start := time.Now()

		if !s.running.IsSet() {
			// the spammer has stopped
			return
		}

		estimatedDuration := messagelayer.Tangle().RateSetter.Estimate()
		time.Sleep(estimatedDuration)

		// we don't care about errors or the actual issued message
		_, err := s.issuePayloadFunc(payload.NewGenericDataPayload([]byte("SPAM")))
		if errors.Is(err, tangle.ErrNotSynced) {
			s.log.Info("Stopped spamming messages because node lost sync")
			s.running.SetTo(false)
			return
		}
		if err != nil {
			s.log.Warnf("could not issue spam payload: %s", err)
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
