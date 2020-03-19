package drng

import (
	"github.com/iotaledger/hive.go/events"
)

var Events = struct {
	NewRandomness *events.Event
}{
	NewRandomness: events.NewEvent(transactionCaller),
}

func transactionCaller(handler interface{}, params ...interface{}) {
	//handler.(func(*value_transaction.ValueTransaction))(params[0].(*value_transaction.ValueTransaction))
}
