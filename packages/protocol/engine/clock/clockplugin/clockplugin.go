package clockplugin

import (
	"github.com/iotaledger/goshimmer/packages/core/module"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/clock"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/options"
)

// ClockPlugin is a module for the Engine that provides a notion of time.
type ClockPlugin struct {
	acceptanceTime   *clock.AnchoredTime
	confirmationTime *clock.AnchoredTime

	module.Module
}

// Provide creates a new ClockPlugin provider with the given options.
func Provide(opts ...options.Option[ClockPlugin]) module.Provider[*engine.Engine, clock.Clock] {
	return module.Provide(func(e *engine.Engine) clock.Clock {
		return options.Apply(&ClockPlugin{
			acceptanceTime:   clock.NewAnchoredTime(),
			confirmationTime: clock.NewAnchoredTime(),
		}, opts, func(c *ClockPlugin) {
			e.HookConstructed(func() {
				e.Events.Clock.AcceptanceTimeUpdated.LinkTo(c.acceptanceTime.OnUpdate)
				e.Events.Clock.ConfirmedTimeUpdated.LinkTo(c.confirmationTime.OnUpdate)

				e.LedgerState.HookInitialized(func() {
					c.acceptanceTime.Set(e.SlotTimeProvider.EndTime(e.Storage.Settings.LatestCommitment().Index()))
					c.confirmationTime.Set(e.SlotTimeProvider.EndTime(e.Storage.Settings.LatestCommitment().Index()))

					c.TriggerInitialized()
				})

				async := event.WithWorkerPool(e.Workers.CreatePool("clock", 1))
				c.HookStopped(e.Events.Consensus.BlockGadget.BlockAccepted.Hook(func(block *blockgadget.Block) {
					c.acceptanceTime.Set(block.IssuingTime())
				}, async).Unhook)

				c.HookStopped(e.Events.Consensus.BlockGadget.BlockConfirmed.Hook(func(block *blockgadget.Block) {
					c.confirmationTime.Set(block.IssuingTime())
				}, async).Unhook)

				c.HookStopped(e.Events.Consensus.SlotGadget.SlotConfirmed.Hook(func(index slot.Index) {
					c.confirmationTime.Set(e.SlotTimeProvider.EndTime(index))
				}, async).Unhook)
			})

			e.HookStopped(c.TriggerStopped)
		}, (*ClockPlugin).TriggerConstructed)
	})
}

func (c *ClockPlugin) AcceptanceTime() *clock.AnchoredTime {
	return c.acceptanceTime
}

func (c *ClockPlugin) ConfirmationTime() *clock.AnchoredTime {
	return c.confirmationTime
}

// code contract (make sure the type implements all required methods).
var _ clock.Clock = &ClockPlugin{}
