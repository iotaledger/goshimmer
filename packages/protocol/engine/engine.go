package engine

import (
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/solidification"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
	"github.com/iotaledger/goshimmer/packages/protocol/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
)

// region Engine ///////////////////////////////////////////////////////////////////////////////////////////////////////

type Engine struct {
	*ledger.Ledger
	*tangle.Tangle
	*solidification.Solidification
	*consensus.Consensus

	optsTangle         []options.Option[tangle.Tangle]
	optsSolidification []options.Option[solidification.Solidification]
	optsConsensus      []options.Option[consensus.Consensus]
}

func New(ledger *ledger.Ledger, evictionManager *eviction.Manager[models.BlockID], validatorSet *validator.Set, opts ...options.Option[Engine]) (engine *Engine) {
	return options.Apply(new(Engine), opts, func(e *Engine) {
		e.Ledger = ledger
		e.Tangle = tangle.New(ledger, evictionManager, validatorSet, e.optsTangle...)
		e.Solidification = solidification.New(e.Tangle.BlockDAG, evictionManager, e.optsSolidification...)
		e.Consensus = consensus.New(e.Tangle, e.optsConsensus...)
	})
}

func (e *Engine) Shutdown() {
	e.Solidification.Shutdown()
	e.Ledger.Shutdown()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithTangleOptions(opts ...options.Option[tangle.Tangle]) options.Option[Engine] {
	return func(e *Engine) {
		e.optsTangle = opts
	}
}

func WithSolidificationOptions(opts ...options.Option[solidification.Solidification]) options.Option[Engine] {
	return func(e *Engine) {
		e.optsSolidification = opts
	}
}

func WithConsensusOptions(opts ...options.Option[consensus.Consensus]) options.Option[Engine] {
	return func(e *Engine) {
		e.optsConsensus = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
