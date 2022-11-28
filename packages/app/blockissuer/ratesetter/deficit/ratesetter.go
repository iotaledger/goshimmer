package deficit

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

// region RateSetter ///////////////////////////////////////////////////////////////////////////////////////////////////

type RateSetter struct {
	events                *utils.Events
	protocol              *protocol.Protocol
	self                  identity.ID
	totalManaRetrieveFunc func() int64
	manaRetrieveFunc      func() map[identity.ID]int64

	issuingQueue      *utils.IssuerQueue
	issueChan         chan *models.Block
	ownRate           *atomic.Float64
	maxRate           float64
	shutdownSignal    chan struct{}
	shutdownOnce      sync.Once
	initOnce          sync.Once
	issueOnce         sync.Once
	issueMutex        sync.RWMutex
	deficitsMutex     sync.RWMutex
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
		issueChan:             make(chan *models.Block),
		ownRate:               atomic.NewFloat64(0),
		shutdownSignal:        make(chan struct{}),
	}, opts, func(r *RateSetter) {
		r.maxRate = float64(time.Second / r.optsSchedulerRate)
	}, func(r *RateSetter) {
		go r.issuerLoop()
	})
}

func (r *RateSetter) getExcessDeficit() float64 {
	r.deficitsMutex.Lock()
	defer r.deficitsMutex.Unlock()
	excessDeficit, _ := r.protocol.CongestionControl.Scheduler().GetExcessDeficit(r.self)
	return excessDeficit
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
	return time.Duration(lo.Max(0.0, (float64(r.work())-r.getExcessDeficit())/r.ownRate.Load()))
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
		issueTimer    = time.NewTimer(r.optsSchedulerRate)
		nextBlockWork = 0.0
	)
loop:
	for {
		select {
		case <-issueTimer.C:
			if r.issuingQueue.Size() == 0 { // wait until something is in the issuing queue
				issueTimer.Stop()
				break
			}
			if (r.getExcessDeficit() >= nextBlockWork) || r.protocol.CongestionControl.Scheduler().IsUncongested(r.self) {
				blk := r.issuingQueue.PopFront()
				r.events.BlockIssued.Trigger(blk)
				if next := r.issuingQueue.Front(); next != nil {
					nextBlockWork = float64(next.Work())
				} else {
					nextBlockWork = 0.0
				}
			}

		// if new block arrives, add it to the issuing queue
		case blk := <-r.issueChan:
			r.issueOnce.Do(func() {
				// issue the block and wait for it to be scheduled for up to the max time to empty a scheduler buffer.
				r.deficitsMutex.Lock()
				maxAwait := r.optsSchedulerRate * time.Duration(r.protocol.CongestionControl.Scheduler().MaxBufferSize())
				if err := r.issueBlockAndAwaitSchedule(blk, maxAwait); err != nil {
					panic(err)
				}
				r.deficitsMutex.Unlock()
			})
			issueTimer.Reset(r.optsSchedulerRate)
			// if issuing queue is full, discard this, else enqueue
			if r.issuingQueue.Size()+1 > utils.MaxLocalQueueSize {
				// drop tail
				r.events.BlockDiscarded.Trigger(blk)
			} else {
				r.issuingQueue.Enqueue(blk)
			}

		// on close, exit the loop
		case <-r.shutdownSignal:
			break loop
		}
	}

}

func (r *RateSetter) issueBlockAndAwaitSchedule(block *models.Block, maxAwait time.Duration) error {
	r.issueMutex.Lock()
	defer r.issueMutex.Unlock()
	scheduled := make(chan *scheduler.Block)
	closure := event.NewClosure(func(scheduledBlock *scheduler.Block) {
		if scheduledBlock.ID() != block.ID() {
			return
		}
		scheduled <- scheduledBlock
	})
	r.protocol.Events.CongestionControl.Scheduler.BlockScheduled.Hook(closure)
	defer r.protocol.Events.CongestionControl.Scheduler.BlockScheduled.Detach(closure)

	r.events.BlockIssued.Trigger(block)

	select {
	case <-time.After(maxAwait):
		return utils.ErrBlockWasNotScheduledInTime
	case <-scheduled:
		return nil
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithSchedulerRate(rate time.Duration) options.Option[RateSetter] {
	return func(rateSetter *RateSetter) {
		rateSetter.optsSchedulerRate = rate
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
