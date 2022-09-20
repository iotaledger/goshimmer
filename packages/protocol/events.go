package protocol

import (
	"github.com/iotaledger/hive.go/core/generics/event"
)

type Events struct {
	event.LinkableCollection[Events, *Events]
}

var NewEvents = event.LinkableConstructor(func() (newEvents *Events) {
	return &Events{}
})
