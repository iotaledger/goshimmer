package votes

import (
	"fmt"

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

	if c.revokeInstead(initialVote, voter, power) {
		c.applyVotes(defaultVote.WithOpinion(Dislike), addedConflictIDs, c.Events.VoterRemoved)
	} else {
		c.applyVotes(defaultVote.WithOpinion(Like), addedConflictIDs, c.Events.VoterAdded)
		c.applyVotes(defaultVote.WithOpinion(Dislike), revokedConflictIDs, c.Events.VoterRemoved)
	}

	return true, false
}

func (c *ConflictTracker[ConflictIDType, ResourceIDType, VotePowerType]) applyVotes(defaultVote *Vote[ConflictIDType, VotePowerType], conflictIDs *set.AdvancedSet[ConflictIDType], triggerEvent *event.Event[*ConflictVoterEvent[ConflictIDType]]) {
	for it := conflictIDs.Iterator(); it.HasNext(); {
		conflict := it.Next()
		votes, _ := c.votes.RetrieveOrCreate(conflict, NewVotes[ConflictIDType, VotePowerType])

		if added, opinionChanged := votes.Add(defaultVote.WithConflictID(conflict)); added && opinionChanged {
			triggerEvent.Trigger(&ConflictVoterEvent[ConflictIDType]{Voter: defaultVote.Voter, ConflictID: conflict})
		}
	}
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
		panic(fmt.Sprintf("votes for conflict %s not found", conflictID))
	}

	vote, exists := votes.Vote(voter)
	if !exists {
		return false
	}
	return vote.Opinion == Like
}

func (c *ConflictTracker[ConflictIDType, ResourceIDType, VotePowerType]) revokeInstead(initialVote *set.AdvancedSet[ConflictIDType], voter *validator.Validator, power VotePowerType) (revokeInstead bool) {

	for it := initialVote.Iterator(); it.HasNext() && !revokeInstead; {
		conflictID := it.Next()
		c.conflictDAG.Utils.ForEachConflictingConflictID(conflictID, func(conflictingConflictID ConflictIDType) bool {
			votes, conflictVotesExist := c.votes.Get(conflictingConflictID)
			if !conflictVotesExist {
				// TODO: can return from here?
				return true
			}
			vote, voteExists := votes.Vote(voter)
			if !voteExists {
				// TODO: can return from here?
				return true
			}
			if vote.VotePower.CompareTo(power) >= 0 {
				revokeInstead = true
				return false
			}

			return true
		})
	}

	return
}
