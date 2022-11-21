package deficit

import (
	"github.com/iotaledger/goshimmer/packages/app/blockissuer/ratesetter/utils"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"
	"go.uber.org/atomic"
	"math"
	"sync"
	"time"
)

// region RateSetter ///////////////////////////////////////////////////////////////////////////////////////////////////

type RateSetter struct {
	events                *utils.Events
	protocol              *protocol.Protocol
	self                  identity.ID
	totalManaRetrieveFunc func() int64
	manaRetrieveFunc      func() map[identity.ID]int64

	issuingQueue      *utils.IssuerQueue
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

func New(protocol *protocol.Protocol, selfIdentity identity.ID, opts ...options.Option[RateSetter]) *RateSetter {

	return options.Apply(&RateSetter{
		events:                utils.NewEvents(),
		protocol:              protocol,
		self:                  selfIdentity,
		manaRetrieveFunc:      protocol.Engine().ManaTracker.ManaByIDs,
		totalManaRetrieveFunc: protocol.Engine().ManaTracker.TotalMana,
		issuingQueue:          utils.NewIssuerQueue(),
		deficitChan:           make(chan float64),
		issueChan:             make(chan *models.Block),
		excessDeficit:         atomic.NewFloat64(0),
		ownRate:               atomic.NewFloat64(0),
		shutdownSignal:        make(chan struct{}),
	}, opts, func(r *RateSetter) {
		go r.issuerLoop()
	}, (*RateSetter).setupEvents)

}

// Shutdown stops the rate setter.
func (r *RateSetter) Shutdown() {
	r.shutdownOnce.Do(func() {
		close(r.shutdownSignal)
	})
}

// Rate returns the current rate of the rate setter.
func (r *RateSetter) Rate() float64 {
	return r.ownRate.Load()
}

// Estimate returns the estimated time until the next block should be issued based on own deficit in the scheduler.
func (r *RateSetter) Estimate() time.Duration {
	return time.Duration(lo.Max(0.0, (float64(r.work())-r.excessDeficit.Load())/r.ownRate.Load()))
}

func (r *RateSetter) work() int {
	return r.issuingQueue.Work()
}

// Size returns the size of the local queue.
func (r *RateSetter) Size() int {
	return r.issuingQueue.Size()
}

// Events returns the events of the Disabled rate setter.
func (r *RateSetter) Events() *utils.Events {
	return r.events
}

// SubmitBlock submits a block to the rate setter issuing queue.
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
func (r *RateSetter) initializeRate() {
	ownMana := float64(lo.Max(r.manaRetrieveFunc()[r.self], scheduler.MinMana))
	totalMana := float64(r.totalManaRetrieveFunc())
	r.ownRate.Store(math.Max(ownMana/totalMana*r.maxRate, 1))
}

func (r *RateSetter) issuerLoop() {
	var (
		nextBlockSize  = 0.0
		lastDeficit    = 0.0
		lastUpdateTime = float64(time.Now().Nanosecond())
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
					nextBlockSize = float64(next.Size())
				} else {
					nextBlockSize = 0.0
				}
			}
			// update the deficit growth rate estimate
			currentTime := float64(time.Now().Nanosecond())
			if excessDeficit > lastDeficit {
				r.ownRate.Store((excessDeficit - lastDeficit) / (currentTime - lastUpdateTime) * float64(time.Second.Nanoseconds()))
			}
			lastDeficit = excessDeficit
			lastUpdateTime = currentTime

		// if new block arrives, add it to the issuing queue
		case blk := <-r.issueChan:
			// if issuing queue is full, discard this, else enqueue
			if r.issuingQueue.Size()+1 > utils.MaxLocalQueueSize {
				// drop tail
				r.events.BlockDiscarded.Trigger(blk)
				break
			} else {
				r.issuingQueue.Enqueue(blk)
			}
			// issue from the top of the queue if allowed
			if r.excessDeficit.Load() >= float64(blk.Size()) || r.protocol.CongestionControl.Scheduler().ReadyBlocksCount() == 0 {
				issueBlk := r.issuingQueue.PopFront()
				r.events.BlockIssued.Trigger(issueBlk)
			}
			if next := r.issuingQueue.Front(); next != nil {
				nextBlockSize = float64(next.Size())
			} else {
				nextBlockSize = 0.0
			}

		// on close, exit the loop
		case <-r.shutdownSignal:
			break loop
		}
	}

}
func (r *RateSetter) setupEvents() {
	r.protocol.Events.CongestionControl.Scheduler.OwnDeficitUpdated.Attach(event.NewClosure(func(issuerID identity.ID) {
		if issuerID == r.self {
			r.deficitChan <- r.protocol.CongestionControl.Scheduler().GetExcessDeficit(issuerID)
		}
	}))

}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithSchedulerRate(rate time.Duration) options.Option[RateSetter] {
	return func(rateSetter *RateSetter) {
		rateSetter.optsSchedulerRate = rate
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
