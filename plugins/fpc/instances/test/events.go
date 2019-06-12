package test

import (
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/fpc"
)

var FinalizedOpinions *events.Event

func finalizedOpinionsCaller(handler interface{}, params ...interface{}) {
	handler.(func([]fpc.Opinion))(params[0].([]fpc.Opinion))
}
