package tipselector

import (
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction"
	"github.com/iotaledger/hive.go/events"
)

type Events struct {
	TipAdded   *events.Event
	TipRemoved *events.Event
}

func transactionIdEvent(handler interface{}, params ...interface{}) {
	handler.(func(transaction.Id))(params[0].(transaction.Id))
}
