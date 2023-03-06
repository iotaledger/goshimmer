package clock

import (
	"github.com/iotaledger/goshimmer/packages/core/module"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/options"
)

// Clock is a component that provides different notions of time for the Engine.
type Clock struct {
	// acceptanceTime is a notion of time that is anchored to the latest accepted block.
	acceptanceTime *RelativeClock

	// confirmationTime is a notion of time that is anchored to the latest confirmed block.
	confirmationTime *RelativeClock

	// Module embeds the required methods of the module.Interface.
	module.Module
}

// Provide creates a new Clock provider with the given options.
func Provide(opts ...options.Option[Clock]) module.Provider[*engine.Engine, engine.Clock] {
	return module.Provide(func(e *engine.Engine) engine.Clock {
		return options.Apply(&Clock{
			acceptanceTime:   NewRelativeClock(),
			confirmationTime: NewRelativeClock(),
		}, opts, func(c *Clock) {
			e.HookConstructed(func() {
				e.LedgerState.HookInitialized(func() {
					now := e.SlotTimeProvider.EndTime(e.Storage.Settings.LatestCommitment().Index())

					c.acceptanceTime.Set(now)
					c.confirmationTime.Set(now)

					c.TriggerInitialized()
				})

				e.Events.Clock.AcceptanceTimeAnchorUpdated.LinkTo(c.acceptanceTime.OnAnchorUpdated)
				e.Events.Clock.ConfirmationTimeAnchorUpdated.LinkTo(c.confirmationTime.OnAnchorUpdated)

				async := event.WithWorkerPool(e.Workers.CreatePool("Clock", 1))
				c.HookStopped(lo.Batch(
					e.Events.Consensus.BlockGadget.BlockAccepted.Hook(func(block *blockgadget.Block) {
						c.acceptanceTime.Advance(block.IssuingTime())
					}, async).Unhook,

					e.Events.Consensus.BlockGadget.BlockConfirmed.Hook(func(block *blockgadget.Block) {
						c.confirmationTime.Advance(block.IssuingTime())
					}, async).Unhook,

					e.Events.Consensus.SlotGadget.SlotConfirmed.Hook(func(index slot.Index) {
						c.confirmationTime.Advance(e.SlotTimeProvider.EndTime(index))
					}, async).Unhook,
				))
			})

			e.HookStopped(c.TriggerStopped)
		}, (*Clock).TriggerConstructed)
	})
}

// AcceptanceTime returns a notion of time that is anchored to the latest accepted block.
func (c *Clock) Acceptance() engine.RelativeClock {
	return c.acceptanceTime
}

// Confirmation returns a notion of time that is anchored to the latest confirmed block.
func (c *Clock) Confirmation() engine.RelativeClock {
	return c.confirmationTime
}
