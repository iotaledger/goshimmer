package udp

import (
    "github.com/iotaledger/goshimmer/packages/events"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/drop"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/ping"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/request"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/response"
    "net"
)

var Events = struct {
    ReceiveDrop     *events.Event
    ReceivePing     *events.Event
    ReceiveRequest  *events.Event
    ReceiveResponse *events.Event
    Error           *events.Event
}{
    events.NewEvent(dropCaller),
    events.NewEvent(pingCaller),
    events.NewEvent(requestCaller),
    events.NewEvent(responseCaller),
    events.NewEvent(errorCaller),
}

func dropCaller(handler interface{}, params ...interface{}) { handler.(func(*drop.Drop))(params[0].(*drop.Drop)) }
func pingCaller(handler interface{}, params ...interface{}) { handler.(func(*ping.Ping))(params[0].(*ping.Ping)) }
func requestCaller(handler interface{}, params ...interface{}) { handler.(func(*request.Request))(params[0].(*request.Request)) }
func responseCaller(handler interface{}, params ...interface{}) { handler.(func(*response.Response))(params[0].(*response.Response)) }
func errorCaller(handler interface{}, params ...interface{}) { handler.(func(net.IP, error))(params[0].(net.IP), params[1].(error)) }
