package bundleprocessor

import (
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/model/bundle"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
)

var Events = pluginEvents{
	DataBundleReceived:    events.NewEvent(bundleEventCaller),
	ValueBundleReceived:   events.NewEvent(bundleEventCaller),
	InvalidBundleReceived: events.NewEvent(bundleEventCaller),
}

type pluginEvents struct {
	DataBundleReceived    *events.Event
	ValueBundleReceived   *events.Event
	InvalidBundleReceived *events.Event
}

func bundleEventCaller(handler interface{}, params ...interface{}) {
	handler.(func(*bundle.Bundle, []*value_transaction.ValueTransaction))(params[0].(*bundle.Bundle), params[1].([]*value_transaction.ValueTransaction))
}
