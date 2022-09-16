package sequencetracker

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/protocol/chain/engine/tangle/booker/markers"
)

type Events struct {
	VotersUpdated *event.Linkable[*VoterUpdatedEvent, Events, *Events]

	event.LinkableCollection[Events, *Events]
}

// NewEvents contains the constructor of the Events object (it is generated by a generic factory).
var NewEvents = event.LinkableConstructor(func() (newEvents *Events) {
	return &Events{
		VotersUpdated: event.NewLinkable[*VoterUpdatedEvent, Events, *Events](),
	}
})

type VoterUpdatedEvent struct {
	Voter                 *validator.Validator
	NewMaxSupportedIndex  markers.Index
	PrevMaxSupportedIndex markers.Index
	SequenceID            markers.SequenceID
}
