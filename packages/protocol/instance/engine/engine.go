package engine

import (
	"time"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/clock"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/congestioncontrol"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/consensus"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/tip"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// region Engine ///////////////////////////////////////////////////////////////////////////////////////////////////////

type Engine struct {
	Events *Events

	Clock             *clock.Clock
	Ledger            *ledger.Ledger
	Tangle            *tangle.Tangle
	Consensus         *consensus.Consensus
	CongestionControl *congestioncontrol.CongestionControl
	TipManager        *tip.Manager

	optsBootstrappedThreshold time.Duration
	optsTangle                []options.Option[tangle.Tangle]
	optsConsensus             []options.Option[consensus.Consensus]
	optsCongestionControl     []options.Option[congestioncontrol.CongestionControl]
}

func New(snapshotTime time.Time, ledger *ledger.Ledger, evictionManager *eviction.Manager[models.BlockID], validatorSet *validator.Set, opts ...options.Option[Engine]) (engine *Engine) {
	return options.Apply(&Engine{
		optsBootstrappedThreshold: 10 * time.Second,
	}, opts, func(e *Engine) {
		e.Clock = clock.NewClock(snapshotTime)
		e.Ledger = ledger
		e.Tangle = tangle.New(ledger, evictionManager, validatorSet, e.optsTangle...)
		e.Consensus = consensus.New(e.Tangle, e.optsConsensus...)
		e.CongestionControl = congestioncontrol.New(e.Consensus.Gadget, e.Tangle, func() map[identity.ID]float64 {
			panic("implement me")
		}, func() float64 {
			panic("implement me")
		}, e.optsCongestionControl...)
		e.TipManager = tip.NewManager(e.Tangle, e.Consensus.Gadget, e.CongestionControl.Scheduler.Block, e.Clock.AcceptedTime, snapshotTime)

		e.Events = NewEvents()
		e.Events.Clock = e.Clock.Events
		e.Events.Tangle = e.Tangle.Events
		e.Events.Consensus = e.Consensus.Events
		e.Events.CongestionControl = e.CongestionControl.Events
		e.Events.TipManager = e.TipManager.Events

		e.setupTipManagerEvents()
	})
}

func (e *Engine) IsBootstrapped() (isBootstrapped bool) {
	return time.Since(e.Clock.RelativeConfirmedTime()) < e.optsBootstrappedThreshold
}

func (e *Engine) Shutdown() {
	e.Ledger.Shutdown()
}

func (e *Engine) setupTipManagerEvents() {
	e.Events.CongestionControl.Scheduler.BlockScheduled.Attach(event.NewClosure(e.TipManager.AddTip))

	e.Events.Consensus.Acceptance.BlockAccepted.Attach(event.NewClosure(func(block *acceptance.Block) {
		e.TipManager.RemoveStrongParents(block.ModelsBlock)
	}))

	e.Events.Tangle.BlockDAG.BlockOrphaned.Hook(event.NewClosure(func(block *blockdag.Block) {
		if schedulerBlock, exists := e.CongestionControl.Scheduler.Block(block.ID()); exists {
			e.TipManager.DeleteTip(schedulerBlock)
		}
	}))

	// TODO: enable once this event is implemented
	// t.tangle.Manager.Events.AllChildrenOrphaned.Hook(event.NewClosure(func(block *Block) {
	// 	if clock.Since(block.IssuingTime()) > tipLifeGracePeriod {
	// 		return
	// 	}
	//
	// 	t.addTip(block)
	// }))
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
