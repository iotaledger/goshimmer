package fpc

import (
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/fpc"
)

type fpcEvents struct {
	NewFinalizedTxs *events.Event
}

func newFinalizedTxsCaller(handler interface{}, params ...interface{}) {
	handler.(func([]fpc.TxOpinion))(params[0].([]fpc.TxOpinion))
}
