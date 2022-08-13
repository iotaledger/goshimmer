package causalorder

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type Events[ID epoch.IndexedID, Entity OrderedEntity[ID]] struct {
	Drop *event.Event[Entity]
}

func newEvents[ID epoch.IndexedID, Entity OrderedEntity[ID]]() *Events[ID, Entity] {
	return &Events[ID, Entity]{
		Drop: event.New[Entity](),
	}
}
