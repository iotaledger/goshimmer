package tangle

import (
	"github.com/iotaledger/goshimmer/packages/events"
)

var Events = pluginEvents{
	TransactionStored: events.NewEvent(transactionCaller),
	TransactionSolid:  events.NewEvent(transactionCaller),
}

type pluginEvents struct {
	TransactionStored *events.Event
	TransactionSolid  *events.Event
}

func transactionCaller(handler interface{}, params ...interface{}) {
	handler.(func(*Transaction))(params[0].(*Transaction))
}
