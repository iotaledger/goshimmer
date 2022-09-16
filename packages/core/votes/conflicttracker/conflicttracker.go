package conflicttracker

import (
	"fmt"

	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/core/votes"
	"github.com/iotaledger/goshimmer/packages/protocol/chain/ledger/conflictdag"
)

type ConflictTracker[ConflictIDType, ResourceIDType comparable, VotePowerType votes.VotePower[VotePowerType]] struct {
	votes *memstorage.Storage[ConflictIDType, *votes.Votes[ConflictIDType, VotePowerType]]

	conflictDAG  *conflictdag.ConflictDAG[ConflictIDType, ResourceIDType]
	validatorSet *validator.Set
	Events       *Events[ConflictIDType]
}

func NewConflictTracker[ConflictIDType, ResourceIDType comparable, VotePowerType votes.VotePower[VotePowerType]](conflictDAG *conflictdag.ConflictDAG[ConflictIDType, ResourceIDType], validatorSet *validator.Set) *ConflictTracker[ConflictIDType, ResourceIDType, VotePowerType] {
	return &ConflictTracker[ConflictIDType, ResourceIDType, VotePowerType]{
		votes:        memstorage.New[ConflictIDType, *votes.Votes[ConflictIDType, VotePowerType]](),
		conflictDAG:  conflictDAG,
		validatorSet: validatorSet,
		Events:       NewEvents[ConflictIDType](),
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

	defaultVote := votes.NewVote[ConflictIDType, VotePowerType](voter, power, votes.UndefinedOpinion)

	eventsToTrigger := c.applyVotes(defaultVote.WithOpinion(votes.Dislike), revokedConflictIDs)
	eventsToTrigger = append(eventsToTrigger, c.applyVotes(defaultVote.WithOpinion(votes.Like), addedConflictIDs)...)

	c.triggerEvents(eventsToTrigger)

	return true, false
}

func (c *ConflictTracker[ConflictIDType, ResourceIDType, VotePowerType]) Voters(conflict ConflictIDType) (voters *validator.Set) {
	votesObj, exists := c.votes.Get(conflict)
	if !exists {
		return validator.NewSet()
	}

	return votesObj.Voters()
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

	vote := votes.NewVote[ConflictIDType, VotePowerType](voter, power, votes.Like).WithConflictID(forkedConflictID)

	votesObj, _ := c.votes.RetrieveOrCreate(forkedConflictID, votes.NewVotes[ConflictIDType, VotePowerType])
	if added, opinionChanged := votesObj.Add(vote); added && opinionChanged {
		c.Events.VoterAdded.Trigger(&VoterEvent[ConflictIDType]{Voter: voter, ConflictID: forkedConflictID})
	}

	return
}

func (c *ConflictTracker[ConflictIDType, ResourceIDType, VotePowerType]) applyVotes(defaultVote *votes.Vote[ConflictIDType, VotePowerType], conflictIDs *set.AdvancedSet[ConflictIDType]) (collectedEvents []*VoterEvent[ConflictIDType]) {
	for it := conflictIDs.Iterator(); it.HasNext(); {
		conflict := it.Next()
		votesObj, created := c.votes.RetrieveOrCreate(conflict, votes.NewVotes[ConflictIDType, VotePowerType])

		conflictVote := defaultVote.WithConflictID(conflict)

		// Only handle Like opinion because dislike should always be created and exist before.
		if created && conflictVote.Opinion == votes.Like {
			if votePower, dislikeInstead := c.revokeConflictInstead(conflict, conflictVote); dislikeInstead {
				conflictVote = conflictVote.WithOpinion(votes.Dislike).WithVotePower(votePower)
			}
		}

		if added, opinionChanged := votesObj.Add(conflictVote); added && opinionChanged {
			collectedEvents = append(collectedEvents, &VoterEvent[ConflictIDType]{Voter: conflictVote.Voter, ConflictID: conflict, Opinion: conflictVote.Opinion})
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
	votesObj, exists := c.votes.Get(conflictID)
	if !exists {
		panic(fmt.Sprintf("votes for conflict %v not found", conflictID))
	}

	vote, exists := votesObj.Vote(voter)
	if !exists {
		return false
	}
	return vote.Opinion == votes.Like
}

func (c *ConflictTracker[ConflictIDType, ResourceIDType, VotePowerType]) revokeConflictInstead(conflictID ConflictIDType, vote *votes.Vote[ConflictIDType, VotePowerType]) (votePower VotePowerType, revokeInstead bool) {
	c.conflictDAG.Utils.ForEachConflictingConflictID(conflictID, func(conflictingConflictID ConflictIDType) bool {
		votesObj, conflictVotesExist := c.votes.Get(conflictingConflictID)
		if !conflictVotesExist {
			revokeInstead = false
			return false
		}

		existingVote, voteExists := votesObj.Vote(vote.Voter)
		if !voteExists {
			revokeInstead = false
			return false
		}

		if existingVote.VotePower.CompareTo(vote.VotePower) >= 0 && existingVote.Opinion == votes.Like {
			revokeInstead = true
			votePower = existingVote.VotePower
			return false
		}

		return true
	})

	return votePower, revokeInstead
}

func (c *ConflictTracker[ConflictIDType, ResourceIDType, VotePowerType]) triggerEvents(eventsToTrigger []*VoterEvent[ConflictIDType]) {
	for _, event := range eventsToTrigger {
		switch event.Opinion {
		case votes.Like:
			c.Events.VoterAdded.Trigger(event)
		case votes.Dislike:
			c.Events.VoterRemoved.Trigger(event)
		}
	}
}
