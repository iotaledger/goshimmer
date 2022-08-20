package votes

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/conflictdag"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/validator"
)

type ConflictTracker[ConflictIDType, ResourceIDType comparable, VotePowerType VotePower[VotePowerType]] struct {
	votes *memstorage.Storage[ConflictIDType, *Votes[ConflictIDType, VotePowerType]]

	conflictDAG  *conflictdag.ConflictDAG[ConflictIDType, ResourceIDType]
	validatorSet *validator.Set
	Events       *ConflictTrackerEvents[ConflictIDType]
}

func NewConflictTracker[ConflictIDType, ResourceIDType comparable, VotePowerType VotePower[VotePowerType]](conflictDAG *conflictdag.ConflictDAG[ConflictIDType, ResourceIDType], validatorSet *validator.Set) *ConflictTracker[ConflictIDType, ResourceIDType, VotePowerType] {
	return &ConflictTracker[ConflictIDType, ResourceIDType, VotePowerType]{
		votes:        memstorage.New[ConflictIDType, *Votes[ConflictIDType, VotePowerType]](),
		conflictDAG:  conflictDAG,
		validatorSet: validatorSet,
		Events:       newConflictTrackerEvents[ConflictIDType](),
	}
}

func (v *ConflictTracker[ConflictIDType, ResourceIDType, VotePowerType]) TrackVote(initialVote *set.AdvancedSet[ConflictIDType], voterID identity.ID, power VotePowerType) (added, invalid bool) {
	addedConflictIDs, revokedConflictIDs, invalid := v.conflictDAG.DetermineVotes(initialVote)
	if invalid {
		return false, true
	}

	voter, exists := v.validatorSet.Get(voterID)
	if !exists {
		return false, false
	}

	defaultVote := NewVote[ConflictIDType, VotePowerType](voter, power, UndefinedOpinion)

	v.applyVotes(defaultVote.WithOpinion(Like), addedConflictIDs, v.Events.VoterAdded)
	v.applyVotes(defaultVote.WithOpinion(Dislike), revokedConflictIDs, v.Events.VoterRemoved)
	return true, false
}

func (v *ConflictTracker[ConflictIDType, ResourceIDType, VotePowerType]) applyVotes(defaultVote *Vote[ConflictIDType, VotePowerType], conflictIDs *set.AdvancedSet[ConflictIDType], triggerEvent *event.Event[*ConflictVoterEvent[ConflictIDType]]) {
	for it := conflictIDs.Iterator(); it.HasNext(); {
		conflict := it.Next()
		votes, _ := v.votes.RetrieveOrCreate(conflict, NewVotes[ConflictIDType, VotePowerType])

		if added, opinionChanged := votes.Add(defaultVote.WithConflictID(conflict)); added && opinionChanged {
			triggerEvent.Trigger(&ConflictVoterEvent[ConflictIDType]{Voter: defaultVote.Voter, ConflictID: conflict})
		}
	}
}

func (v *ConflictTracker[ConflictIDType, ResourceIDType, VotePowerType]) Voters(conflict ConflictIDType) (voters *set.AdvancedSet[*validator.Validator]) {
	votes, exists := v.votes.Get(conflict)
	if !exists {
		return set.NewAdvancedSet[*validator.Validator]()
	}

	return votes.Voters()
}
