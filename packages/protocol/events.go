package protocol

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/protocol/instancemanager"
)

type Events struct {
	InstanceManager *instancemanager.Events

	event.LinkableCollection[Events, *Events]
}

var NewEvents = event.LinkableConstructor(func() (newEvents *Events) {
	return &Events{
		InstanceManager: instancemanager.NewEvents(),
	}
})
