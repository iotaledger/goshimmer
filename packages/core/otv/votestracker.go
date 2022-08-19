package otv

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/conflictdag"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/validator"
)

type VotesTracker[ConflictIDType, ResourceIDType comparable] struct {
	votes *memstorage.Storage[ConflictIDType, *Votes[ConflictIDType]]

	conflictDAG  *conflictdag.ConflictDAG[ConflictIDType, ResourceIDType]
	validatorSet *validator.Set
	Events       *Events[ConflictIDType]
}

func NewVotesTracker[ConflictIDType, ResourceIDType comparable](conflictDAG *conflictdag.ConflictDAG[ConflictIDType, ResourceIDType], validatorSet *validator.Set) *VotesTracker[ConflictIDType, ResourceIDType] {
	return &VotesTracker[ConflictIDType, ResourceIDType]{
		conflictDAG:  conflictDAG,
		validatorSet: validatorSet,
		Events:       newEvents[ConflictIDType](),
	}
}

func (v *VotesTracker[ConflictIDType, ResourceIDType]) TrackVote(initialVote *set.AdvancedSet[ConflictIDType], voterID identity.ID, power VotePower) (added, invalid bool) {
	addedConflictIDs, revokedConflictIDs, invalid := v.conflictDAG.DetermineVotes(initialVote)
	if invalid {
		return false, true
	}

	voter, exists := v.validatorSet.Get(voterID)
	if !exists {
		return false, false
	}

	defaultVote := NewVote[ConflictIDType](voter, power, UndefinedOpinion)

	v.applyVotes(defaultVote.WithOpinion(Like), addedConflictIDs, v.Events.VoterAdded)
	v.applyVotes(defaultVote.WithOpinion(Dislike), revokedConflictIDs, v.Events.VoterRemoved)
	return true, false
}

func (v *VotesTracker[ConflictIDType, ResourceIDType]) applyVotes(defaultVote *Vote[ConflictIDType], conflictIDs *set.AdvancedSet[ConflictIDType], triggerEvent *event.Event[*VoterEvent[ConflictIDType]]) {
	for it := conflictIDs.Iterator(); it.HasNext(); {
		conflict := it.Next()
		votes, _ := v.votes.RetrieveOrCreate(conflict, NewVotes[ConflictIDType])

		if added, opinionChanged := votes.Add(defaultVote.WithConflictID(conflict)); added && opinionChanged {
			triggerEvent.Trigger(&VoterEvent[ConflictIDType]{Voter: defaultVote.Voter, Resource: conflict})
		}
	}
}

func (v *VotesTracker[ConflictIDType, ResourceIDType]) Voters(conflict ConflictIDType) (voters *set.AdvancedSet[*validator.Validator]) {
	votes, exists := v.votes.Get(conflict)
	if !exists {
		return set.NewAdvancedSet[*validator.Validator]()
	}

	return votes.Voters()
}
