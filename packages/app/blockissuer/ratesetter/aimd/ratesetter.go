package aimd

import (
	"math"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/app/blockissuer/ratesetter/utils"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"
	"go.uber.org/atomic"
)

const (
	// RateSettingIncrease is the global additive increase parameter.
	// In the Access Control for Distributed Ledgers in the Internet of Things: A Networking Approach paper this parameter is denoted as "A".
	RateSettingIncrease = 0.075
	// RateSettingDecrease global multiplicative decrease parameter (larger than 1).
	// In the Access Control for Distributed Ledgers in the Internet of Things: A Networking Approach paper this parameter is denoted as "β".
	RateSettingDecrease = 1.5
	// WMax is the maximum inbox threshold for the node. This value denotes when maximum active mana holder backs off its rate.
	WMax = 10
	// WMin is the min inbox threshold for the node. This value denotes when nodes with the least mana back off their rate.
	WMin = 5
)

// region RateSetter ///////////////////////////////////////////////////////////////////////////////////////////////////

// RateSetter sets the issue rate of the block issuer using an AIMD-based algorithm.
type RateSetter struct {
	events                *utils.Events
	protocol              *protocol.Protocol
	self                  identity.ID
	totalManaRetrieveFunc func() int64
	manaRetrieveFunc      func() map[identity.ID]int64

	issuingQueue        *utils.IssuerQueue
	issueChan           chan *models.Block
	ownRate             *atomic.Float64
	pauseUpdates        uint
	initialPauseUpdates uint
	maxRate             float64
	shutdownSignal      chan struct{}
	shutdownOnce        sync.Once
	initOnce            sync.Once

	optsInitialRate   float64
	optsPause         time.Duration
	optsSchedulerRate time.Duration
}

// New returns a new AIMD RateSetter.
func New(protocol *protocol.Protocol, selfIdentity identity.ID, opts ...options.Option[RateSetter]) *RateSetter {

	return options.Apply(&RateSetter{
		events:                utils.NewEvents(),
		protocol:              protocol,
		self:                  selfIdentity,
		manaRetrieveFunc:      protocol.Engine().ManaTracker.ManaByIDs,
		totalManaRetrieveFunc: protocol.Engine().ManaTracker.TotalMana,
		issuingQueue:          utils.NewIssuerQueue(),
		issueChan:             make(chan *models.Block),
		ownRate:               atomic.NewFloat64(0),
		shutdownSignal:        make(chan struct{}),
	}, opts, func(r *RateSetter) {
		r.initialPauseUpdates = uint(r.optsPause / r.optsSchedulerRate)
		r.maxRate = float64(time.Second / r.optsSchedulerRate)
	}, func(r *RateSetter) {
		go r.issuerLoop()
	}, (*RateSetter).setupEvents)

}

// Events returns the events of the AIMD rate setter.
func (r *RateSetter) Events() *utils.Events {
	return r.events
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

// SubmitBlock submits a block to the local issuing queue.
func (r *RateSetter) SubmitBlock(block *models.Block) error {

	if identity.NewID(block.IssuerPublicKey()) != r.self {
		return utils.ErrInvalidIssuer
	}
	r.initOnce.Do(func() {
		// initialize mana vectors in cache here when mana vectors are already loaded
		r.initializeRate()
	})

	select {
	case r.issueChan <- block:
		return nil
	case <-r.shutdownSignal:
		return utils.ErrStopped
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
		r.initializeRate()
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
	ownMana := float64(lo.Max(r.manaRetrieveFunc()[r.self], scheduler.MinMana))
	maxManaValue := float64(lo.Max(append(lo.Values(r.manaRetrieveFunc()), scheduler.MinMana)...))
	totalMana := float64(r.totalManaRetrieveFunc())

	ownRate := r.ownRate.Load()

	// TODO: make sure not to issue or scheduled blocks older than TSC
	if float64(r.protocol.CongestionControl.Scheduler().IssuerQueueSize(r.self)) > math.Max(WMin, WMax*ownMana/maxManaValue) {
		ownRate /= RateSettingDecrease
		r.pauseUpdates = r.initialPauseUpdates
	} else {
		ownRate += math.Max(RateSettingIncrease*ownMana/totalMana, 0.01)
	}
	r.ownRate.Store(math.Min(r.maxRate, ownRate))
}

func (r *RateSetter) issuerLoop() {
	var (
		issueTimer    = time.NewTimer(r.optsSchedulerRate) // setting this to 0 will cause a trigger right away
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
			r.events.BlockIssued.Trigger(block)
			lastIssueTime = time.Now()

			if next := r.issuingQueue.Front(); next != nil {
				issueTimer.Reset(time.Until(lastIssueTime.Add(r.issueInterval())))
				timerStopped = false
			}

		// add a new block to the local issuer queue
		case block := <-r.issueChan:
			if r.issuingQueue.Size()+1 > utils.MaxLocalQueueSize {
				r.events.BlockDiscarded.Trigger(block)
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
		r.events.BlockDiscarded.Trigger(r.issuingQueue.PopFront())
	}
}

func (r *RateSetter) issueInterval() time.Duration {
	wait := time.Duration(math.Ceil(float64(1) / r.ownRate.Load() * float64(time.Second)))
	return wait
}

func (r *RateSetter) initializeRate() {
	if r.optsInitialRate > 0.0 {
		r.ownRate.Store(r.optsInitialRate)
	} else {
		ownMana := float64(lo.Max(r.manaRetrieveFunc()[r.self], scheduler.MinMana))
		totalMana := float64(r.totalManaRetrieveFunc())
		r.ownRate.Store(math.Max(ownMana/totalMana*r.maxRate, 1))
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithInitialRate(initialRate float64) options.Option[RateSetter] {
	return func(rateSetter *RateSetter) {
		rateSetter.optsInitialRate = initialRate
	}
}

func WithPause(pause time.Duration) options.Option[RateSetter] {
	return func(rateSetter *RateSetter) {
		rateSetter.optsPause = pause
	}
}

func WithSchedulerRate(rate time.Duration) options.Option[RateSetter] {
	return func(rateSetter *RateSetter) {
		rateSetter.optsSchedulerRate = rate
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
