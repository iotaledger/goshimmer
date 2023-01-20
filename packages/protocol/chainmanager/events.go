package chainmanager

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/eviction"
)

type Events struct {
	CommitmentMissing         *event.Linkable[commitment.ID]
	MissingCommitmentReceived *event.Linkable[commitment.ID]
	ForkDetected              *event.Linkable[*ForkDetectedEvent]
	EvictionState             *eviction.Events

	event.LinkableCollection[Events, *Events]
}

var NewEvents = event.LinkableConstructor(func() *Events {
	return &Events{
		CommitmentMissing:         event.NewLinkable[commitment.ID](),
		MissingCommitmentReceived: event.NewLinkable[commitment.ID](),
		ForkDetected:              event.NewLinkable[*ForkDetectedEvent](),
	}
})

type ForkDetectedEvent struct {
	Source     identity.ID
	Commitment *commitment.Commitment
	Chain      *Chain
}

func (e *ForkDetectedEvent) StartEpoch() epoch.Index {
	return e.Chain.ForkingPoint.ID().Index()
}

func (e *ForkDetectedEvent) EndEpoch() epoch.Index {
	return e.Commitment.Index()
}

func (e *ForkDetectedEvent) EpochCount() int64 {
	return int64(e.EndEpoch() - e.StartEpoch())
}
