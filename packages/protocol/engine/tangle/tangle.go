package tangle

import (
	"github.com/iotaledger/goshimmer/packages/core/module"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
)

type Tangle interface {
	Events() *Events

	Booker() booker.Booker

	BlockDAG() blockdag.BlockDAG

	module.Interface
}
