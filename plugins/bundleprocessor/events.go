package bundleprocessor

import (
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/model/bundle"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
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
	handler.(func(errors.IdentifiableError))(params[0].(errors.IdentifiableError))
}

func bundleEventCaller(handler interface{}, params ...interface{}) {
	handler.(func(*bundle.Bundle, []*value_transaction.ValueTransaction))(params[0].(*bundle.Bundle), params[1].([]*value_transaction.ValueTransaction))
}
