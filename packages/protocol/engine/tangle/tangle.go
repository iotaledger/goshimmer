package tangle

import (
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/hive.go/runtime/module"
)

type Tangle interface {
	Events() *Events

	Booker() booker.Booker

	BlockDAG() blockdag.BlockDAG

	module.Interface
}
