package otv

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/orderedmap"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/validator"
)

type Vote[ConflictIDType comparable, VotePowerType VotePower[any]] struct {
	Voter      *validator.Validator
	ConflictID ConflictIDType
	Opinion    Opinion
	VotePower  VotePowerType
}

// NewVote derives a Vote for th.
func NewVote[ConflictIDType comparable, VotePowerType VotePower[any]](voter *validator.Validator, votePower VotePowerType, opinion Opinion) (voteWithOpinion *Vote[ConflictIDType, VotePowerType]) {
	return &Vote[ConflictIDType, VotePowerType]{
		Voter:     voter,
		VotePower: votePower,
		Opinion:   opinion,
	}
}

// WithOpinion derives a vote for the given Opinion.
func (v *Vote[ConflictIDType, VotePowerType]) WithOpinion(opinion Opinion) (voteWithOpinion *Vote[ConflictIDType, VotePowerType]) {
	return &Vote[ConflictIDType, VotePowerType]{
		Voter:      v.Voter,
		ConflictID: v.ConflictID,
		Opinion:    opinion,
		VotePower:  v.VotePower,
	}
}

// WithConflictID derives a vote for the given ConflictID.
func (v *Vote[ConflictIDType, VotePowerType]) WithConflictID(conflictID ConflictIDType) (voteWithConflictID *Vote[ConflictIDType, VotePowerType]) {
	return &Vote[ConflictIDType, VotePowerType]{
		Voter:      v.Voter,
		ConflictID: conflictID,
		Opinion:    v.Opinion,
		VotePower:  v.VotePower,
	}
}

// region Votes ////////////////////////////////////////////////////////////////////////////////////////////////////////

type Votes[ConflictIDType comparable, VotePowerType VotePower[any]] struct {
	o orderedmap.OrderedMap[identity.ID, *Vote[ConflictIDType, VotePowerType]]

	m sync.RWMutex
}

func NewVotes[ConflictIDType comparable, VotePowerType VotePower[any]]() *Votes[ConflictIDType, VotePowerType] {
	return &Votes[ConflictIDType, VotePowerType]{
		o: *orderedmap.New[identity.ID, *Vote[ConflictIDType, VotePowerType]](),
	}
}

func (v *Votes[ConflictIDType, VotePowerType]) Add(vote *Vote[ConflictIDType, VotePowerType]) (added bool, opinionChanged bool) {
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

func (v *Votes[ConflictIDType, VotePowerType]) Delete(vote *Vote[ConflictIDType, VotePowerType]) (deleted bool) {
	v.m.Lock()
	defer v.m.Unlock()

	return v.o.Delete(vote.Voter.ID())
}

func (v *Votes[ConflictIDType, VotePowerType]) Voters() (voters *set.AdvancedSet[*validator.Validator]) {
	voters = set.NewAdvancedSet[*validator.Validator]()

	v.m.RLock()
	defer v.m.RUnlock()

	v.o.ForEach(func(id identity.ID, vote *Vote[ConflictIDType, VotePowerType]) bool {
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
type VotePower[T any] interface {
	CompareTo(other T) int
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
