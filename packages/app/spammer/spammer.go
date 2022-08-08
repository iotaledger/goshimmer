package spammer

import (
	"math/rand"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/logger"
	"github.com/iotaledger/hive.go/core/typeutils"
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/core/tangleold"
	"github.com/iotaledger/goshimmer/packages/core/tangleold/payload"
)

const (
	// limit the number of max allowed go routines created during spam.
	maxGoroutines = 2
)

// IssuePayloadFunc is a function which issues a payload.
type IssuePayloadFunc = func(payload payload.Payload, parentsCount ...int) (*tangleold.Block, error)

// EstimateFunc returns the time estimate required for the block to be issued by the rate setter.
type EstimateFunc = func() time.Duration

// Spammer spams blocks with a static data payload.
type Spammer struct {
	issuePayloadFunc IssuePayloadFunc
	estimateFunc     EstimateFunc
	log              *logger.Logger
	running          typeutils.AtomicBool
	shutdown         chan struct{}
	wg               sync.WaitGroup
	goroutinesCount  *atomic.Int32
}

// New creates a new spammer.
func New(issuePayloadFunc IssuePayloadFunc, log *logger.Logger, estimateFunc EstimateFunc) *Spammer {
	return &Spammer{
		issuePayloadFunc: issuePayloadFunc,
		estimateFunc:     estimateFunc,
		shutdown:         make(chan struct{}),
		log:              log,
	}
}

// Start starts the spammer to spam with the given blocks per time unit,
// according to a inter block issuing function (IMIF)
func (s *Spammer) Start(rate int, timeUnit time.Duration, imif string) {
	// only start if not yet running
	if s.running.SetToIf(false, true) {
		s.wg.Add(1)
		go s.run(rate, timeUnit, imif)
	}
}

// Shutdown shuts down the spammer.
func (s *Spammer) Shutdown() {
	s.signalShutdown()
	s.wg.Wait()
}

func (s *Spammer) signalShutdown() {
	if s.running.SetToIf(true, false) {
		s.shutdown <- struct{}{}
	}
}

func (s *Spammer) run(rate int, timeUnit time.Duration, imif string) {
	defer s.wg.Done()
	// create ticker with interval for default imif
	ticker := time.NewTicker(timeUnit / time.Duration(rate))
	defer ticker.Stop()
	s.goroutinesCount = atomic.NewInt32(0)
	for {
		select {
		case <-s.shutdown:
			return
		case <-ticker.C:
			estimatedDuration := s.estimateFunc()
			// TODO: only sleep if estimate > some threshold.
			time.Sleep(estimatedDuration)

			// adjust the ticker interval for the poisson imif
			if imif == "poisson" {
				ticker = time.NewTicker(time.Duration(float64(timeUnit.Nanoseconds()) * rand.ExpFloat64() / float64(rate)))
			}
			// start only if at most maxGoroutines not finished their work
			if s.goroutinesCount.Load() >= maxGoroutines {
				break
			}
			go func() {
				s.goroutinesCount.Add(1)
				defer s.goroutinesCount.Add(-1)
				// we don't care about errors or the actual issued block
				_, err := s.issuePayloadFunc(payload.NewGenericDataPayload([]byte("SPAM")))
				if errors.Is(err, tangleold.ErrNotBootstrapped) {
					s.log.Info("Stopped spamming blocks because node lost sync")
					s.signalShutdown()
					return
				}
				if err != nil {
					s.log.Warnf("could not issue spam payload: %s", err)
				}
			}()
		}
	}
}
