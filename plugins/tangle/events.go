package tangle

import (
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/hive.go/events"
)

var Events = struct {
	TransactionStored *events.Event
	TransactionSolid  *events.Event
}{
	TransactionStored: events.NewEvent(transactionCaller),
	TransactionSolid:  events.NewEvent(transactionCaller),
}

func transactionCaller(handler interface{}, params ...interface{}) {
	handler.(func(*value_transaction.ValueTransaction))(params[0].(*value_transaction.ValueTransaction))
}
