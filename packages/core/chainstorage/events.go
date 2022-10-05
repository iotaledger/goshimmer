package chainstorage

import (
	"github.com/iotaledger/hive.go/core/generics/event"
)

type Events struct {
	Error event.Linkable[error, Events, *Events]

	event.LinkableCollection[Events, *Events]
}
