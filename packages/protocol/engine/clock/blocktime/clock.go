package blocktime

import (
	"github.com/iotaledger/goshimmer/packages/core/module"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/clock"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/options"
)

// Clock is a component that provides different notions of time for the Engine.
type Clock struct {
	// acceptedClock is a notion of time that is anchored to the latest accepted block.
	acceptedClock *RelativeClock

	// confirmedClock is a notion of time that is anchored to the latest confirmed block.
	confirmedClock *RelativeClock

	// Module embeds the required methods of the module.Interface.
	module.Module
}

// NewProvider creates a new Clock provider with the given options.
func NewProvider(opts ...options.Option[Clock]) module.Provider[*engine.Engine, clock.Clock] {
	return module.Provide(func(e *engine.Engine) clock.Clock {
		return options.Apply(&Clock{
			acceptedClock:  NewRelativeClock(),
			confirmedClock: NewRelativeClock(),
		}, opts, func(c *Clock) {
			e.HookConstructed(func() {
				e.LedgerState.HookInitialized(func() {
					c.acceptedClock.Set(e.SlotTimeProvider.EndTime(e.Storage.Settings.LatestCommitment().Index()))
					c.confirmedClock.Set(e.SlotTimeProvider.EndTime(e.Storage.Settings.LatestCommitment().Index()))

					c.TriggerInitialized()
				})

				e.Events.Clock.AcceptedTimeUpdated.LinkTo(c.acceptedClock.OnAnchorUpdated)
				e.Events.Clock.ConfirmedTimeUpdated.LinkTo(c.confirmedClock.OnAnchorUpdated)

				async := event.WithWorkerPool(e.Workers.CreatePool("Clock", 1))
				c.HookStopped(lo.Batch(
					e.Events.Consensus.BlockGadget.BlockAccepted.Hook(func(block *blockgadget.Block) {
						c.acceptedClock.Advance(block.IssuingTime())
					}, async).Unhook,

					e.Events.Consensus.BlockGadget.BlockConfirmed.Hook(func(block *blockgadget.Block) {
						c.confirmedClock.Advance(block.IssuingTime())
					}, async).Unhook,

					e.Events.Consensus.SlotGadget.SlotConfirmed.Hook(func(index slot.Index) {
						c.confirmedClock.Advance(e.SlotTimeProvider.EndTime(index))
					}, async).Unhook,
				))
			})

			e.HookStopped(c.TriggerStopped)
		}, (*Clock).TriggerConstructed)
	})
}

// Accepted returns a notion of time that is anchored to the latest accepted block.
func (c *Clock) Accepted() clock.RelativeClock {
	return c.acceptedClock
}

// Confirmed returns a notion of time that is anchored to the latest confirmed block.
func (c *Clock) Confirmed() clock.RelativeClock {
	return c.confirmedClock
}
