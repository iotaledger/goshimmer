package fpc

import (
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/fpc"
)

type fpcEvents struct {
	VotingDone *events.Event
}

func votingDoneCaller(handler interface{}, params ...interface{}) {
	handler.(func([]fpc.TxOpinion))(params[0].([]fpc.TxOpinion))
}
