package otv

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/conflictdag"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/validator"
)

type VotesTracker[ConflictIDType, ResourceIDType comparable] struct {
	votes *memstorage.Storage[ConflictIDType, *Votes]

	conflictDAG  *conflictdag.ConflictDAG[ConflictIDType, ResourceIDType]
	validatorSet *validator.Set
	Events       *Events
}

func NewVotesTracker[ConflictIDType, ResourceIDType comparable](conflictDAG *conflictdag.ConflictDAG[ConflictIDType, ResourceIDType], validatorSet *validator.Set) *VotesTracker[ConflictIDType, ResourceIDType] {
	return &VotesTracker[ConflictIDType, ResourceIDType]{
		conflictDAG:  conflictDAG,
		validatorSet: validatorSet,
		Events:       newEvents(),
	}
}

func (v *VotesTracker[ConflictIDType, ResourceIDType]) TrackVote(initialVote utxo.TransactionIDs, voterID identity.ID, power VotePower) {
	addedConflicts, revokedConflicts, invalid := v.conflictDAG.DetermineVotes(initialVote)
	if invalid {
		// TODO: set as subjectively invalid
	}

	voter, exists := v.validatorSet.Get(voterID)
	if !exists {
		return
	}

	defaultVote := NewConflictVote(voter, power, utxo.TransactionID{}, UndefinedOpinion)

	v.applyVotes(defaultVote.WithOpinion(Like), addedConflicts, v.Events.VoterAdded)
	v.applyVotes(defaultVote.WithOpinion(Dislike), revokedConflicts, v.Events.VoterRemoved)
}

func (v *VotesTracker[ConflictIDType, ResourceIDType]) applyVotes(defaultVote *Vote, conflicts utxo.TransactionIDs, triggerEvent *event.Event[*VoterEvent]) {
	for it := conflicts.Iterator(); it.HasNext(); {
		conflict := it.Next()
		votes, _ := v.votes.RetrieveOrCreate(conflict, NewVotes)

		if added, opinionChanged := votes.Add(defaultVote.WithConflictID(conflict)); added && opinionChanged {
			triggerEvent.Trigger(&VoterEvent{Voter: defaultVote.Voter, Resource: conflict})
		}
	}
}

func (v *VotesTracker[ConflictIDType, ResourceIDType]) Voters(conflict utxo.TransactionID) (voters *set.AdvancedSet[*validator.Validator]) {
	votes, exists := v.votes.Get(conflict)
	if !exists {
		return set.NewAdvancedSet[*validator.Validator]()
	}

	return votes.Voters()
}
