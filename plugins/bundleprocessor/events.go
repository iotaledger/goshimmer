package bundleprocessor

import (
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/model/bundle"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
)

var Events = pluginEvents{
	BundleSolid:   events.NewEvent(bundleEventCaller),
	InvalidBundle: events.NewEvent(bundleEventCaller),
}

type pluginEvents struct {
	BundleSolid   *events.Event
	InvalidBundle *events.Event
}

func bundleEventCaller(handler interface{}, params ...interface{}) {
	handler.(func(*bundle.Bundle, []*value_transaction.ValueTransaction))(params[0].(*bundle.Bundle), params[1].([]*value_transaction.ValueTransaction))
}
