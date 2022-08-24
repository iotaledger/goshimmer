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
	Voter                  *validator.Validator
	NewMaxSupportedMarker  markers.Marker
	PrevMaxSupportedMarker markers.Marker
}

func newSequenceTrackerEvents() *SequenceTrackerEvents {
	return &SequenceTrackerEvents{
		VoterAdded: event.New[*SequenceVoterEvent](),
	}
}
