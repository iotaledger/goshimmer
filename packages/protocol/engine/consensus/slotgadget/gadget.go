package slotgadget

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/core/votes/slottracker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

type Gadget struct {
	Events *Events

	tangle              *tangle.Tangle
	lastConfirmedSlot   slot.Index
	totalWeightCallback func() int64

	optsSlotConfirmationThreshold float64

	workers *workerpool.Group

	sync.RWMutex
}

func New(workers *workerpool.Group, tangle *tangle.Tangle, lastConfirmedSlot slot.Index, totalWeightCallback func() int64, opts ...options.Option[Gadget]) (gadget *Gadget) {
	return options.Apply(&Gadget{
		workers:                       workers,
		optsSlotConfirmationThreshold: 0.67,
	}, opts, func(a *Gadget) {
		a.Events = NewEvents()

		a.tangle = tangle
		a.lastConfirmedSlot = lastConfirmedSlot
		a.totalWeightCallback = totalWeightCallback
	}, (*Gadget).setup)
}

func (g *Gadget) LastConfirmedSlot() slot.Index {
	g.RLock()
	defer g.RUnlock()

	return g.lastConfirmedSlot
}

func (g *Gadget) setLastConfirmedSlot(i slot.Index) {
	g.Lock()
	defer g.Unlock()

	g.lastConfirmedSlot = i
}

func (g *Gadget) setup() {
	g.tangle.Booker.VirtualVoting.Events.SlotTracker.VotersUpdated.Hook(func(evt *slottracker.VoterUpdatedEvent) {
		g.refreshSlotConfirmation(evt.PrevLatestSlotIndex, evt.NewLatestSlotIndex)
	}, event.WithWorkerPool(g.workers.CreatePool("Refresh", 2)))
}

func (g *Gadget) refreshSlotConfirmation(previousLatestSlotIndex slot.Index, newLatestSlotIndex slot.Index) {
	totalWeight := g.totalWeightCallback()

	for i := lo.Max(g.LastConfirmedSlot(), previousLatestSlotIndex) + 1; i <= newLatestSlotIndex; i++ {
		if !IsThresholdReached(totalWeight, g.tangle.Booker.VirtualVoting.SlotVotersTotalWeight(i), g.optsSlotConfirmationThreshold) {
			break
		}

		// Lock here, so that SlotVotersTotalWeight is not inside the lock. Otherwise, it might cause a deadlock,
		// because one thread owns write-lock on VirtualVoting lock and needs read lock on SlotGadget lock,
		// while this method holds WriteLock on SlotGadget lock and is waiting for ReadLock on VirtualVoting.
		g.setLastConfirmedSlot(i)

		g.Events.SlotConfirmed.Trigger(i)
	}
}

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithSlotConfirmationThreshold(acceptanceThreshold float64) options.Option[Gadget] {
	return func(gadget *Gadget) {
		gadget.optsSlotConfirmationThreshold = acceptanceThreshold
	}
}

func IsThresholdReached(weight, otherWeight int64, threshold float64) bool {
	return otherWeight > int64(float64(weight)*threshold)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
