package deficit

import (
	"sync"
	"time"

	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/options"
)

// region RateSetter ///////////////////////////////////////////////////////////////////////////////////////////////////

type RateSetter struct {
	protocol              *protocol.Protocol
	self                  identity.ID
	totalManaRetrieveFunc func() int64
	manaRetrieveFunc      func() map[identity.ID]int64

	ownRate           *atomic.Float64
	maxRate           float64
	initOnce          sync.Once
	optsSchedulerRate time.Duration
}

func New(protocol *protocol.Protocol, selfIdentity identity.ID, opts ...options.Option[RateSetter]) *RateSetter {
	return options.Apply(&RateSetter{
		protocol:              protocol,
		self:                  selfIdentity,
		manaRetrieveFunc:      protocol.Engine().ThroughputQuota.BalanceByIDs,
		totalManaRetrieveFunc: protocol.Engine().ThroughputQuota.TotalBalance,
		ownRate:               atomic.NewFloat64(0),
	}, opts, func(r *RateSetter) {
		r.maxRate = float64(time.Second / r.optsSchedulerRate)
	})
}

func (r *RateSetter) getExcessDeficit() float64 {
	excessDeficit, _ := r.protocol.CongestionControl.Scheduler().GetExcessDeficit(r.self)
	return excessDeficit
}

// Rate returns the estimated rate of the rate setter.
func (r *RateSetter) Rate() float64 {
	r.initOnce.Do(func() {
		// initialize mana vectors in cache here when mana vectors are already loaded
		r.initializeRate()
	})
	return r.ownRate.Load()
}

// Estimate returns the estimated time until the next block should be issued based on own deficit in the scheduler.
func (r *RateSetter) Estimate() time.Duration {
	// Note: excess deficit is always 0 for <minmana issuers.
	if r.protocol.CongestionControl.Scheduler().IsUncongested() {
		return time.Duration(0)
	} else {
		// Note: this method needs to be updated to take the expected work of the incoming block as an argument.
		expectedWork := models.MaxBlockWork
		return time.Duration(lo.Max(0.0, (float64(expectedWork)-r.getExcessDeficit())/r.Rate()))
	}
}

func (r *RateSetter) initializeRate() {
	ownMana := float64(lo.Max(r.manaRetrieveFunc()[r.self], scheduler.MinMana))
	totalMana := float64(r.totalManaRetrieveFunc())
	r.ownRate.Store(ownMana / totalMana * r.maxRate)
}

func (r *RateSetter) Shutdown() {
	// don't need to shut the deficit-based ratesetter down.
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithSchedulerRate(rate time.Duration) options.Option[RateSetter] {
	return func(rateSetter *RateSetter) {
		rateSetter.optsSchedulerRate = rate
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
