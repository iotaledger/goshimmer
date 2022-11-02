package chainmanager

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
)

type Events struct {
	CommitmentMissing         *event.Linkable[commitment.ID]
	MissingCommitmentReceived *event.Linkable[commitment.ID]
	ForkDetected              *event.Linkable[*Chain]

	event.LinkableCollection[Events, *Events]
}

var NewEvents = event.LinkableConstructor(func() *Events {
	return &Events{
		CommitmentMissing:         event.NewLinkable[commitment.ID](),
		MissingCommitmentReceived: event.NewLinkable[commitment.ID](),
		ForkDetected:              event.NewLinkable[*Chain](),
	}
})
