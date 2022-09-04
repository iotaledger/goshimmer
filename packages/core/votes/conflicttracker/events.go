package conflicttracker

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/core/votes"
)

type Events[ConflictIDType comparable] struct {
	VoterAdded   *event.LinkableCollectionEvent[*VoteEvent[ConflictIDType], Events[ConflictIDType], *Events[ConflictIDType]]
	VoterRemoved *event.LinkableCollectionEvent[*VoteEvent[ConflictIDType], Events[ConflictIDType], *Events[ConflictIDType]]

	event.LinkableCollection[Events[ConflictIDType], *Events[ConflictIDType]]
}

func NewEvents[ConflictIDType comparable](optLinkTargets ...*Events[ConflictIDType]) *Events[ConflictIDType] {
	return event.LinkableCollectionConstructor[Events[ConflictIDType]](func(e *Events[ConflictIDType]) {
		e.VoterAdded = event.NewLinkableCollectionEvent[*VoteEvent[ConflictIDType]](e, func(target *Events[ConflictIDType]) {
			e.VoterAdded.LinkTo(target.VoterAdded)
		})
		e.VoterRemoved = event.NewLinkableCollectionEvent[*VoteEvent[ConflictIDType]](e, func(target *Events[ConflictIDType]) {
			e.VoterRemoved.LinkTo(target.VoterRemoved)
		})
	})(optLinkTargets...)
}

type VoteEvent[ConflictIDType comparable] struct {
	Voter      *validator.Validator
	ConflictID ConflictIDType
	Opinion    votes.Opinion
}