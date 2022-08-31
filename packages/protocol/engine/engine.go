package engine

import (
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
)

type Engine struct {
	*tangle.Tangle
}

func NewEngine(ledger *ledger.Ledger) (engine *Engine) {
	return &Engine{
		Tangle: tangle.New(ledger, nil, nil),
	}
}
