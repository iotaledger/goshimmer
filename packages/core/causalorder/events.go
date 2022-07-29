package causalorder

import (
	"github.com/iotaledger/hive.go/generics/event"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type Events[ID epoch.IndexedID, Entity OrderedEntity[ID]] struct {
	Emit *event.Event[Entity]
	Drop *event.Event[Entity]
}

func newEvents[ID epoch.IndexedID, Entity OrderedEntity[ID]]() *Events[ID, Entity] {
	return &Events[ID, Entity]{
		Emit: event.New[Entity](),
		Drop: event.New[Entity](),
	}
}
