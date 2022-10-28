package epochconfirmation

import (
	"fmt"
	"sync"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/core/votes/epochtracker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
)

type Gadget struct {
	Events *Events

	tangle              *tangle.Tangle
	lastConfirmedEpoch  epoch.Index
	totalWeightCallback func() (int64, error)

	optsEpochConfirmationThreshold float64

	sync.RWMutex
}

func New(tangle *tangle.Tangle, lastConfirmedEpoch epoch.Index, totalWeightCallback func() (int64, error), opts ...options.Option[Gadget]) (gadget *Gadget) {
	return options.Apply(&Gadget{
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

func (g *Gadget) setup() {
	g.tangle.VirtualVoting.Events.EpochTracker.VotersUpdated.Attach(event.NewClosure[*epochtracker.VoterUpdatedEvent](func(evt *epochtracker.VoterUpdatedEvent) {
		g.refreshEpochConfirmation(evt.PrevLatestEpochIndex, evt.NewLatestEpochIndex)
	}))
}

func (g *Gadget) refreshEpochConfirmation(previousLatestEpochIndex epoch.Index, newLatestEpochIndex epoch.Index) {
	g.Lock()
	defer g.Unlock()
	totalWeight, err := g.totalWeightCallback()
	if err != nil {
		panic(err)
	}

	for i := lo.Max(g.lastConfirmedEpoch+1, previousLatestEpochIndex); i <= newLatestEpochIndex; i++ {
		if validator.IsThresholdReached(totalWeight, g.tangle.VirtualVoting.EpochVoters(i).TotalWeight(), g.optsEpochConfirmationThreshold) {
			g.lastConfirmedEpoch = i
			g.Events.EpochConfirmed.Trigger(i)
			fmt.Println(">> EpochConfirmed: ", i)
		}
	}
}

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithEpochConfirmationThreshold(acceptanceThreshold float64) options.Option[Gadget] {
	return func(gadget *Gadget) {
		gadget.optsEpochConfirmationThreshold = acceptanceThreshold
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
