package chainmanager

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
)

type Events struct {
	CommitmentMissing         *event.Linkable[commitment.ID, Events, *Events]
	MissingCommitmentReceived *event.Linkable[commitment.ID, Events, *Events]
	ForkDetected              *event.Linkable[*Chain, Events, *Events]

	event.LinkableCollection[Events, *Events]
}

var NewEvents = event.LinkableConstructor(func() *Events {
	return &Events{
		CommitmentMissing:         event.NewLinkable[commitment.ID, Events](),
		MissingCommitmentReceived: event.NewLinkable[commitment.ID, Events](),
		ForkDetected:              event.NewLinkable[*Chain, Events](),
	}
})
