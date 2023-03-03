package tangletime

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

// Clock is a module for the Engine that provides a notion of time.
type Clock struct {
	engine        *engine.Engine
	events        *clock.Events
	lastAccepted  *timeUpdate
	lastConfirmed *timeUpdate

	module.Module
}

// New creates a new Clock with the given options.
func New(e *engine.Engine, opts ...options.Option[Clock]) *Clock {
	return options.Apply(&Clock{
		engine:        e,
		events:        clock.NewEvents(),
		lastAccepted:  &timeUpdate{},
		lastConfirmed: &timeUpdate{},
	}, opts, (*Clock).init)
}

// NewProvider creates a new Clock provider with the given options.
func NewProvider(opts ...options.Option[Clock]) module.Provider[*engine.Engine, clock.Clock] {
	return module.Provide(func(e *engine.Engine) clock.Clock {
		return New(e, opts...)
	})
}

// Events returns the Events of the Clock.
func (c *Clock) Events() *clock.Events {
	return c.events
}

// AcceptedTime returns the Time of the last accepted Block.
func (c *Clock) AcceptedTime() (acceptedTime time.Time) {
	return c.lastAccepted.Time()
}

// RelativeAcceptedTime returns the real-Time adjusted version of the Time of the last accepted Block.
func (c *Clock) RelativeAcceptedTime() (relativeAcceptedTime time.Time) {
	return c.lastAccepted.RelativeTime()
}

// ConfirmedTime returns the Time of the last confirmed Block.
func (c *Clock) ConfirmedTime() (confirmedTime time.Time) {
	return c.lastConfirmed.Time()
}

// RelativeConfirmedTime returns the real-Time adjusted version of the Time of the last confirmed Block.
func (c *Clock) RelativeConfirmedTime() (relativeConfirmedTime time.Time) {
	return c.lastConfirmed.RelativeTime()
}

func (c *Clock) init() {
	c.engine.HookConstructed(func() {
		c.engine.Events.Clock.LinkTo(c.events)

		workers := c.engine.Workers.CreateGroup("clock")
		wpAccepted := event.WithWorkerPool(workers.CreatePool("setAcceptedTime", 1))
		wpConfirmed := event.WithWorkerPool(workers.CreatePool("setConfirmedTime", 1))

		c.engine.LedgerState.HookInitialized(func() {
			c.setAcceptedTime(c.engine.SlotTimeProvider.EndTime(c.engine.Storage.Settings.LatestCommitment().Index()))
			c.setConfirmedTime(c.engine.SlotTimeProvider.EndTime(c.engine.Storage.Settings.LatestCommitment().Index()))

			c.HookStopped(lo.Batch(
				c.engine.Events.Consensus.BlockGadget.BlockAccepted.Hook(func(block *blockgadget.Block) { c.setAcceptedTime(block.IssuingTime()) }, wpAccepted).Unhook,
				c.engine.Events.Consensus.BlockGadget.BlockConfirmed.Hook(func(block *blockgadget.Block) { c.setConfirmedTime(block.IssuingTime()) }, wpConfirmed).Unhook,
				c.engine.Events.Consensus.SlotGadget.SlotConfirmed.Hook(func(index slot.Index) { c.setConfirmedTime(c.engine.SlotTimeProvider.EndTime(index)) }, wpConfirmed).Unhook,
			))

			c.TriggerInitialized()
		})
	})
}

// setAcceptedTime sets the Time of the last accepted Block.
func (c *Clock) setAcceptedTime(acceptedTime time.Time) (updated bool) {
	now := time.Now()
	if updated = c.lastAccepted.Update(now, acceptedTime); updated {
		c.events.AcceptanceTimeUpdated.Trigger(acceptedTime, now)
	}

	return
}

// setConfirmedTime sets the Time of the last confirmed Block.
func (c *Clock) setConfirmedTime(confirmedTime time.Time) (updated bool) {
	now := time.Now()
	if updated = c.lastConfirmed.Update(now, confirmedTime); updated {
		c.events.ConfirmedTimeUpdated.Trigger(confirmedTime, now)
	}

	return
}

// code contract (make sure the type implements all required methods).
var _ clock.Clock = &Clock{}
