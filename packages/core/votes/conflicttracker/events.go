package conflicttracker

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/core/votes"
)

type Events[ConflictIDType comparable] struct {
	VoterAdded   *event.Linkable[*VoterEvent[ConflictIDType], Events[ConflictIDType], *Events[ConflictIDType]]
	VoterRemoved *event.Linkable[*VoterEvent[ConflictIDType], Events[ConflictIDType], *Events[ConflictIDType]]

	event.LinkableCollection[Events[ConflictIDType], *Events[ConflictIDType]]
}

func NewEvents[ConflictIDType comparable](optLinkTargets ...*Events[ConflictIDType]) *Events[ConflictIDType] {
	return event.NewLinkableEvents(func(self *Events[ConflictIDType]) (linker func(*Events[ConflictIDType])) {
		self.VoterAdded = event.NewLinkable[*VoterEvent[ConflictIDType], Events[ConflictIDType], *Events[ConflictIDType]]()
		self.VoterRemoved = event.NewLinkable[*VoterEvent[ConflictIDType], Events[ConflictIDType], *Events[ConflictIDType]]()

		return func(newTarget *Events[ConflictIDType]) {
			self.VoterAdded.LinkTo(newTarget.VoterAdded)
			self.VoterRemoved.LinkTo(newTarget.VoterRemoved)
		}
	})(optLinkTargets...)
}

type VoterEvent[ConflictIDType comparable] struct {
	Voter      *validator.Validator
	ConflictID ConflictIDType
	Opinion    votes.Opinion
}
