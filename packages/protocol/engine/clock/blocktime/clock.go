package blocktime

import (
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/clock"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/module"
	"github.com/iotaledger/hive.go/runtime/options"
)

// Clock implements the clock.Clock interface that sources its notion of time from accepted and confirmed blocks.
type Clock struct {
	// acceptedTime contains a notion of time that is anchored to the latest accepted block.
	acceptedTime *RelativeTime

	// confirmedTime contains a notion of time that is anchored to the latest confirmed block.
	confirmedTime *RelativeTime

	// Module embeds the required methods of the module.Interface.
	module.Module
}

// NewProvider creates a new Clock provider with the given options.
func NewProvider(opts ...options.Option[Clock]) module.Provider[*engine.Engine, clock.Clock] {
	return module.Provide(func(e *engine.Engine) clock.Clock {
		return options.Apply(&Clock{
			acceptedTime:  NewRelativeTime(),
			confirmedTime: NewRelativeTime(),
		}, opts, func(c *Clock) {
			e.HookConstructed(func() {
				e.Ledger.HookInitialized(func() {
					c.acceptedTime.Set(e.SlotTimeProvider().EndTime(e.Storage.Settings.LatestCommitment().Index()))
					c.confirmedTime.Set(e.SlotTimeProvider().EndTime(e.Storage.Settings.LatestCommitment().Index()))

					c.TriggerInitialized()
				})

				e.Events.Clock.AcceptedTimeUpdated.LinkTo(c.acceptedTime.OnUpdated)
				e.Events.Clock.ConfirmedTimeUpdated.LinkTo(c.confirmedTime.OnUpdated)

				asyncOpt := event.WithWorkerPool(e.Workers.CreatePool("Clock", 1))
				c.HookStopped(lo.Batch(
					e.Events.Consensus.BlockGadget.BlockAccepted.Hook(func(block *blockgadget.Block) {
						c.acceptedTime.Advance(block.IssuingTime())
					}, asyncOpt).Unhook,

					e.Events.Consensus.BlockGadget.BlockConfirmed.Hook(func(block *blockgadget.Block) {
						c.acceptedTime.Advance(block.IssuingTime())
						c.confirmedTime.Advance(block.IssuingTime())
					}, asyncOpt).Unhook,

					e.Events.Consensus.SlotGadget.SlotConfirmed.Hook(func(index slot.Index) {
						c.acceptedTime.Advance(e.SlotTimeProvider().EndTime(index))
						c.confirmedTime.Advance(e.SlotTimeProvider().EndTime(index))
					}, asyncOpt).Unhook,
				))
			})

			e.HookStopped(c.TriggerStopped)
		}, (*Clock).TriggerConstructed)
	})
}

// Accepted returns a notion of time that is anchored to the latest accepted block.
func (c *Clock) Accepted() clock.RelativeTime {
	return c.acceptedTime
}

// Confirmed returns a notion of time that is anchored to the latest confirmed block.
func (c *Clock) Confirmed() clock.RelativeTime {
	return c.confirmedTime
}
