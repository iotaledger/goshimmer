package ratesetter

import (
	"math"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/pkg/errors"
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

const (
	// MaxLocalQueueSize is the maximum local (containing the block to be issued) queue size in bytes.
	MaxLocalQueueSize = 20
	// RateSettingIncrease is the global additive increase parameter.
	// In the Access Control for Distributed Ledgers in the Internet of Things: A Networking Approach paper this parameter is denoted as "A".
	RateSettingIncrease = 0.075
	// RateSettingDecrease global multiplicative decrease parameter (larger than 1).
	// In the Access Control for Distributed Ledgers in the Internet of Things: A Networking Approach paper this parameter is denoted as "β".
	RateSettingDecrease = 1.5
	// Wmax is the maximum inbox threshold for the node. This value denotes when maximum active mana holder backs off its rate.
	Wmax = 10
	// Wmin is the min inbox threshold for the node. This value denotes when nodes with the least mana back off their rate.
	Wmin = 5
)

var (
	// ErrInvalidIssuer is returned when an invalid block is passed to the rate setter.
	ErrInvalidIssuer = errors.New("block not issued by local node")
	// ErrStopped is returned when a block is passed to a stopped rate setter.
	ErrStopped = errors.New("rate setter stopped")
)

// region RateSetter ///////////////////////////////////////////////////////////////////////////////////////////////////

// RateSetter is a Tangle component that takes care of congestion control of local node.
type RateSetter struct {
	Events                      *Events
	protocol                    *protocol.Protocol
	self                        identity.ID
	totalAccessManaRetrieveFunc func() int64
	accessManaMapRetrieverFunc  func() map[identity.ID]int64

	issuingQueue        *IssuerQueue
	issueChan           chan *models.Block
	ownRate             *atomic.Float64
	pauseUpdates        uint
	initialPauseUpdates uint
	maxRate             float64
	shutdownSignal      chan struct{}
	shutdownOnce        sync.Once
	initOnce            sync.Once

	optsEnabled       bool
	optsInitialRate   float64
	optsPause         time.Duration
	optsSchedulerRate time.Duration
}

// New returns a new RateSetter.
func New(protocol *protocol.Protocol, accessManaMapRetrieverFunc func() map[identity.ID]int64, totalAccessManaRetrieveFunc func() int64, selfIdentity identity.ID, opts ...options.Option[RateSetter]) *RateSetter {
	return options.Apply(&RateSetter{
		Events: newEvents(),

		protocol:                    protocol,
		self:                        selfIdentity,
		accessManaMapRetrieverFunc:  accessManaMapRetrieverFunc,
		totalAccessManaRetrieveFunc: totalAccessManaRetrieveFunc,

		issuingQueue:   NewIssuerQueue(),
		issueChan:      make(chan *models.Block),
		ownRate:        atomic.NewFloat64(0),
		shutdownSignal: make(chan struct{}),
	}, opts, func(r *RateSetter) {
		r.initialPauseUpdates = uint(r.optsPause / r.optsSchedulerRate)
		r.maxRate = float64(time.Second / r.optsSchedulerRate)
	}, func(r *RateSetter) {
		if r.optsEnabled {
			go r.issuerLoop()
		}
	})
}

// Setup sets up the behavior of the component by making it attach to the relevant events of the other components.
func (r *RateSetter) setupEvents() {
	r.protocol.Events.CongestionControl.Scheduler.BlockScheduled.Attach(event.NewClosure(func(_ *scheduler.Block) {
		if r.pauseUpdates > 0 {
			r.pauseUpdates--
			return
		}
		if r.issuingQueue.Size()+r.protocol.CongestionControl.Scheduler().IssuerQueueSize(r.self) > 0 {
			r.rateSetting()
		}
	}))
}

// IssueBlock submits a block to the local issuing queue.
func (r *RateSetter) IssueBlock(block *models.Block) error {
	if !r.optsEnabled {
		r.Events.BlockIssued.Trigger(block)
		return nil
	}

	if identity.NewID(block.IssuerPublicKey()) != r.self {
		return ErrInvalidIssuer
	}
	r.initOnce.Do(func() {
		// initialize mana vectors in cache here when mana vectors are already loaded
		r.initializeInitialRate()
	})

	select {
	case r.issueChan <- block:
		return nil
	case <-r.shutdownSignal:
		return ErrStopped
	}
}

// Shutdown shuts down the RateSetter.
func (r *RateSetter) Shutdown() {
	r.shutdownOnce.Do(func() {
		close(r.shutdownSignal)
	})
}

// Rate returns the rate of the rate setter.
func (r *RateSetter) Rate() float64 {
	return r.ownRate.Load()
}

// Size returns the size of the issuing queue.
func (r *RateSetter) Size() int {
	return r.issuingQueue.Size()
}

// Estimate estimates the issuing time of new block.
func (r *RateSetter) Estimate() time.Duration {
	r.initOnce.Do(func() {
		// initialize mana vectors in cache here when mana vectors are already loaded
		r.initializeInitialRate()
	})

	var pauseUpdate time.Duration
	if r.pauseUpdates > 0 {
		pauseUpdate = time.Duration(float64(r.pauseUpdates)) * r.optsSchedulerRate
	}
	// dummy estimate
	return lo.Max(time.Duration(math.Ceil(float64(r.Size())/r.ownRate.Load()*float64(time.Second))), pauseUpdate)
}

// rateSetting updates the rate ownRate at which blocks can be issued by the node.
func (r *RateSetter) rateSetting() {
	// Return access mana or MinMana to allow zero mana nodes issue.
	ownMana := float64(lo.Max(r.accessManaMapRetrieverFunc()[r.self], scheduler.MinMana))
	maxManaValue := float64(lo.Max(append(lo.Values(r.accessManaMapRetrieverFunc()), scheduler.MinMana)...))
	totalMana := float64(r.totalAccessManaRetrieveFunc())

	ownRate := r.ownRate.Load()

	// TODO: make sure not to issue or scheduled blocks older than TSC
	if float64(r.protocol.CongestionControl.Scheduler().IssuerQueueSize(r.self)) > math.Max(Wmin, Wmax*ownMana/maxManaValue) {
		ownRate /= RateSettingDecrease
		r.pauseUpdates = r.initialPauseUpdates
	} else {
		ownRate += math.Max(RateSettingIncrease*ownMana/totalMana, 0.01)
	}
	r.ownRate.Store(math.Min(r.maxRate, ownRate))
}

func (r *RateSetter) issuerLoop() {
	var (
		issueTimer    = time.NewTimer(0) // setting this to 0 will cause a trigger right away
		timerStopped  = false
		lastIssueTime = time.Now()
	)
	defer issueTimer.Stop()

loop:
	for {
		select {
		// a new block can be submitted to the scheduler
		case <-issueTimer.C:
			timerStopped = true
			if r.issuingQueue.Front() == nil {
				continue
			}

			block := r.issuingQueue.PopFront()
			r.Events.BlockIssued.Trigger(block)
			lastIssueTime = time.Now()

			if next := r.issuingQueue.Front(); next != nil {
				issueTimer.Reset(time.Until(lastIssueTime.Add(r.issueInterval())))
				timerStopped = false
			}

		// add a new block to the local issuer queue
		case block := <-r.issueChan:
			if r.issuingQueue.Size()+1 > MaxLocalQueueSize {
				r.Events.BlockDiscarded.Trigger(block)
				continue
			}
			// add to queue
			r.issuingQueue.Enqueue(block)

			// set a new timer if needed
			// if a timer is already running it is not updated, even if the ownRate has changed
			if !timerStopped {
				break
			}
			if next := r.issuingQueue.Front(); next != nil {
				issueTimer.Reset(time.Until(lastIssueTime.Add(r.issueInterval())))
			}

		// on close, exit the loop
		case <-r.shutdownSignal:
			break loop
		}
	}

	// discard all remaining blocks at shutdown
	for r.issuingQueue.Size() > 0 {
		r.Events.BlockDiscarded.Trigger(r.issuingQueue.PopFront())
	}
}

func (r *RateSetter) issueInterval() time.Duration {
	wait := time.Duration(math.Ceil(float64(1) / r.ownRate.Load() * float64(time.Second)))
	return wait
}

func (r *RateSetter) initializeInitialRate() {
	if r.optsInitialRate > 0.0 {
		r.ownRate.Store(r.optsInitialRate)
	} else {
		ownMana := float64(lo.Max(r.accessManaMapRetrieverFunc()[r.self], scheduler.MinMana))
		totalMana := float64(r.totalAccessManaRetrieveFunc())
		r.ownRate.Store(math.Max(ownMana/totalMana*r.maxRate, 1))
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithEnabled(enabled bool) options.Option[RateSetter] {
	return func(rateSetter *RateSetter) {
		rateSetter.optsEnabled = enabled
	}
}

func WithInitialRate(initialRate float64) options.Option[RateSetter] {
	return func(rateSetter *RateSetter) {
		rateSetter.optsInitialRate = initialRate
	}
}

func WithSchedulerRate(schedulerRate time.Duration) options.Option[RateSetter] {
	return func(rateSetter *RateSetter) {
		rateSetter.optsSchedulerRate = schedulerRate
	}
}

func WithPause(pause time.Duration) options.Option[RateSetter] {
	return func(rateSetter *RateSetter) {
		rateSetter.optsPause = pause
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
