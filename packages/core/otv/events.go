package otv

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/core/validator"
)

type Events[ConflictIDType comparable] struct {
	VoterAdded   *event.Event[*VoterEvent[ConflictIDType]]
	VoterRemoved *event.Event[*VoterEvent[ConflictIDType]]
}

type VoterEvent[ConflictIDType comparable] struct {
	Voter    *validator.Validator
	Resource ConflictIDType
}

func newEvents[ConflictIDType comparable]() *Events[ConflictIDType] {
	return &Events[ConflictIDType]{
		VoterAdded:   event.New[*VoterEvent[ConflictIDType]](),
		VoterRemoved: event.New[*VoterEvent[ConflictIDType]](),
	}
}
