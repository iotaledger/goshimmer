package node

import (
    "github.com/iotaledger/goshimmer/packages/events"
)

type pluginEvents struct {
    Configure *events.Event
    Run       *events.Event
}

func pluginCaller(handler interface{}, params ...interface{}) { handler.(func(*Plugin))(params[0].(*Plugin)) }