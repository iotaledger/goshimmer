package module

import (
	"github.com/iotaledger/goshimmer/packages/core/module"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/clock"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/options"
)

// Clock is a component that provides different notions of time for the Engine.
type Clock struct {
	// acceptanceTime is a notion of time that is anchored to the latest accepted block.
	acceptanceTime *clock.AnchoredTime

	// confirmationTime is a notion of time that is anchored to the latest confirmed block.
	confirmationTime *clock.AnchoredTime

	// Module embeds the required methods of the module.Interface.
	module.Module
}

// Provide creates a new Clock provider with the given options.
func Provide(opts ...options.Option[Clock]) module.Provider[*engine.Engine, clock.Clock] {
	return module.Provide(func(e *engine.Engine) clock.Clock {
		return options.Apply(&Clock{
			acceptanceTime:   clock.NewAnchoredTime(),
			confirmationTime: clock.NewAnchoredTime(),
		}, opts, func(c *Clock) {
			e.HookConstructed(func() {
				e.Events.Clock.AcceptanceTimeUpdated.LinkTo(c.acceptanceTime.OnUpdate)
				e.Events.Clock.ConfirmedTimeUpdated.LinkTo(c.confirmationTime.OnUpdate)

				e.LedgerState.HookInitialized(func() {
					c.acceptanceTime.Set(e.SlotTimeProvider.EndTime(e.Storage.Settings.LatestCommitment().Index()))
					c.confirmationTime.Set(e.SlotTimeProvider.EndTime(e.Storage.Settings.LatestCommitment().Index()))

					c.TriggerInitialized()
				})

				async := event.WithWorkerPool(e.Workers.CreatePool("ClockPlugin", 1))

				c.HookStopped(e.Events.Consensus.BlockGadget.BlockAccepted.Hook(func(block *blockgadget.Block) { c.acceptanceTime.Set(block.IssuingTime()) }, async).Unhook)
				c.HookStopped(e.Events.Consensus.BlockGadget.BlockConfirmed.Hook(func(block *blockgadget.Block) { c.confirmationTime.Set(block.IssuingTime()) }, async).Unhook)
				c.HookStopped(e.Events.Consensus.SlotGadget.SlotConfirmed.Hook(func(index slot.Index) { c.confirmationTime.Set(e.SlotTimeProvider.EndTime(index)) }, async).Unhook)
			})

			e.HookStopped(c.TriggerStopped)
		}, (*Clock).TriggerConstructed)
	})
}

// AcceptanceTime returns a notion of time that is anchored to the latest accepted block.
func (c *Clock) AcceptanceTime() *clock.AnchoredTime {
	return c.acceptanceTime
}

// ConfirmationTime returns a notion of time that is anchored to the latest confirmed block.
func (c *Clock) ConfirmationTime() *clock.AnchoredTime {
	return c.confirmationTime
}

// code contract (make sure the type implements all required methods).
var _ clock.Clock = &Clock{}
