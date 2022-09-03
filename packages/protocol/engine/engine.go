package engine

import (
	"time"

	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
	"github.com/iotaledger/goshimmer/packages/protocol/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
)

// region Engine ///////////////////////////////////////////////////////////////////////////////////////////////////////

type Engine struct {
	Clock     *Clock
	Ledger    *ledger.Ledger
	Tangle    *tangle.Tangle
	Consensus *consensus.Consensus

	optsBootstrappedThreshold time.Duration
	optsTangle                []options.Option[tangle.Tangle]
	optsConsensus             []options.Option[consensus.Consensus]
}

func New(ledger *ledger.Ledger, evictionManager *eviction.Manager[models.BlockID], validatorSet *validator.Set, opts ...options.Option[Engine]) (engine *Engine) {
	return options.Apply(&Engine{
		optsBootstrappedThreshold: 10 * time.Second,
	}, opts, func(e *Engine) {
		// TODO: REPLACE WITH TIME OF LATEST CLEAN EPOCH
		e.Clock = NewClock(time.Now())
		e.Ledger = ledger
		e.Tangle = tangle.New(ledger, evictionManager, validatorSet, e.optsTangle...)
		e.Consensus = consensus.New(e.Tangle, e.optsConsensus...)
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
