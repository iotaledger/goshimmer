package bundleprocessor

import (
	"github.com/iotaledger/goshimmer/packages/model/bundle"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/hive.go/events"
)

var Events = pluginEvents{
	Error:         events.NewEvent(errorCaller),
	BundleSolid:   events.NewEvent(bundleEventCaller),
	InvalidBundle: events.NewEvent(bundleEventCaller),
}

type pluginEvents struct {
	Error         *events.Event
	BundleSolid   *events.Event
	InvalidBundle *events.Event
}

func errorCaller(handler interface{}, params ...interface{}) {
	handler.(func(error))(params[0].(error))
}

func bundleEventCaller(handler interface{}, params ...interface{}) {
	handler.(func(*bundle.Bundle, []*value_transaction.ValueTransaction))(params[0].(*bundle.Bundle), params[1].([]*value_transaction.ValueTransaction))
}
