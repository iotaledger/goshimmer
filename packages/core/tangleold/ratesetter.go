package tangleold

import (
	"math"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/lo"
	"github.com/iotaledger/hive.go/identity"
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/core/tangleold/schedulerutils"
)

const (
	// MaxLocalQueueSize is the maximum local (containing the block to be issued) queue size in bytes.
	MaxLocalQueueSize = 20
	// RateSettingIncrease is the global additive increase parameter.
	// In the Access Control for Distributed Ledgers in the Internet of Things: A Networking Approach paper this parameter is denoted as "A".
	RateSettingIncrease = 0.075
	// RateSettingDecrease global multiplicative decrease parameter (value between 0 and 1).
	// In the Access Control for Distributed Ledgers in the Internet of Things: A Networking Approach paper this parameter is denoted as "β".
	RateSettingDecrease = 0.7
	// Wmax is the maximum inbox threshold for the node. This value denotes when maximum active mana holder backs off its rate.
	Wmax = 10
	// Wmin is the min inbox threshold for the node. This value denotes when nodes with the least mana back off their rate.
	Wmin = 5
	// The rate setter can be in one of three modes: aimd, deficit or disabled.
	aimdMode     = "aimd"
	deficitMode  = "deficit"
	disabledMode = "disabled"
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
	Mode             string
	Initial          float64
	RateSettingPause time.Duration
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region RateSetter interface ///////////////////////////////////////////////////////////////////////////////////////////////////

type RateSetter interface {
	// setup and shutdown
	Setup()
	Shutdown()
	// web API
	Rate() float64
	Estimate() time.Duration
	Size() int
	// getter for events
	RateSetterEvents() *RateSetterEvents
}

// NewRateSetter returns a new RateSetter.
func NewRateSetter(tangle *Tangle) RateSetter {
	if tangle.Options.RateSetterParams.Mode == aimdMode {
		return NewAIMDRateSetter(tangle)
	} else if tangle.Options.RateSetterParams.Mode == disabledMode {
		return NewDisabledRateSetter(tangle)
	}
	// return a deficit-based ratesetter by default if any invalid mode is specified.
	return NewDeficitRateSetter(tangle)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////

// region AIMDRateSetter /////////////////////////////////////////////////////////////////////////////////////////

// AIMD rate setter adjusts local issuing rate by increasing rate additively
// and then multiplicatively decreasing when congestion is detected.
type AIMDRateSetter struct {
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

func NewAIMDRateSetter(tangle *Tangle) *AIMDRateSetter {
	return &AIMDRateSetter{
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
}

func (r *AIMDRateSetter) RateSetterEvents() *RateSetterEvents {
	return r.Events
}

// Setup sets up the behavior of the component by making it attach to the relevant events of the other components.
func (r *AIMDRateSetter) Setup() {
	r.tangle.BlockFactory.Events.BlockConstructed.Attach(event.NewClosure(func(event *BlockConstructedEvent) {
		if err := r.Issue(event.Block); err != nil {
			r.Events.Error.Trigger(errors.Errorf("failed to submit to rate setter: %w", err))
		}
	}))
	// update own AIMD rate setting each time a block is scheduled
	r.tangle.Scheduler.Events.BlockScheduled.Attach(event.NewClosure(func(_ *BlockScheduledEvent) {
		if r.pauseUpdates > 0 {
			r.pauseUpdates--
			return
		}
		if r.issuingQueue.Size()+r.tangle.Scheduler.NodeQueueSize(selfLocalIdentity.ID()) > 0 {
			r.AIMDUpdate()
		}
	}))
	go r.IssuerLoop()
}

// Shutdown shuts down the RateSetter.
func (r *AIMDRateSetter) Shutdown() {
	r.shutdownOnce.Do(func() {
		close(r.shutdownSignal)
	})
}

// Issue submits a block to the local issuing queue.
func (r *AIMDRateSetter) Issue(block *Block) error {
	if identity.NewID(block.IssuerPublicKey()) != r.self {
		return ErrInvalidIssuer
	}
	r.initOnce.Do(func() {
		// initialize mana vectors in cache here when mana vectors are already loaded
		r.InitializeRate()
	})

	select {
	case r.issueChan <- block:
		return nil
	case <-r.shutdownSignal:
		return ErrStopped
	}
}

// Rate returns the rate of the rate setter.
func (r *AIMDRateSetter) Rate() float64 {
	return r.ownRate.Load()
}

// Size returns the size (in blocks) of the issuing queue.
func (r *AIMDRateSetter) Size() int {
	return r.issuingQueue.Size()
}

// Size returns the size (in blocks) of the issuing queue.
func (r *AIMDRateSetter) Work() int {
	return r.issuingQueue.Work()
}

// Estimate estimates the issuing time of new block.
func (r *AIMDRateSetter) Estimate() time.Duration {
	r.initOnce.Do(func() {
		// initialize mana vectors in cache here when mana vectors are already loaded
		r.InitializeRate()
	})

	var pauseUpdate time.Duration
	if r.pauseUpdates > 0 {
		pauseUpdate = time.Duration(float64(r.pauseUpdates)) * r.tangle.Scheduler.Rate()
	}
	// dummy estimate
	return lo.Max(time.Duration(math.Ceil(float64(r.Size())/r.ownRate.Load()*float64(time.Second))), pauseUpdate)
}

// rateSetting updates the rate ownRate at which blocks can be issued by the node.
func (r *AIMDRateSetter) AIMDUpdate() {
	// Return access mana or MinMana to allow zero mana nodes issue.
	ownMana := math.Max(r.tangle.Options.SchedulerParams.AccessManaMapRetrieverFunc()[r.self], MinMana)
	maxManaValue := lo.Max(append(lo.Values(r.tangle.Options.SchedulerParams.AccessManaMapRetrieverFunc()), MinMana)...)
	totalMana := r.tangle.Options.SchedulerParams.TotalAccessManaRetrieveFunc()

	ownRate := r.ownRate.Load()

	// TODO: make sure not to issue or scheduled blocks older than TSC
	if float64(r.tangle.Scheduler.NodeQueueSize(r.self)) > math.Max(Wmin, Wmax*ownMana/maxManaValue) {
		ownRate *= RateSettingDecrease
		r.pauseUpdates = r.initialPauseUpdates
	} else {
		ownRate += math.Max(RateSettingIncrease*ownMana/totalMana, 0.01)
	}
	r.ownRate.Store(math.Min(r.maxRate, ownRate))
}

func (r *AIMDRateSetter) IssuerLoop() {
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

func (r *AIMDRateSetter) issueInterval() time.Duration {
	wait := time.Duration(math.Ceil(float64(1) / r.ownRate.Load() * float64(time.Second)))
	return wait
}

func (r *AIMDRateSetter) InitializeRate() {
	if r.tangle.Options.RateSetterParams.Initial > 0.0 {
		r.ownRate.Store(r.tangle.Options.RateSetterParams.Initial)
	} else {
		ownMana := math.Max(r.tangle.Options.SchedulerParams.AccessManaMapRetrieverFunc()[r.tangle.Options.Identity.ID()], MinMana)
		totalMana := r.tangle.Options.SchedulerParams.TotalAccessManaRetrieveFunc()
		r.ownRate.Store(math.Max(ownMana/totalMana*r.maxRate, 1))
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////

// region DeficitRateSetter /////////////////////////////////////////////////////////////////////////////////////////

// Deficit rate setter uses the deficit held by the node and an estimate of its increase rate
// to decide when it is a good time to issue something.
type DeficitRateSetter struct {
	tangle         *Tangle
	Events         *RateSetterEvents
	self           identity.ID
	issuingQueue   *schedulerutils.NodeQueue
	issueChan      chan *Block
	deficitChan    chan float64
	excessDeficit  *atomic.Float64
	ownRate        *atomic.Float64
	maxRate        float64
	shutdownSignal chan struct{}
	shutdownOnce   sync.Once
	initOnce       sync.Once
}

func NewDeficitRateSetter(tangle *Tangle) *DeficitRateSetter {
	return &DeficitRateSetter{
		tangle:         tangle,
		Events:         newRateSetterEvents(),
		self:           tangle.Options.Identity.ID(),
		issuingQueue:   schedulerutils.NewNodeQueue(tangle.Options.Identity.ID()),
		issueChan:      make(chan *Block),
		deficitChan:    make(chan float64),
		excessDeficit:  atomic.NewFloat64(0),
		ownRate:        atomic.NewFloat64(0),
		maxRate:        float64(time.Second / tangle.Scheduler.Rate()),
		shutdownSignal: make(chan struct{}),
		shutdownOnce:   sync.Once{},
	}
}

func (r *DeficitRateSetter) RateSetterEvents() *RateSetterEvents {
	return r.Events
}

// Setup sets up the behavior of the component by making it attach to the relevant events of the other components.
func (r *DeficitRateSetter) Setup() {
	r.tangle.BlockFactory.Events.BlockConstructed.Attach(event.NewClosure(func(event *BlockConstructedEvent) {
		if err := r.Issue(event.Block); err != nil {
			r.Events.Error.Trigger(errors.Errorf("failed to submit to rate setter: %w", err))
		}
	}))
	// update deficit-based rate setting if own deficit is modified
	r.tangle.Scheduler.Events.OwnDeficitUpdated.Attach(event.NewClosure(func(event *OwnDeficitUpdatedEvent) {
		r.deficitChan <- event.Deficit
	}))
	go r.IssuerLoop()
}

// Issue submits a block to the local issuing queue.
func (r *DeficitRateSetter) Issue(block *Block) error {
	if identity.NewID(block.IssuerPublicKey()) != r.self {
		return ErrInvalidIssuer
	}
	r.initOnce.Do(func() {
		// initialize mana vectors in cache here when mana vectors are already loaded
		r.InitializeRate()
	})

	select {
	case r.issueChan <- block:
		return nil
	case <-r.shutdownSignal:
		return ErrStopped
	}
}

// Shutdown shuts down the RateSetter.
func (r *DeficitRateSetter) Shutdown() {
	r.shutdownOnce.Do(func() {
		close(r.shutdownSignal)
	})
}

// Rate returns the rate of the rate setter.
func (r *DeficitRateSetter) Rate() float64 {
	return r.ownRate.Load()
}

// Size returns the size (in blocks) of the issuing queue.
func (r *DeficitRateSetter) Size() int {
	return r.issuingQueue.Size()
}

// Size returns the size (in blocks) of the issuing queue.
func (r *DeficitRateSetter) Work() int {
	return r.issuingQueue.Work()
}

// Estimate estimates the issuing time of new block.
func (r *DeficitRateSetter) Estimate() time.Duration {
	// everything is in units of work (bytes) rather than blocks for deficit-based rate setter
	return time.Duration(lo.Max(0, (float64(r.Work())-r.excessDeficit.Load())/r.ownRate.Load()))
}

func (r *DeficitRateSetter) IssuerLoop() {
	var (
		nextBlockSize  = float64(0.0)
		lastDeficit    = float64(0.0)
		lastUpdateTime = float64(time.Now().Nanosecond()) / 1000000000
	)
loop:
	for {
		select {
		// if own deficit changes, check if we can issue something
		case excessDeficit := <-r.deficitChan:
			r.excessDeficit.Store(excessDeficit)
			if nextBlockSize > 0 && (excessDeficit >= nextBlockSize || r.tangle.Scheduler.ReadyBlocksCount() == 0) {
				blk := r.issuingQueue.PopFront().(*Block)
				r.Events.BlockIssued.Trigger(&BlockConstructedEvent{Block: blk})
				if next := r.issuingQueue.Front(); next != nil {
					nextBlockSize = 0.0
				} else {
					nextBlockSize = float64(next.Size())
				}
			}
			// update the deficit growth rate estimate
			currentTime := float64(time.Now().Nanosecond()) / 1000000000
			if excessDeficit > lastDeficit {
				r.ownRate.Store((excessDeficit - lastDeficit) / (currentTime - lastUpdateTime))
			}
			lastDeficit = excessDeficit
			lastUpdateTime = currentTime

		// if new block arrives, add it to the issuing queue
		case blk := <-r.issueChan:
			// if issuing queue is empty and we have enough deficit, issue this immediately
			if r.excessDeficit.Load() >= float64(blk.Size()) || r.tangle.Scheduler.ReadyBlocksCount() == 0 {
				if r.Size() == 0 {
					r.Events.BlockIssued.Trigger(&BlockConstructedEvent{Block: blk})
					break
				}
				issueBlk := r.issuingQueue.PopFront().(*Block)
				r.Events.BlockIssued.Trigger(&BlockConstructedEvent{Block: issueBlk})
			}
			// if issuing queue is full, discard this
			if r.issuingQueue.Size()+1 > MaxLocalQueueSize {
				// drop tail
				r.Events.BlockDiscarded.Trigger(&BlockDiscardedEvent{blk.ID()})
				break
			}
			// add to queue
			r.issuingQueue.Submit(blk)
			r.issuingQueue.Ready(blk)

			if next := r.issuingQueue.Front(); next != nil {
				nextBlockSize = 0.0
			} else {
				nextBlockSize = float64(next.Size())
			}

		// on close, exit the loop
		case <-r.shutdownSignal:
			break loop
		}
	}

}

func (r *DeficitRateSetter) InitializeRate() {
	if r.tangle.Options.RateSetterParams.Initial > 0.0 {
		r.ownRate.Store(r.tangle.Options.RateSetterParams.Initial)
	} else {
		ownMana := math.Max(r.tangle.Options.SchedulerParams.AccessManaMapRetrieverFunc()[r.tangle.Options.Identity.ID()], MinMana)
		totalMana := r.tangle.Options.SchedulerParams.TotalAccessManaRetrieveFunc()
		r.ownRate.Store(math.Max(ownMana/totalMana*r.maxRate, 1))
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////

// region DisabledRateSetter /////////////////////////////////////////////////////////////////////////////////////////

// Disable rate setter always allows to issue something when requested.
type DisabledRateSetter struct {
	tangle              *Tangle
	Events              *RateSetterEvents
	self                identity.ID
	issuingQueue        *schedulerutils.NodeQueue
	issueChan           chan *Block
	deficitChan         chan float64
	excessDeficit       *atomic.Float64
	ownRate             *atomic.Float64
	pauseUpdates        uint
	initialPauseUpdates uint
	maxRate             float64
	shutdownSignal      chan struct{}
	shutdownOnce        sync.Once
	initOnce            sync.Once
}

func NewDisabledRateSetter(tangle *Tangle) *DisabledRateSetter {
	return &DisabledRateSetter{
		tangle:              tangle,
		Events:              newRateSetterEvents(),
		self:                tangle.Options.Identity.ID(),
		issuingQueue:        schedulerutils.NewNodeQueue(tangle.Options.Identity.ID()),
		issueChan:           make(chan *Block),
		deficitChan:         make(chan float64),
		excessDeficit:       atomic.NewFloat64(0),
		ownRate:             atomic.NewFloat64(0),
		pauseUpdates:        0,
		initialPauseUpdates: uint(tangle.Options.RateSetterParams.RateSettingPause / tangle.Scheduler.Rate()),
		maxRate:             float64(time.Second / tangle.Scheduler.Rate()),
		shutdownSignal:      make(chan struct{}),
		shutdownOnce:        sync.Once{},
	}
}

func (r *DisabledRateSetter) RateSetterEvents() *RateSetterEvents {
	return r.Events
}

// Setup sets up the behavior of the component by making it attach to the relevant events of the other components.
func (r *DisabledRateSetter) Setup() {
	r.tangle.BlockFactory.Events.BlockConstructed.Attach(event.NewClosure(func(event *BlockConstructedEvent) {
		r.Events.BlockIssued.Trigger(event) // blocks immediately issued rather than added to issuing queue
	}))
}

// Shutdown shuts down the RateSetter.
func (r *DisabledRateSetter) Shutdown() {
	r.shutdownOnce.Do(func() {
		close(r.shutdownSignal)
	})
}

// Rate returns the rate of the rate setter.
func (r *DisabledRateSetter) Rate() float64 {
	return r.ownRate.Load()
}

// Size returns the size (in blocks) of the issuing queue.
func (r *DisabledRateSetter) Size() int {
	return r.issuingQueue.Size()
}

// Estimate estimates the issuing time of new block.
func (r *DisabledRateSetter) Estimate() time.Duration {
	return time.Duration(0) // return no wait if rate setter is disabled
}

// Size returns the size (in blocks) of the issuing queue.
func (r *DisabledRateSetter) Work() int {
	return r.issuingQueue.Work()
}

func (r *DisabledRateSetter) InitializeRate() {
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
