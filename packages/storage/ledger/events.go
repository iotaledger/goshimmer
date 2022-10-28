package ledger

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/identity"
)

type Events struct {
	ConsensusWeightsUpdated *event.Linkable[map[identity.ID]*TimedBalance]

	event.LinkableCollection[Events, *Events]
}

// NewEvents contains the constructor of the Events object (it is generated by a generic factory).
var NewEvents = event.LinkableConstructor(func() (self *Events) {
	return &Events{
		ConsensusWeightsUpdated: event.NewLinkable[map[identity.ID]*TimedBalance](),
	}
})
