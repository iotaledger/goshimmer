package booker

import (
	"github.com/iotaledger/hive.go/generics/event"
)

type Events struct {
	BlockBooked *event.Event[*Block]
}
