package engine

import (
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/consensus"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/tsc"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// region Engine ///////////////////////////////////////////////////////////////////////////////////////////////////////

type Engine struct {
	Events *Events

	IsBootstrapped func() bool
	Ledger         *ledger.Ledger
	Tangle         *tangle.Tangle
	Consensus      *consensus.Consensus
	TSCManager     *tsc.TSCManager

	optsTangleOptions     []options.Option[tangle.Tangle]
	optsConsensusOptions  []options.Option[consensus.Consensus]
	optsTSCManagerOptions []options.Option[tsc.TSCManager]
}

func New(isBootstrapped func() bool, ledger *ledger.Ledger, evictionManager *eviction.Manager[models.BlockID], validatorSet *validator.Set, opts ...options.Option[Engine]) (engine *Engine) {
	return options.Apply(&Engine{}, opts, func(e *Engine) {
		e.IsBootstrapped = isBootstrapped
		e.Ledger = ledger
		e.Tangle = tangle.New(ledger, evictionManager, validatorSet, e.optsTangleOptions...)
		e.Consensus = consensus.New(e.Tangle, e.optsConsensusOptions...)
		e.TSCManager = tsc.New(e.Consensus.IsBlockAccepted, e.Tangle)

		e.Events = NewEvents()
		e.Events.Tangle = e.Tangle.Events
		e.Events.Consensus = e.Consensus.Events
		e.Events.Ledger = e.Ledger.Events
	})
}

func (e *Engine) Shutdown() {
	e.Ledger.Shutdown()
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

func WithTSCManagerOptions(opts ...options.Option[tsc.TSCManager]) options.Option[Engine] {
	return func(e *Engine) {
		e.optsTSCManagerOptions = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
