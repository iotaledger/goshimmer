package conflicttracker

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/core/votes"
)

type Events[ConflictIDType comparable] struct {
	VoterAdded   *event.Linkable[*VoterEvent[ConflictIDType]]
	VoterRemoved *event.Linkable[*VoterEvent[ConflictIDType]]

	event.LinkableCollection[Events[ConflictIDType], *Events[ConflictIDType]]
}

func NewEvents[ConflictIDType comparable](optLinkTargets ...*Events[ConflictIDType]) *Events[ConflictIDType] {
	return event.LinkableConstructor(func() (self *Events[ConflictIDType]) {
		return &Events[ConflictIDType]{
			VoterAdded:   event.NewLinkable[*VoterEvent[ConflictIDType]](),
			VoterRemoved: event.NewLinkable[*VoterEvent[ConflictIDType]](),
		}
	})(optLinkTargets...)
}

type VoterEvent[ConflictIDType comparable] struct {
	Voter      *validator.Validator
	ConflictID ConflictIDType
	Opinion    votes.Opinion
}
