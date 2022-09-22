package engine

import (
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/congestioncontrol"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/consensus"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// region Engine ///////////////////////////////////////////////////////////////////////////////////////////////////////

type Engine struct {
	Events *Events

	IsBootstrapped    func() bool
	Ledger            *ledger.Ledger
	Tangle            *tangle.Tangle
	Consensus         *consensus.Consensus
	CongestionControl *congestioncontrol.CongestionControl

	optsTangleOptions            []options.Option[tangle.Tangle]
	optsConsensusOptions         []options.Option[consensus.Consensus]
	optsCongestionControlOptions []options.Option[congestioncontrol.CongestionControl]
}

func New(isBootstrapped func() bool, ledger *ledger.Ledger, evictionManager *eviction.Manager[models.BlockID], validatorSet *validator.Set, opts ...options.Option[Engine]) (engine *Engine) {
	return options.Apply(&Engine{}, opts, func(e *Engine) {
		e.IsBootstrapped = isBootstrapped
		e.Ledger = ledger
		e.Tangle = tangle.New(ledger, evictionManager, validatorSet, e.optsTangleOptions...)
		e.Consensus = consensus.New(e.Tangle, e.optsConsensusOptions...)
		e.CongestionControl = congestioncontrol.New(e.Consensus.Gadget, e.Tangle, func() map[identity.ID]float64 {
			panic("implement me")
		}, func() float64 {
			panic("implement me")
		}, e.optsCongestionControlOptions...)

		e.Events = NewEvents()
		e.Events.Tangle = e.Tangle.Events
		e.Events.Consensus = e.Consensus.Events
		e.Events.CongestionControl = e.CongestionControl.Events
		e.Events.Ledger = e.Ledger.Events

		e.setupTipManagerEvents()
	})
}

func (e *Engine) Shutdown() {
	e.Ledger.Shutdown()
}

func (e *Engine) setupTipManagerEvents() {

}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithTangleOptions(opts ...options.Option[tangle.Tangle]) options.Option[Engine] {
	return func(e *Engine) {
		e.optsTangleOptions = opts
	}
}

func WithConsensusOptions(opts ...options.Option[consensus.Consensus]) options.Option[Engine] {
	return func(e *Engine) {
		e.optsConsensusOptions = opts
	}
}

func WithCongestionControlOptions(opts ...options.Option[congestioncontrol.CongestionControl]) options.Option[Engine] {
	return func(e *Engine) {
		e.optsCongestionControlOptions = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
