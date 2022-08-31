package votes

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
)

type ConflictTrackerEvents[ConflictIDType comparable] struct {
	VoterAdded   *event.Event[*ConflictVoterEvent[ConflictIDType]]
	VoterRemoved *event.Event[*ConflictVoterEvent[ConflictIDType]]
}

type ConflictVoterEvent[ConflictIDType comparable] struct {
	Voter      *validator.Validator
	ConflictID ConflictIDType
}

func newConflictTrackerEvents[ConflictIDType comparable]() *ConflictTrackerEvents[ConflictIDType] {
	return &ConflictTrackerEvents[ConflictIDType]{
		VoterAdded:   event.New[*ConflictVoterEvent[ConflictIDType]](),
		VoterRemoved: event.New[*ConflictVoterEvent[ConflictIDType]](),
	}
}

type SequenceTrackerEvents struct {
	VoterAdded *event.Event[*SequenceVoterEvent]
}

type SequenceVoterEvent struct {
	Voter  *validator.Validator
	Marker markers.Marker
}

func newSequenceTrackerEvents() *SequenceTrackerEvents {
	return &SequenceTrackerEvents{
		VoterAdded: event.New[*SequenceVoterEvent](),
	}
}
