package epochgadget

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/workerpool"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/votes/epochtracker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
)

type Gadget struct {
	Events *Events

	tangle              *tangle.Tangle
	lastConfirmedEpoch  epoch.Index
	totalWeightCallback func() int64

	optsEpochConfirmationThreshold float64

	workers *workerpool.Group

	sync.RWMutex
}

func New(workers *workerpool.Group, tangle *tangle.Tangle, lastConfirmedEpoch epoch.Index, totalWeightCallback func() int64, opts ...options.Option[Gadget]) (gadget *Gadget) {
	return options.Apply(&Gadget{
		workers:                        workers,
		optsEpochConfirmationThreshold: 0.67,
	}, opts, func(a *Gadget) {
		a.Events = NewEvents()

		a.tangle = tangle
		a.lastConfirmedEpoch = lastConfirmedEpoch
		a.totalWeightCallback = totalWeightCallback
	}, (*Gadget).setup)
}

func (g *Gadget) LastConfirmedEpoch() epoch.Index {
	g.RLock()
	defer g.RUnlock()

	return g.lastConfirmedEpoch
}

func (g *Gadget) setLastConfirmedEpoch(i epoch.Index) {
	g.Lock()
	defer g.Unlock()

	g.lastConfirmedEpoch = i
}

func (g *Gadget) setup() {
	event.AttachWithWorkerPool(g.tangle.VirtualVoting.Events.EpochTracker.VotersUpdated, func(evt *epochtracker.VoterUpdatedEvent) {
		g.refreshEpochConfirmation(evt.PrevLatestEpochIndex, evt.NewLatestEpochIndex)
	}, g.workers.CreatePool("Refresh"))
}

func (g *Gadget) refreshEpochConfirmation(previousLatestEpochIndex epoch.Index, newLatestEpochIndex epoch.Index) {
	totalWeight := g.totalWeightCallback()

	for i := lo.Max(g.LastConfirmedEpoch(), previousLatestEpochIndex) + 1; i <= newLatestEpochIndex; i++ {
		if !IsThresholdReached(totalWeight, g.tangle.VirtualVoting.EpochVotersTotalWeight(i), g.optsEpochConfirmationThreshold) {
			break
		}

		// Lock here, so that EpochVotersTotalWeight is not inside the lock. Otherwise, it might cause a deadlock,
		// because one thread owns write-lock on VirtualVoting lock and needs read lock on EpochGadget lock,
		// while this method holds WriteLock on EpochGadget lock and is waiting for ReadLock on VirtualVoting.
		g.setLastConfirmedEpoch(i)

		g.Events.EpochConfirmed.Trigger(i)
	}
}

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithEpochConfirmationThreshold(acceptanceThreshold float64) options.Option[Gadget] {
	return func(gadget *Gadget) {
		gadget.optsEpochConfirmationThreshold = acceptanceThreshold
	}
}

func IsThresholdReached(weight, otherWeight int64, threshold float64) bool {
	return otherWeight > int64(float64(weight)*threshold)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
