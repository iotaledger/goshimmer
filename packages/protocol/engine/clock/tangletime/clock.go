package tangletime

import (
	"github.com/iotaledger/goshimmer/packages/core/module"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/clock"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/options"
)

// Clock is a module for the Engine that provides a notion of time.
type Clock struct {
	events           *clock.Events
	acceptanceTime   *clock.AnchoredTime
	confirmationTime *clock.AnchoredTime

	module.Module
}

// New creates a new Clock with the given options.
func New(e *engine.Engine, opts ...options.Option[Clock]) *Clock {
	return options.Apply(&Clock{
		acceptanceTime:   clock.NewAnchoredTime(),
		confirmationTime: clock.NewAnchoredTime(),
	}, opts, func(c *Clock) {
		e.HookConstructed(func() {
			e.Events.Clock.AcceptanceTimeUpdated.LinkTo(c.acceptanceTime.OnUpdate)
			e.Events.Clock.ConfirmedTimeUpdated.LinkTo(c.confirmationTime.OnUpdate)

			async := event.WithWorkerPool(e.Workers.CreatePool("clock", 1))

			c.HookStopped(e.Events.Consensus.BlockGadget.BlockAccepted.Hook(c.onBlockAccepted, async).Unhook)
			c.HookStopped(e.Events.Consensus.BlockGadget.BlockConfirmed.Hook(c.onBlockConfirmed, async).Unhook)
			c.HookStopped(e.Events.Consensus.SlotGadget.SlotConfirmed.Hook(func(index slot.Index) {
				c.confirmationTime.Set(e.SlotTimeProvider.EndTime(index))
			}, async).Unhook)

			e.LedgerState.HookInitialized(func() {
				c.acceptanceTime.Set(e.SlotTimeProvider.EndTime(e.Storage.Settings.LatestCommitment().Index()))
				c.confirmationTime.Set(e.SlotTimeProvider.EndTime(e.Storage.Settings.LatestCommitment().Index()))

				c.TriggerInitialized()
			})
		})

		e.HookStopped(c.TriggerStopped)
	}, (*Clock).TriggerConstructed)
}

// NewProvider creates a new Clock provider with the given options.
func NewProvider(opts ...options.Option[Clock]) module.Provider[*engine.Engine, clock.Clock] {
	return module.Provide(func(e *engine.Engine) clock.Clock {
		return New(e, opts...)
	})
}

func (c *Clock) AcceptanceTime() *clock.AnchoredTime {
	return c.acceptanceTime
}

func (c *Clock) ConfirmationTime() *clock.AnchoredTime {
	return c.confirmationTime
}

func (c *Clock) onBlockAccepted(block *blockgadget.Block) {
	c.acceptanceTime.Set(block.IssuingTime())
}

func (c *Clock) onBlockConfirmed(block *blockgadget.Block) {
	c.confirmationTime.Set(block.IssuingTime())
}

// code contract (make sure the type implements all required methods).
var _ clock.Clock = &Clock{}
