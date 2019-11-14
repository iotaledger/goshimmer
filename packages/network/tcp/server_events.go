package tcp

import (
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/hive.go/events"
)

type serverEvents struct {
	Start    *events.Event
	Shutdown *events.Event
	Connect  *events.Event
	Error    *events.Event
}

func managedConnectionCaller(handler interface{}, params ...interface{}) {
	handler.(func(*network.ManagedConnection))(params[0].(*network.ManagedConnection))
}
