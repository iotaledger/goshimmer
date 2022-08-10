package tangleold

import (
	"math"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/core/tangleold/schedulerutils"
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

// region RateSetterParams /////////////////////////////////////////////////////////////////////////////////////////////

// RateSetterParams represents the parameters for RateSetter.
type RateSetterParams struct {
	Enabled          bool
	Initial          float64
	RateSettingPause time.Duration
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region RateSetter ///////////////////////////////////////////////////////////////////////////////////////////////////

// RateSetter is a Tangle component that takes care of congestion control of local node.
type RateSetter struct {
	tangle              *Tangle
	Events              *RateSetterEvents
	self                identity.ID
	issuingQueue        *schedulerutils.NodeQueue
	issueChan           chan *Block
	ownRate             *atomic.Float64
	pauseUpdates        uint
	initialPauseUpdates uint
	maxRate             float64
	shutdownSignal      chan struct{}
	shutdownOnce        sync.Once
	initOnce            sync.Once
}

// NewRateSetter returns a new RateSetter.
func NewRateSetter(tangle *Tangle) *RateSetter {
	rateSetter := &RateSetter{
		tangle:              tangle,
		Events:              newRateSetterEvents(),
		self:                tangle.Options.Identity.ID(),
		issuingQueue:        schedulerutils.NewNodeQueue(tangle.Options.Identity.ID()),
		issueChan:           make(chan *Block),
		ownRate:             atomic.NewFloat64(0),
		pauseUpdates:        0,
		initialPauseUpdates: uint(tangle.Options.RateSetterParams.RateSettingPause / tangle.Scheduler.Rate()),
		maxRate:             float64(time.Second / tangle.Scheduler.Rate()),
		shutdownSignal:      make(chan struct{}),
		shutdownOnce:        sync.Once{},
	}

	go rateSetter.issuerLoop()
	return rateSetter
}

// Setup sets up the behavior of the component by making it attach to the relevant events of the other components.
func (r *RateSetter) Setup() {
	if !r.tangle.Options.RateSetterParams.Enabled {
		r.tangle.BlockFactory.Events.BlockConstructed.Attach(event.NewClosure(func(event *BlockConstructedEvent) {
			r.Events.BlockIssued.Trigger(event)
		}))
		return
	}

	r.tangle.BlockFactory.Events.BlockConstructed.Attach(event.NewClosure(func(event *BlockConstructedEvent) {
		if err := r.Issue(event.Block); err != nil {
			r.Events.Error.Trigger(errors.Errorf("failed to submit to rate setter: %w", err))
		}
	}))
	// update own rate setting
	r.tangle.Scheduler.Events.BlockScheduled.Attach(event.NewClosure(func(_ *BlockScheduledEvent) {
		if r.pauseUpdates > 0 {
			r.pauseUpdates--
			return
		}
		if r.issuingQueue.Size()+r.tangle.Scheduler.NodeQueueSize(selfLocalIdentity.ID()) > 0 {
			r.rateSetting()
		}
	}))
}

// Issue submits a block to the local issuing queue.
func (r *RateSetter) Issue(block *Block) error {
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
		pauseUpdate = time.Duration(float64(r.pauseUpdates)) * r.tangle.Scheduler.Rate()
	}
	// dummy estimate
	return lo.Max(time.Duration(math.Ceil(float64(r.Size())/r.ownRate.Load()*float64(time.Second))), pauseUpdate)
}

// rateSetting updates the rate ownRate at which blocks can be issued by the node.
func (r *RateSetter) rateSetting() {
	// Return access mana or MinMana to allow zero mana nodes issue.
	ownMana := math.Max(r.tangle.Options.SchedulerParams.AccessManaMapRetrieverFunc()[r.self], MinMana)
	maxManaValue := lo.Max(append(lo.Values(r.tangle.Options.SchedulerParams.AccessManaMapRetrieverFunc()), MinMana)...)
	totalMana := r.tangle.Options.SchedulerParams.TotalAccessManaRetrieveFunc()

	ownRate := r.ownRate.Load()

	// TODO: make sure not to issue or scheduled blocks older than TSC
	if float64(r.tangle.Scheduler.NodeQueueSize(r.self)) > math.Max(Wmin, Wmax*ownMana/maxManaValue) {
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

			blk := r.issuingQueue.PopFront().(*Block)
			r.Events.BlockIssued.Trigger(&BlockConstructedEvent{Block: blk})
			lastIssueTime = time.Now()

			if next := r.issuingQueue.Front(); next != nil {
				issueTimer.Reset(time.Until(lastIssueTime.Add(r.issueInterval())))
				timerStopped = false
			}

		// add a new block to the local issuer queue
		case blk := <-r.issueChan:
			if r.issuingQueue.Size()+1 > MaxLocalQueueSize {
				r.Events.BlockDiscarded.Trigger(&BlockDiscardedEvent{blk.ID()})
				continue
			}
			// add to queue
			r.issuingQueue.Submit(blk)
			r.issuingQueue.Ready(blk)

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
	for _, id := range r.issuingQueue.IDs() {
		r.Events.BlockDiscarded.Trigger(&BlockDiscardedEvent{blockIDFromElementID(id)})
	}
}

func (r *RateSetter) issueInterval() time.Duration {
	wait := time.Duration(math.Ceil(float64(1) / r.ownRate.Load() * float64(time.Second)))
	return wait
}

func (r *RateSetter) initializeInitialRate() {
	if r.tangle.Options.RateSetterParams.Initial > 0.0 {
		r.ownRate.Store(r.tangle.Options.RateSetterParams.Initial)
	} else {
		ownMana := math.Max(r.tangle.Options.SchedulerParams.AccessManaMapRetrieverFunc()[r.tangle.Options.Identity.ID()], MinMana)
		totalMana := r.tangle.Options.SchedulerParams.TotalAccessManaRetrieveFunc()
		r.ownRate.Store(math.Max(ownMana/totalMana*r.maxRate, 1))
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region RateSetterEvents /////////////////////////////////////////////////////////////////////////////////////////////

// RateSetterEvents represents events happening in the rate setter.
type RateSetterEvents struct {
	BlockDiscarded *event.Event[*BlockDiscardedEvent]
	BlockIssued    *event.Event[*BlockConstructedEvent]
	Error          *event.Event[error]
}

func newRateSetterEvents() (new *RateSetterEvents) {
	return &RateSetterEvents{
		BlockDiscarded: event.New[*BlockDiscardedEvent](),
		BlockIssued:    event.New[*BlockConstructedEvent](),
		Error:          event.New[error](),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
