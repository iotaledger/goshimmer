package tangletime

import (
	"time"

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
	events        *clock.Events
	lastAccepted  *timeUpdate
	lastConfirmed *timeUpdate

	module.Module
}

// New creates a new Clock with the given options.
func New(e *engine.Engine, opts ...options.Option[Clock]) *Clock {
	return options.Apply(&Clock{
		events:        clock.NewEvents(),
		lastAccepted:  &timeUpdate{},
		lastConfirmed: &timeUpdate{},
	}, opts, func(c *Clock) {
		e.HookConstructed(func() {
			e.Events.Clock.LinkTo(c.events)

			worker := event.WithWorkerPool(e.Workers.CreatePool("clock", 1))
			c.HookStopped(e.Events.Consensus.BlockGadget.BlockAccepted.Hook(c.onBlockAccepted, worker).Unhook)
			c.HookStopped(e.Events.Consensus.BlockGadget.BlockConfirmed.Hook(c.onBlockConfirmed, worker).Unhook)
			c.HookStopped(e.Events.Consensus.SlotGadget.SlotConfirmed.Hook(func(index slot.Index) {
				c.SetConfirmedTime(e.SlotTimeProvider.EndTime(index))
			}, worker).Unhook)

			e.LedgerState.HookInitialized(func() {
				c.SetAcceptedTime(e.SlotTimeProvider.EndTime(e.Storage.Settings.LatestCommitment().Index()))
				c.SetConfirmedTime(e.SlotTimeProvider.EndTime(e.Storage.Settings.LatestCommitment().Index()))

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

// AcceptedTime returns the Time of the last accepted Block.
func (c *Clock) AcceptedTime() (acceptedTime time.Time) {
	return c.lastAccepted.Time()
}

// RelativeAcceptedTime returns the real-Time adjusted version of the Time of the last accepted Block.
func (c *Clock) RelativeAcceptedTime() (relativeAcceptedTime time.Time) {
	return c.lastAccepted.RelativeTime()
}

// SetAcceptedTime sets the time of the last accepted Block.
func (c *Clock) SetAcceptedTime(acceptedTime time.Time) {
	if now := time.Now(); c.lastAccepted.Update(now, acceptedTime) {
		c.events.AcceptanceTimeUpdated.Trigger(acceptedTime, now)
	}
}

// ConfirmedTime returns the Time of the last confirmed Block.
func (c *Clock) ConfirmedTime() (confirmedTime time.Time) {
	return c.lastConfirmed.Time()
}

// RelativeConfirmedTime returns the real-Time adjusted version of the Time of the last confirmed Block.
func (c *Clock) RelativeConfirmedTime() (relativeConfirmedTime time.Time) {
	return c.lastConfirmed.RelativeTime()
}

// SetConfirmedTime sets the Time of the last confirmed Block.
func (c *Clock) SetConfirmedTime(confirmedTime time.Time) {
	if now := time.Now(); c.lastConfirmed.Update(now, confirmedTime) {
		c.events.ConfirmedTimeUpdated.Trigger(confirmedTime, now)
	}
}

// Events returns the Events of the Clock.
func (c *Clock) Events() *clock.Events {
	return c.events
}

func (c *Clock) onBlockAccepted(block *blockgadget.Block) {
	c.SetAcceptedTime(block.IssuingTime())
}

func (c *Clock) onBlockConfirmed(block *blockgadget.Block) {
	c.SetConfirmedTime(block.IssuingTime())
}

// code contract (make sure the type implements all required methods).
var _ clock.Clock = &Clock{}
