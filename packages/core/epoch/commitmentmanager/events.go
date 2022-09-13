package commitmentmanager

import (
	"github.com/iotaledger/hive.go/core/generics/event"
)

type Events struct {
	ForkDetected *event.Linkable[*Chain, Events, *Events]

	event.LinkableCollection[Events, *Events]
}

var NewEvents = event.LinkableConstructor(func() *Events {
	return &Events{
		ForkDetected: event.NewLinkable[*Chain, Events](),
	}
})
