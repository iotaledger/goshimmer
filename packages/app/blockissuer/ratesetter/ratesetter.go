package ratesetter

import (
	"math"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"
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

const (
	// The rate setter can be in one of three modes: aimd, deficit or disabled.
	AIMDMode RateSetterModeType = iota
	DeficitMode
	DisabledMode
)

type RateSetterModeType int8

func ParseRateSetterMode(s string) RateSetterModeType {
	switch strings.ToLower(s) {
	case "aimd":
		return AIMDMode
	case "disabled":
		return DisabledMode
	default:
		return DeficitMode
	}
}

func (m RateSetterModeType) String() string {
	switch m {
	case AIMDMode:
		return "aimd"
	case DisabledMode:
		return "disabled"
	default:
		return "deficit"
	}

}

var (
	// ErrInvalidIssuer is returned when an invalid block is passed to the rate setter.
	ErrInvalidIssuer = errors.New("block not issued by local node")
	// ErrStopped is returned when a block is passed to a stopped rate setter.
	ErrStopped = errors.New("rate setter stopped")
)

// region RateSetter interface ///////////////////////////////////////////////////////////////////////////////////////////////////

type RateSetter interface {
	// shutdown
	Shutdown()
	// web API
	Rate() float64
	Estimate() time.Duration
	Size() int
	Events() *Events
	// block issuer
	IssueBlock(*models.Block) error
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region DeficitRateSetter ///////////////////////////////////////////////////////////////////////////////////////////////////

type DeficitRateSetter struct {
	events                *Events
	protocol              *protocol.Protocol
	self                  identity.ID
	totalManaRetrieveFunc func() int64
	manaRetrieveFunc      func() map[identity.ID]int64

	issuingQueue      *IssuerQueue
	issueChan         chan *models.Block
	deficitChan       chan float64
	excessDeficit     *atomic.Float64
	ownRate           *atomic.Float64
	maxRate           float64
	shutdownSignal    chan struct{}
	shutdownOnce      sync.Once
	initOnce          sync.Once
	optsSchedulerRate time.Duration
}

func NewDeficit(protocol *protocol.Protocol, selfIdentity identity.ID, opts ...options.Option[DeficitRateSetter]) *DeficitRateSetter {

	return options.Apply(&DeficitRateSetter{
		events:                newEvents(),
		protocol:              protocol,
		self:                  selfIdentity,
		manaRetrieveFunc:      protocol.Engine().ManaTracker.ManaByIDs,
		totalManaRetrieveFunc: protocol.Engine().ManaTracker.TotalMana,
		issuingQueue:          NewIssuerQueue(),
		deficitChan:           make(chan float64),
		issueChan:             make(chan *models.Block),
		excessDeficit:         atomic.NewFloat64(0),
		ownRate:               atomic.NewFloat64(0),
		shutdownSignal:        make(chan struct{}),
	}, opts, func(r *DeficitRateSetter) {
		go r.issuerLoop()
	})

}

// shutdown
func (r *DeficitRateSetter) Shutdown() {
	r.shutdownOnce.Do(func() {
		close(r.shutdownSignal)
	})
}
func (r *DeficitRateSetter) Rate() float64 {
	return r.ownRate.Load()
}
func (r *DeficitRateSetter) Estimate() time.Duration {
	return time.Duration(lo.Max(0.0, (float64(r.work())-r.excessDeficit.Load())/r.ownRate.Load()))
}

func (r *DeficitRateSetter) work() int {
	return r.issuingQueue.Work()
}

func (r *DeficitRateSetter) Size() int {
	return r.issuingQueue.Size()
}

func (r *DeficitRateSetter) Events() *Events {
	return r.events
}

func (r *DeficitRateSetter) IssueBlock(block *models.Block) error {
	if identity.NewID(block.IssuerPublicKey()) != r.self {
		return ErrInvalidIssuer
	}
	r.initOnce.Do(func() {
		// initialize mana vectors in cache here when mana vectors are already loaded
		r.initializeRate()
	})

	select {
	case r.issueChan <- block:
		return nil
	case <-r.shutdownSignal:
		return ErrStopped
	}
}
func (r *DeficitRateSetter) initializeRate() {
	ownMana := float64(lo.Max(r.manaRetrieveFunc()[r.self], scheduler.MinMana))
	totalMana := float64(r.totalManaRetrieveFunc())
	r.ownRate.Store(math.Max(ownMana/totalMana*r.maxRate, 1))
}

func (r *DeficitRateSetter) issuerLoop() {
	var (
		nextBlockSize  = 0.0
		lastDeficit    = 0.0
		lastUpdateTime = float64(time.Now().Nanosecond()) / 1000000000
	)
loop:
	for {
		select {
		// if own deficit changes, check if we can issue something
		case excessDeficit := <-r.deficitChan:
			r.excessDeficit.Store(excessDeficit)
			if nextBlockSize > 0 && (excessDeficit >= nextBlockSize || r.protocol.CongestionControl.Scheduler().ReadyBlocksCount() == 0) {
				blk := r.issuingQueue.PopFront()
				r.events.BlockIssued.Trigger(blk)
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
			if r.excessDeficit.Load() >= float64(blk.Size()) || r.protocol.CongestionControl.Scheduler().ReadyBlocksCount() == 0 {
				if r.Size() == 0 {
					r.events.BlockIssued.Trigger(blk)
					break
				}
				issueBlk := r.issuingQueue.PopFront()
				r.events.BlockIssued.Trigger(issueBlk)
			}
			// if issuing queue is full, discard this
			if r.issuingQueue.Size()+1 > MaxLocalQueueSize {
				// drop tail
				r.events.BlockDiscarded.Trigger(blk)
				break
			}
			// add to queue
			r.issuingQueue.Enqueue(blk)

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
func (r *DeficitRateSetter) setupEvents() {

	// TODO: update deficit-based rate setting if own deficit is modified
	r.protocol.CongestionControl.Scheduler().Events.OwnDeficitUpdated.Attach(event.NewClosure(func(issuerID identity.ID) {
		if issuerID == r.self {
			r.deficitChan <- r.protocol.CongestionControl.Scheduler().GetExcessDeficit(issuerID)
		}
	}))

}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region AIMDRateSetter ///////////////////////////////////////////////////////////////////////////////////////////////////

// AIMDRateSetter sets the issue rate of the block issuer using an AIMD-based algorithm.
type AIMDRateSetter struct {
	events                *Events
	protocol              *protocol.Protocol
	self                  identity.ID
	totalManaRetrieveFunc func() int64
	manaRetrieveFunc      func() map[identity.ID]int64

	issuingQueue        *IssuerQueue
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

// New returns a new AIMDRateSetter.
func NewAIMD(protocol *protocol.Protocol, selfIdentity identity.ID, opts ...options.Option[AIMDRateSetter]) *AIMDRateSetter {

	return options.Apply(&AIMDRateSetter{
		events:                newEvents(),
		protocol:              protocol,
		self:                  selfIdentity,
		manaRetrieveFunc:      protocol.Engine().ManaTracker.ManaByIDs,
		totalManaRetrieveFunc: protocol.Engine().ManaTracker.TotalMana,
		issuingQueue:          NewIssuerQueue(),
		issueChan:             make(chan *models.Block),
		ownRate:               atomic.NewFloat64(0),
		shutdownSignal:        make(chan struct{}),
	}, opts, func(r *AIMDRateSetter) {
		r.initialPauseUpdates = uint(r.optsPause / r.optsSchedulerRate)
		r.maxRate = float64(time.Second / r.optsSchedulerRate)
	}, func(r *AIMDRateSetter) {
		go r.issuerLoop()
	}, (*AIMDRateSetter).setupEvents)

}

func (r *AIMDRateSetter) Events() *Events {
	return r.events
}

// Setup sets up the behavior of the component by making it attach to the relevant events of the other components.
func (r *AIMDRateSetter) setupEvents() {
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
func (r *AIMDRateSetter) IssueBlock(block *models.Block) error {

	if identity.NewID(block.IssuerPublicKey()) != r.self {
		return ErrInvalidIssuer
	}
	r.initOnce.Do(func() {
		// initialize mana vectors in cache here when mana vectors are already loaded
		r.initializeRate()
	})

	select {
	case r.issueChan <- block:
		return nil
	case <-r.shutdownSignal:
		return ErrStopped
	}
}

// Shutdown shuts down the RateSetter.
func (r *AIMDRateSetter) Shutdown() {
	r.shutdownOnce.Do(func() {
		close(r.shutdownSignal)
	})
}

// Rate returns the rate of the rate setter.
func (r *AIMDRateSetter) Rate() float64 {
	return r.ownRate.Load()
}

// Size returns the size of the issuing queue.
func (r *AIMDRateSetter) Size() int {
	return r.issuingQueue.Size()
}

// Estimate estimates the issuing time of new block.
func (r *AIMDRateSetter) Estimate() time.Duration {
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
func (r *AIMDRateSetter) rateSetting() {
	// Return access mana or MinMana to allow zero mana nodes issue.
	ownMana := float64(lo.Max(r.manaRetrieveFunc()[r.self], scheduler.MinMana))
	maxManaValue := float64(lo.Max(append(lo.Values(r.manaRetrieveFunc()), scheduler.MinMana)...))
	totalMana := float64(r.totalManaRetrieveFunc())

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

func (r *AIMDRateSetter) issuerLoop() {
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
			r.events.BlockIssued.Trigger(block)
			lastIssueTime = time.Now()

			if next := r.issuingQueue.Front(); next != nil {
				issueTimer.Reset(time.Until(lastIssueTime.Add(r.issueInterval())))
				timerStopped = false
			}

		// add a new block to the local issuer queue
		case block := <-r.issueChan:
			if r.issuingQueue.Size()+1 > MaxLocalQueueSize {
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

func (r *AIMDRateSetter) issueInterval() time.Duration {
	wait := time.Duration(math.Ceil(float64(1) / r.ownRate.Load() * float64(time.Second)))
	return wait
}

func (r *AIMDRateSetter) initializeRate() {
	if r.optsInitialRate > 0.0 {
		r.ownRate.Store(r.optsInitialRate)
	} else {
		ownMana := float64(lo.Max(r.manaRetrieveFunc()[r.self], scheduler.MinMana))
		totalMana := float64(r.totalManaRetrieveFunc())
		r.ownRate.Store(math.Max(ownMana/totalMana*r.maxRate, 1))
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region DisabledRateSetter ///////////////////////////////////////////////////////////////////////////////////////////////////

type DisabledRateSetter struct {
	events       *Events
	issuingQueue *IssuerQueue
}

func NewDisabled() *DisabledRateSetter {
	return &DisabledRateSetter{
		events: newEvents(),
	}
}

func (r *DisabledRateSetter) Shutdown() {
	// shutdown?
}

func (r *DisabledRateSetter) Rate() float64 {
	return 0
}
func (r *DisabledRateSetter) Estimate() time.Duration {
	return time.Duration(0)
}
func (r *DisabledRateSetter) Size() int {
	return r.issuingQueue.Size()
}
func (r *DisabledRateSetter) Events() *Events {
	return r.events
}

func (r *DisabledRateSetter) IssueBlock(block *models.Block) error {
	r.events.BlockIssued.Trigger(block)
	return nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithInitialRate(initialRate float64) options.Option[AIMDRateSetter] {
	return func(rateSetter *AIMDRateSetter) {
		rateSetter.optsInitialRate = initialRate
	}
}

func WithPause(pause time.Duration) options.Option[AIMDRateSetter] {
	return func(rateSetter *AIMDRateSetter) {
		rateSetter.optsPause = pause
	}
}

func WithSchedulerRateAIMD(rate time.Duration) options.Option[AIMDRateSetter] {
	return func(rateSetter *AIMDRateSetter) {
		rateSetter.optsSchedulerRate = rate
	}
}

func WithSchedulerRateDeficit(rate time.Duration) options.Option[DeficitRateSetter] {
	return func(rateSetter *DeficitRateSetter) {
		rateSetter.optsSchedulerRate = rate
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
