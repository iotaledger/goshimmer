package engine

import (
	"time"

	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/congestioncontrol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
	"github.com/iotaledger/goshimmer/packages/protocol/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
)

// region Engine ///////////////////////////////////////////////////////////////////////////////////////////////////////

type Engine struct {
	Events *Events

	Clock             *Clock
	Ledger            *ledger.Ledger
	Tangle            *tangle.Tangle
	Consensus         *consensus.Consensus
	CongestionControl *congestioncontrol.CongestionControl
	// TODO: FILL THIS IN
	TipManager bool

	optsBootstrappedThreshold time.Duration
	optsSchedulerRate         time.Duration
	optsTangle                []options.Option[tangle.Tangle]
	optsConsensus             []options.Option[consensus.Consensus]
	optsCongestionControl     []options.Option[congestioncontrol.CongestionControl]
}

func New(snapshotTime time.Time, ledger *ledger.Ledger, evictionManager *eviction.Manager[models.BlockID], validatorSet *validator.Set, opts ...options.Option[Engine]) (engine *Engine) {
	return options.Apply(&Engine{
		optsBootstrappedThreshold: 10 * time.Second,
		optsSchedulerRate:         5 * time.Millisecond,
	}, opts, func(e *Engine) {
		e.Clock = NewClock(snapshotTime)
		e.Ledger = ledger
		e.Tangle = tangle.New(ledger, evictionManager, validatorSet, e.optsTangle...)
		e.Consensus = consensus.New(e.Tangle, e.optsConsensus...)
		e.CongestionControl = congestioncontrol.New(e.Consensus.IsBlockAccepted, e.Consensus.Events.BlockAccepted, e.Tangle, func() map[identity.ID]float64 {
			panic("implement me")
		}, func() float64 {
			panic("implement me")
		}, e.optsSchedulerRate, e.optsCongestionControl...)

		e.Events = NewEvents()
		e.Events.Tangle = e.Tangle.Events
		e.Events.CongestionControl = e.CongestionControl.Events
	})
}

func (e *Engine) IsBootstrapped() (isBootstrapped bool) {
	return time.Since(e.Clock.RelativeConfirmedTime()) < e.optsBootstrappedThreshold
}

func (e *Engine) Shutdown() {
	e.Ledger.Shutdown()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithBootstrapThreshold(threshold time.Duration) options.Option[Engine] {
	return func(e *Engine) {
		e.optsBootstrappedThreshold = threshold
	}
}

func WithTangleOptions(opts ...options.Option[tangle.Tangle]) options.Option[Engine] {
	return func(e *Engine) {
		e.optsTangle = opts
	}
}

func WithConsensusOptions(opts ...options.Option[consensus.Consensus]) options.Option[Engine] {
	return func(e *Engine) {
		e.optsConsensus = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
