package tangle

import (
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/hive.go/events"
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
	handler.(func(*value_transaction.ValueTransaction))(params[0].(*value_transaction.ValueTransaction))
}
