package otv

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/validator"
)

type Events struct {
	VoterAdded   *event.Event[*VoterEvent]
	VoterRemoved *event.Event[*VoterEvent]
}

type VoterEvent struct {
	Voter    *validator.Validator
	Resource utxo.TransactionID
}

func newEvents() *Events {
	return &Events{
		VoterAdded:   event.New[*VoterEvent](),
		VoterRemoved: event.New[*VoterEvent](),
	}
}
