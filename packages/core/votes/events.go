package votes

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/core/markers"
	"github.com/iotaledger/goshimmer/packages/core/validator"
)

type ConflictTrackerEvents[ConflictIDType comparable] struct {
	VoterAdded   *event.Event[*ConflictVoterEvent[ConflictIDType]]
	VoterRemoved *event.Event[*ConflictVoterEvent[ConflictIDType]]
}

type ConflictVoterEvent[ConflictIDType comparable] struct {
	Voter      *validator.Validator
	ConflictID ConflictIDType
	Opinion    Opinion
}

func newConflictTrackerEvents[ConflictIDType comparable]() *ConflictTrackerEvents[ConflictIDType] {
	return &ConflictTrackerEvents[ConflictIDType]{
		VoterAdded:   event.New[*ConflictVoterEvent[ConflictIDType]](),
		VoterRemoved: event.New[*ConflictVoterEvent[ConflictIDType]](),
	}
}

type SequenceTrackerEvents struct {
	SequenceVotersUpdated *event.Event[*SequenceVotersUpdatedEvent]
}

type SequenceVotersUpdatedEvent struct {
	NewMaxSupportedIndex  markers.Index
	PrevMaxSupportedIndex markers.Index
	SequenceID            markers.SequenceID
}

func newSequenceTrackerEvents() *SequenceTrackerEvents {
	return &SequenceTrackerEvents{
		SequenceVotersUpdated: event.New[*SequenceVotersUpdatedEvent](),
	}
}
