package udp

import (
	"net"

	"github.com/iotaledger/hive.go/events"
)

type serverEvents struct {
	Start       *events.Event
	Shutdown    *events.Event
	ReceiveData *events.Event
	Error       *events.Event
}

func dataCaller(handler interface{}, params ...interface{}) {
	handler.(func(*net.UDPAddr, []byte))(params[0].(*net.UDPAddr), params[1].([]byte))
}
