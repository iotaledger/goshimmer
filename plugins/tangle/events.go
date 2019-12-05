package tangle

import (
	"github.com/iotaledger/goshimmer/packages/model/meta_transaction"
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
	handler.(func(*meta_transaction.MetaTransaction))(params[0].(*meta_transaction.MetaTransaction))
}
