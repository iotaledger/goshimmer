package tangleconsensus

import (
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget/tresholdblockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/conflictresolver"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/slotgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/slotgadget/totalweightslotgadget"
	"github.com/iotaledger/hive.go/runtime/module"
	"github.com/iotaledger/hive.go/runtime/options"
)

// region Consensus ////////////////////////////////////////////////////////////////////////////////////////////////////

type Consensus struct {
	events *consensus.Events

	blockGadget      blockgadget.Gadget
	slotGadget       slotgadget.Gadget
	conflictResolver *conflictresolver.ConflictResolver

	optsBlockGadgetProvider module.Provider[*engine.Engine, blockgadget.Gadget]
	optsSlotGadgetProvider  module.Provider[*engine.Engine, slotgadget.Gadget]

	module.Module
}

func NewProvider(opts ...options.Option[Consensus]) module.Provider[*engine.Engine, consensus.Consensus] {
	return module.Provide(func(e *engine.Engine) consensus.Consensus {
		return options.Apply(&Consensus{
			events:                  consensus.NewEvents(),
			optsBlockGadgetProvider: tresholdblockgadget.NewProvider(),
			optsSlotGadgetProvider:  totalweightslotgadget.NewProvider(),
		}, opts, func(c *Consensus) {
			c.blockGadget = c.optsBlockGadgetProvider(e)
			c.slotGadget = c.optsSlotGadgetProvider(e)

			c.events.BlockGadget.LinkTo(c.blockGadget.Events())
			c.events.SlotGadget.LinkTo(c.slotGadget.Events())

			e.HookConstructed(func() {
				c.conflictResolver = conflictresolver.New(e.Ledger.MemPool().ConflictDAG(), e.Tangle.Booker().VirtualVoting().ConflictVotersTotalWeight)

				e.Events.Consensus.LinkTo(c.events)

				e.Events.Consensus.BlockGadget.Error.Hook(e.Events.Error.Trigger)

				c.TriggerConstructed()
				e.HookInitialized(c.TriggerInitialized)
			})
		})
	})
}

func (c *Consensus) Events() *consensus.Events {
	return c.events
}

func (c *Consensus) BlockGadget() blockgadget.Gadget {
	return c.blockGadget
}

func (c *Consensus) SlotGadget() slotgadget.Gadget {
	return c.slotGadget
}

func (c *Consensus) ConflictResolver() *conflictresolver.ConflictResolver {
	return c.conflictResolver
}

var _ consensus.Consensus = new(Consensus)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithBlockGadgetProvider(provider module.Provider[*engine.Engine, blockgadget.Gadget]) options.Option[Consensus] {
	return func(c *Consensus) {
		c.optsBlockGadgetProvider = provider
	}
}

func WithSlotGadgetProvider(provider module.Provider[*engine.Engine, slotgadget.Gadget]) options.Option[Consensus] {
	return func(c *Consensus) {
		c.optsSlotGadgetProvider = provider
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
