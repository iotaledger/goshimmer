package module

import (
	"time"

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
	// engine contains a reference to the Engine.
	engine *engine.Engine

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
			engine:           e,
			acceptanceTime:   clock.NewAnchoredTime(),
			confirmationTime: clock.NewAnchoredTime(),
		}, opts, func(c *Clock) {
			e.HookConstructed(func() {
				c.setupEvents(event.WithWorkerPool(e.Workers.CreatePool("Clock", 1)))

				e.LedgerState.HookInitialized(func() {
					c.setInitialTime(e.SlotTimeProvider.EndTime(e.Storage.Settings.LatestCommitment().Index()))

					c.TriggerInitialized()
				})
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

// setInitialTime initializes the time values of the Clock.
func (c *Clock) setInitialTime(now time.Time) {
	c.acceptanceTime.Set(now)
	c.confirmationTime.Set(now)
}

// setupEvents connects the Clock to the Engine events.
func (c *Clock) setupEvents(workerPool event.Option) {
	c.engine.Events.Clock.AcceptanceTimeUpdated.LinkTo(c.acceptanceTime.OnAnchorUpdated)
	c.engine.Events.Clock.ConfirmationTimeUpdated.LinkTo(c.confirmationTime.OnAnchorUpdated)

	c.HookStopped(lo.Batch(
		c.engine.Events.Consensus.BlockGadget.BlockAccepted.Hook(c.onBlockAccepted, workerPool).Unhook,
		c.engine.Events.Consensus.BlockGadget.BlockConfirmed.Hook(c.onBlockConfirmed, workerPool).Unhook,
		c.engine.Events.Consensus.SlotGadget.SlotConfirmed.Hook(c.onSlotConfirmed, workerPool).Unhook,
	))
}

// onBlockAccepted is called when a block is accepted.
func (c *Clock) onBlockAccepted(block *blockgadget.Block) {
	c.acceptanceTime.Advance(block.IssuingTime())
}

// onBlockConfirmed is called when a block is confirmed.
func (c *Clock) onBlockConfirmed(block *blockgadget.Block) {
	c.confirmationTime.Advance(block.IssuingTime())
}

// onSlotConfirmed is called when a slot is confirmed.
func (c *Clock) onSlotConfirmed(index slot.Index) {
	c.confirmationTime.Advance(c.engine.SlotTimeProvider.EndTime(index))
}
