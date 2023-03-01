package aimd

import (
	"math"
	"sync"
	"time"

	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/options"
)

const (
	// RateSettingIncrease is the global additive increase parameter.
	// In the Access Control for Distributed Ledgers in the Internet of Things: A Networking Approach paper this parameter is denoted as "A".
	RateSettingIncrease = 0.075
	// RateSettingDecrease global multiplicative decrease parameter (larger than 1).
	// In the Access Control for Distributed Ledgers in the Internet of Things: A Networking Approach paper this parameter is denoted as "β".
	RateSettingDecrease = 0.7
	// WMax is the maximum inbox threshold for the node. This value denotes when maximum active mana holder backs off its rate.
	WMax = 10
	// WMin is the min inbox threshold for the node. This value denotes when nodes with the least mana back off their rate.
	WMin = 5
)

// region RateSetter ///////////////////////////////////////////////////////////////////////////////////////////////////

// RateSetter sets the issue rate of the block issuer using an AIMD-based algorithm.
type RateSetter struct {
	protocol              *protocol.Protocol
	self                  identity.ID
	totalManaRetrieveFunc func() int64
	manaRetrieveFunc      func() map[identity.ID]int64

	rateUpdateChan      chan *models.Block
	creditUpdateChan    chan *models.Block
	ownRate             *atomic.Float64
	pauseUpdates        uint
	initialPauseUpdates uint
	maxRate             float64
	shutdownSignal      chan struct{}
	shutdownOnce        sync.Once
	initOnce            sync.Once
	issueCredits        float64
	creditsMutex        sync.RWMutex
	maxCredits          float64

	optsInitialRate   float64
	optsPause         time.Duration
	optsSchedulerRate time.Duration
}

// New returns a new AIMD RateSetter.
func New(protocol *protocol.Protocol, selfIdentity identity.ID, opts ...options.Option[RateSetter]) *RateSetter {
	return options.Apply(&RateSetter{
		protocol:              protocol,
		self:                  selfIdentity,
		manaRetrieveFunc:      protocol.Engine().ThroughputQuota.BalanceByIDs,
		totalManaRetrieveFunc: protocol.Engine().ThroughputQuota.TotalBalance,
		rateUpdateChan:        make(chan *models.Block),
		creditUpdateChan:      make(chan *models.Block),
		ownRate:               atomic.NewFloat64(0),
		issueCredits:          0.0,
		maxCredits:            models.MaxBlockWork,
		shutdownSignal:        make(chan struct{}),
	}, opts, func(r *RateSetter) {
		r.initialPauseUpdates = uint(r.optsPause / r.optsSchedulerRate)
		r.maxRate = float64(time.Second / r.optsSchedulerRate)
	}, func(r *RateSetter) {
		go r.rateSetting()
	}, (*RateSetter).setupEvents)
}

// Setup sets up the behavior of the component by making it attach to the relevant events of the other components.
func (r *RateSetter) setupEvents() {
	wp := r.protocol.Workers.CreatePool("RateSetter", 2)

	r.protocol.Events.CongestionControl.Scheduler.BlockScheduled.Hook(func(block *scheduler.Block) {
		if r.pauseUpdates > 0 {
			r.pauseUpdates--
			return
		}
		// update credits every time a block is scheduled.
		r.creditUpdateChan <- block.ModelsBlock
		// only update rate when the issuer is active
		if r.protocol.CongestionControl.Scheduler().IssuerQueueSize(r.self) > 0 {
			r.rateUpdateChan <- block.ModelsBlock
		}
	}, event.WithWorkerPool(wp))

	r.protocol.Events.CongestionControl.Scheduler.BlockSubmitted.Hook(func(block *scheduler.Block) {
		if block.IssuerID() == r.self {
			r.updateIssueCredits(float64(-block.Work()))
		}
	}, event.WithWorkerPool(wp))
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
	// dummy estimate of work, replace with estimated work as argument.
	estimatedWork := float64(models.MaxBlockWork)
	return lo.Max(time.Duration(0), pauseUpdate+time.Duration((r.getIssueCredits()-estimatedWork)/r.ownRate.Load()))
}

func (r *RateSetter) rateSetting() {
	var lastCreditUpdate = time.Now()

loop:
	for {
		select {
		case <-r.rateUpdateChan:
			ownMana := float64(lo.Max(r.manaRetrieveFunc()[r.self], scheduler.MinMana))
			maxManaValue := float64(lo.Max(append(lo.Values(r.manaRetrieveFunc()), scheduler.MinMana)...))
			totalMana := float64(r.totalManaRetrieveFunc())
			ownRate := r.ownRate.Load()
			// TODO: make sure not to issue or scheduled blocks older than TSC
			if float64(r.protocol.CongestionControl.Scheduler().IssuerQueueSize(r.self)) > math.Max(WMin, WMax*ownMana/maxManaValue) {
				ownRate *= RateSettingDecrease
				r.pauseUpdates = r.initialPauseUpdates
			} else {
				ownRate += math.Max(RateSettingIncrease*ownMana/totalMana, 0.01)
			}
			r.ownRate.Store(math.Min(r.maxRate, ownRate))

		case <-r.creditUpdateChan:
			ownRate := r.ownRate.Load()
			r.updateIssueCredits(ownRate * float64(time.Since(lastCreditUpdate)))
			lastCreditUpdate = time.Now()

		// on close, exit the loop
		case <-r.shutdownSignal:
			break loop
		}
	}
}

func (r *RateSetter) updateIssueCredits(credits float64) {
	r.creditsMutex.Lock()
	defer r.creditsMutex.Unlock()

	r.issueCredits += credits
}

func (r *RateSetter) getIssueCredits() float64 {
	r.creditsMutex.Lock()
	defer r.creditsMutex.Unlock()

	return r.issueCredits
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
