package network

import (
	"github.com/iotaledger/hive.go/events"
)

type BufferedConnectionEvents struct {
	ReceiveData *events.Event
	Close       *events.Event
	Error       *events.Event
}

func dataCaller(handler interface{}, params ...interface{}) {
	handler.(func([]byte))(params[0].([]byte))
}
