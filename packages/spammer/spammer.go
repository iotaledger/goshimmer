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

	// create ticker with interval for default imif
	ticker := time.NewTicker(timeUnit / time.Duration(rate))
	defer ticker.Stop()

	done := make(chan bool)
	wg := sync.WaitGroup{}

	for {
		select {
		case <-done:
			wg.Wait()
			return
		case <-ticker.C:
			wg.Add(1)
			// adjust the ticker interval for the poisson imif
			if imif == "poisson" {
				ticker = time.NewTicker(time.Duration(float64(timeUnit.Nanoseconds()) * rand.ExpFloat64() / float64(rate)))
			}
			go func() {
				defer wg.Done()
				if !s.running.IsSet() {
					// the spammer has stopped
					done <- true
				}
				// we don't care about errors or the actual issued message
				_, err := s.issuePayloadFunc(payload.NewGenericDataPayload([]byte("SPAM")))
				if errors.Is(err, tangle.ErrNotSynced) {
					s.log.Info("Stopped spamming messages because node lost sync")
					done <- true
				}
				if err != nil {
					s.log.Warnf("could not issue spam payload: %s", err)
				}
			}()
		}
	}
}
