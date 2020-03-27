package tipselector

import (
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/model/message"
	"github.com/iotaledger/hive.go/events"
)

type Events struct {
	TipAdded   *events.Event
	TipRemoved *events.Event
}

func transactionIdEvent(handler interface{}, params ...interface{}) {
	handler.(func(message.Id))(params[0].(message.Id))
}
