package tipselector

import (
	"github.com/iotaledger/hive.go/events"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
)

type Events struct {
	TipAdded   *events.Event
	TipRemoved *events.Event
}

func transactionIdEvent(handler interface{}, params ...interface{}) {
	handler.(func(message.Id))(params[0].(message.Id))
}
