package chainmanager

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/eviction"
)

type Events struct {
	CommitmentMissing         *event.Linkable[commitment.ID]
	MissingCommitmentReceived *event.Linkable[commitment.ID]
	ForkDetected              *event.Linkable[*Fork]
	EvictionState             *eviction.Events

	event.LinkableCollection[Events, *Events]
}

var NewEvents = event.LinkableConstructor(func() *Events {
	return &Events{
		CommitmentMissing:         event.NewLinkable[commitment.ID](),
		MissingCommitmentReceived: event.NewLinkable[commitment.ID](),
		ForkDetected:              event.NewLinkable[*Fork](),
	}
})
