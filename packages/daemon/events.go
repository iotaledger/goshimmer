package daemon

import (
	"github.com/iotaledger/hive.go/events"
)

var Events = struct {
	Run      *events.Event
	Shutdown *events.Event
}{
	Run:      events.NewEvent(events.CallbackCaller),
	Shutdown: events.NewEvent(events.CallbackCaller),
}
