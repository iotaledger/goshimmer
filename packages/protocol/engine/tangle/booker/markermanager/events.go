package markermanager

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
)

type Events struct {
	SequenceEvicted *event.Linkable[markers.SequenceID, Events, *Events]

	event.LinkableCollection[Events, *Events]
}

// NewEvents contains the constructor of the Events object (it is generated by a generic factory).
var NewEvents = event.LinkableCollectionConstructor[Events](func(e *Events) {
	e.SequenceEvicted = event.Link(event.NewLinkable[markers.SequenceID, Events](), e, func(target *Events) { e.SequenceEvicted.LinkTo(target.SequenceEvicted) })
})
