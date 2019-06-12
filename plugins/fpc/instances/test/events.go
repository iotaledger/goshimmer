package test

import (
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/fpc"
)

var FinalizedOpinions *events.Event

func finalizedOpinionsCaller(handler interface{}, params ...interface{}) {
	handler.(func([]fpc.TxOpinion))(params[0].([]fpc.TxOpinion))
}
