package engine

import (
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/core/eviction"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
)

// region Engine ///////////////////////////////////////////////////////////////////////////////////////////////////////

type Engine struct {
	*tangle.Tangle

	optsTangle []options.Option[tangle.Tangle]
}

func NewEngine(ledger *ledger.Ledger, evictionManager *eviction.Manager[models.BlockID], validatorSet *validator.Set, opts ...options.Option[Engine]) (engine *Engine) {
	return options.Apply(new(Engine), opts, func(e *Engine) {
		e.Tangle = tangle.New(ledger, evictionManager, validatorSet, e.optsTangle...)
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithTangleOptions(tangleOptions ...options.Option[tangle.Tangle]) options.Option[Engine] {
	return func(e *Engine) {
		e.optsTangle = tangleOptions
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
