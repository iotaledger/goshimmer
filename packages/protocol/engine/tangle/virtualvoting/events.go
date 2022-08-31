package virtualvoting

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/core/votes"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
)

type Events struct {
	BlockTracked         *event.Event[*Block]
	ConflictVoterAdded   *event.Event[*votes.ConflictVoterEvent[utxo.TransactionID]]
	ConflictVoterRemoved *event.Event[*votes.ConflictVoterEvent[utxo.TransactionID]]
	SequenceVoterAdded   *event.Event[*votes.SequenceVoterEvent]
}

// newEvents creates a new Events instance.
func newEvents(conflictTrackerEvents *votes.ConflictTrackerEvents[utxo.TransactionID], sequenceTrackerEvents *votes.SequenceTrackerEvents) *Events {
	return &Events{
		BlockTracked:         event.New[*Block](),
		ConflictVoterAdded:   conflictTrackerEvents.VoterAdded,
		ConflictVoterRemoved: conflictTrackerEvents.VoterRemoved,
		SequenceVoterAdded:   sequenceTrackerEvents.VoterAdded,
	}
}
