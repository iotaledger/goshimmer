package votes

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/orderedmap"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/validator"
)

type Vote[ConflictIDType comparable] struct {
	Voter      *validator.Validator
	ConflictID ConflictIDType
	Opinion    Opinion
	VotePower  VotePower
}

// NewVote derives a Vote for th.
func NewVote[ConflictIDType comparable](voter *validator.Validator, votePower VotePower, opinion Opinion) (voteWithOpinion *Vote[ConflictIDType]) {
	return &Vote[ConflictIDType]{
		Voter:     voter,
		VotePower: votePower,
		Opinion:   opinion,
	}
}

// WithOpinion derives a vote for the given Opinion.
func (v *Vote[ConflictIDType]) WithOpinion(opinion Opinion) (voteWithOpinion *Vote[ConflictIDType]) {
	return &Vote[ConflictIDType]{
		Voter:      v.Voter,
		ConflictID: v.ConflictID,
		Opinion:    opinion,
		VotePower:  v.VotePower,
	}
}

// WithConflictID derives a vote for the given ConflictID.
func (v *Vote[ConflictIDType]) WithConflictID(conflictID ConflictIDType) (voteWithConflictID *Vote[ConflictIDType]) {
	return &Vote[ConflictIDType]{
		Voter:      v.Voter,
		ConflictID: conflictID,
		Opinion:    v.Opinion,
		VotePower:  v.VotePower,
	}
}

// region Votes ////////////////////////////////////////////////////////////////////////////////////////////////////////

type Votes[ConflictIDType comparable] struct {
	o orderedmap.OrderedMap[identity.ID, *Vote[ConflictIDType]]

	m sync.RWMutex
}

func NewVotes[ConflictIDType comparable]() *Votes[ConflictIDType] {
	return &Votes[ConflictIDType]{
		o: *orderedmap.New[identity.ID, *Vote[ConflictIDType]](),
	}
}

func (v *Votes[ConflictIDType]) Add(vote *Vote[ConflictIDType]) (added bool, opinionChanged bool) {
	v.m.Lock()
	defer v.m.Unlock()

	previousVote, exists := v.o.Get(vote.Voter.ID())
	if !exists {
		return v.o.Set(vote.Voter.ID(), vote), true
	}
	if vote.VotePower.CompareTo(previousVote.VotePower) <= 0 {
		return false, false
	}

	return v.o.Set(vote.Voter.ID(), vote), previousVote.Opinion != vote.Opinion
}

func (v *Votes[ConflictIDType]) Delete(vote *Vote[ConflictIDType]) (deleted bool) {
	v.m.Lock()
	defer v.m.Unlock()

	return v.o.Delete(vote.Voter.ID())
}

func (v *Votes[ConflictIDType]) Voters() (voters *set.AdvancedSet[*validator.Validator]) {
	voters = set.NewAdvancedSet[*validator.Validator]()

	v.m.RLock()
	defer v.m.RUnlock()

	v.o.ForEach(func(id identity.ID, vote *Vote[ConflictIDType]) bool {
		if vote.Opinion == Like {
			voters.Add(vote.Voter)
		}
		return true
	})

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// VotePower is used to establish an absolute order of votes, regardless of their arrival order.
// Currently, the used VotePower is the SequenceNumber embedded in the Block Layout, so that, regardless
// of the order in which votes are received, the same conclusion is computed.
// Alternatively, the objective timestamp of a Block could be used.
type VotePower interface {
	CompareTo(other VotePower) int
}

// region Opinion //////////////////////////////////////////////////////////////////////////////////////////////////////

// Opinion is a type that represents the Opinion of a node on a certain Conflict.
type Opinion uint8

const (
	// UndefinedOpinion represents the zero value of the Opinion type.
	UndefinedOpinion Opinion = iota
	Like
	Dislike
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
