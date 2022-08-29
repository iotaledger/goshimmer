package votes

import (
	"fmt"

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

func (c *ConflictTracker[ConflictIDType, ResourceIDType, VotePowerType]) TrackVote(initialVote *set.AdvancedSet[ConflictIDType], voterID identity.ID, power VotePowerType) (added, invalid bool) {
	addedConflictIDs, revokedConflictIDs, invalid := c.conflictDAG.DetermineVotes(initialVote)
	if invalid {
		return false, true
	}

	voter, exists := c.validatorSet.Get(voterID)
	if !exists {
		return false, false
	}

	defaultVote := NewVote[ConflictIDType, VotePowerType](voter, power, UndefinedOpinion)

	eventsToTrigger := c.applyVotes(defaultVote.WithOpinion(Dislike), revokedConflictIDs)
	eventsToTrigger = append(eventsToTrigger, c.applyVotes(defaultVote.WithOpinion(Like), addedConflictIDs)...)

	c.triggerEvents(eventsToTrigger)

	return true, false
}

func (c *ConflictTracker[ConflictIDType, ResourceIDType, VotePowerType]) Voters(conflict ConflictIDType) (voters *set.AdvancedSet[*validator.Validator]) {
	votes, exists := c.votes.Get(conflict)
	if !exists {
		return set.NewAdvancedSet[*validator.Validator]()
	}

	return votes.Voters()
}

func (c *ConflictTracker[ConflictIDType, ResourceIDType, VotePowerType]) AddSupportToForkedConflict(forkedConflictID ConflictIDType, parentConflictIDs *set.AdvancedSet[ConflictIDType], voterID identity.ID, power VotePowerType) {
	voter, exists := c.validatorSet.Get(voterID)
	if !exists {
		return
	}

	// We need to make sure that the voter supports all the conflict's parents.
	if !c.voterSupportsAllConflicts(voter, parentConflictIDs) {
		return
	}

	vote := NewVote[ConflictIDType, VotePowerType](voter, power, Like).WithConflictID(forkedConflictID)

	votes, _ := c.votes.RetrieveOrCreate(forkedConflictID, NewVotes[ConflictIDType, VotePowerType])
	if added, opinionChanged := votes.Add(vote); added && opinionChanged {
		c.Events.VoterAdded.Trigger(&ConflictVoterEvent[ConflictIDType]{Voter: voter, ConflictID: forkedConflictID})
	}

	return
}

func (c *ConflictTracker[ConflictIDType, ResourceIDType, VotePowerType]) applyVotes(defaultVote *Vote[ConflictIDType, VotePowerType], conflictIDs *set.AdvancedSet[ConflictIDType]) (collectedEvents []*ConflictVoterEvent[ConflictIDType]) {
	for it := conflictIDs.Iterator(); it.HasNext(); {
		conflict := it.Next()
		votes, created := c.votes.RetrieveOrCreate(conflict, NewVotes[ConflictIDType, VotePowerType])

		conflictVote := defaultVote.WithConflictID(conflict)

		// Only handle Like opinion because dislike should always be created and exist before.
		if created && conflictVote.Opinion == Like {
			if votePower, dislikeInstead := c.revokeConflictInstead(conflict, defaultVote); dislikeInstead {
				conflictVote = conflictVote.WithOpinion(Dislike).WithVotePower(votePower)
			}
		}

		if added, opinionChanged := votes.Add(conflictVote); added && opinionChanged {
			collectedEvents = append(collectedEvents, &ConflictVoterEvent[ConflictIDType]{Voter: conflictVote.Voter, ConflictID: conflict, Opinion: conflictVote.Opinion})
		}
	}
	return collectedEvents
}

func (c *ConflictTracker[ConflictIDType, ResourceIDType, VotePowerType]) voterSupportsAllConflicts(voter *validator.Validator, conflictIDs *set.AdvancedSet[ConflictIDType]) (allConflictsSupported bool) {
	for it := conflictIDs.Iterator(); it.HasNext(); {
		conflictID := it.Next()

		if !c.voterSupportsConflict(voter, conflictID) {
			return false
		}
	}

	return true
}

func (c *ConflictTracker[ConflictIDType, ResourceIDType, VotePowerType]) voterSupportsConflict(voter *validator.Validator, conflictID ConflictIDType) bool {
	votes, exists := c.votes.Get(conflictID)
	if !exists {
		panic(fmt.Sprintf("votes for conflict %v not found", conflictID))
	}

	vote, exists := votes.Vote(voter)
	if !exists {
		return false
	}
	return vote.Opinion == Like
}

func (c *ConflictTracker[ConflictIDType, ResourceIDType, VotePowerType]) revokeConflictInstead(conflictID ConflictIDType, vote *Vote[ConflictIDType, VotePowerType]) (votePower VotePowerType, revokeInstead bool) {
	c.conflictDAG.Utils.ForEachConflictingConflictID(conflictID, func(conflictingConflictID ConflictIDType) bool {
		votes, conflictVotesExist := c.votes.Get(conflictingConflictID)
		if !conflictVotesExist {
			revokeInstead = false
			return false
		}

		existingVote, voteExists := votes.Vote(vote.Voter)
		if !voteExists {
			revokeInstead = false
			return false
		}

		if existingVote.VotePower.CompareTo(vote.VotePower) >= 0 {
			revokeInstead = true
			votePower = existingVote.VotePower
			return false
		}

		return true
	})

	return votePower, revokeInstead
}

func (c *ConflictTracker[ConflictIDType, ResourceIDType, VotePowerType]) triggerEvents(eventsToTrigger []*ConflictVoterEvent[ConflictIDType]) {
	for _, event := range eventsToTrigger {
		switch event.Opinion {
		case Like:
			c.Events.VoterAdded.Trigger(event)
		case Dislike:
			c.Events.VoterRemoved.Trigger(event)
		}
	}
}
