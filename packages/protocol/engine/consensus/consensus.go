package consensus

import (
	"github.com/iotaledger/goshimmer/packages/core/module"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/conflictresolver"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/slotgadget"
)

type Consensus interface {
	Events() *Events

	BlockGadget() blockgadget.Gadget

	SlotGadget() slotgadget.Gadget

	ConflictResolver() *conflictresolver.ConflictResolver

	module.Interface
}
