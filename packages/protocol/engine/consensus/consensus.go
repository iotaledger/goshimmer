package consensus

import (
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/conflictresolver"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/slotgadget"
	"github.com/iotaledger/hive.go/runtime/module"
)

type Consensus interface {
	Events() *Events

	BlockGadget() blockgadget.Gadget

	SlotGadget() slotgadget.Gadget

	ConflictResolver() *conflictresolver.ConflictResolver

	module.Interface
}
