package votes

import (
	"sync"

	"github.com/iotaledger/hive.go/constraints"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/ds/orderedmap"
)

type Vote[ConflictIDType comparable, VotePowerType constraints.Comparable[VotePowerType]] struct {
	Voter      identity.ID
	ConflictID ConflictIDType
	Opinion    Opinion
	VotePower  VotePowerType
}

// NewVote derives a Vote for th.
func NewVote[ConflictIDType comparable, VotePowerType constraints.Comparable[VotePowerType]](voter identity.ID, votePower VotePowerType, opinion Opinion) (voteWithOpinion *Vote[ConflictIDType, VotePowerType]) {
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

// WithVotePower derives a vote for the given VotePower.
func (v *Vote[ConflictIDType, VotePowerType]) WithVotePower(power VotePowerType) (voteWithOpinion *Vote[ConflictIDType, VotePowerType]) {
	return &Vote[ConflictIDType, VotePowerType]{
		Voter:      v.Voter,
		ConflictID: v.ConflictID,
		Opinion:    v.Opinion,
		VotePower:  power,
	}
}

// region Votes ////////////////////////////////////////////////////////////////////////////////////////////////////////

type Votes[ConflictIDType comparable, VotePowerType constraints.Comparable[VotePowerType]] struct {
	o orderedmap.OrderedMap[identity.ID, *Vote[ConflictIDType, VotePowerType]]

	m sync.RWMutex
}

func NewVotes[ConflictIDType comparable, VotePowerType constraints.Comparable[VotePowerType]]() *Votes[ConflictIDType, VotePowerType] {
	return &Votes[ConflictIDType, VotePowerType]{
		o: *orderedmap.New[identity.ID, *Vote[ConflictIDType, VotePowerType]](),
	}
}

func (v *Votes[ConflictIDType, VotePowerType]) Add(vote *Vote[ConflictIDType, VotePowerType]) (added bool, opinionChanged bool) {
	v.m.Lock()
	defer v.m.Unlock()

	previousVote, exists := v.o.Get(vote.Voter)
	if !exists {
		v.o.Set(vote.Voter, vote)
		return true, true
	}

	if vote.VotePower.Compare(previousVote.VotePower) <= 0 {
		return false, false
	}

	v.o.Set(vote.Voter, vote)

	return true, previousVote.Opinion != vote.Opinion
}

func (v *Votes[ConflictIDType, VotePowerType]) Delete(vote *Vote[ConflictIDType, VotePowerType]) (deleted bool) {
	v.m.Lock()
	defer v.m.Unlock()

	return v.o.Delete(vote.Voter)
}

func (v *Votes[ConflictIDType, VotePowerType]) Voters() (voters *advancedset.AdvancedSet[identity.ID]) {
	voters = advancedset.New[identity.ID]()

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

func (v *Votes[ConflictIDType, VotePowerType]) Vote(voter identity.ID) (vote *Vote[ConflictIDType, VotePowerType], exists bool) {
	v.m.RLock()
	defer v.m.RUnlock()

	return v.o.Get(voter)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Opinion //////////////////////////////////////////////////////////////////////////////////////////////////////

// Opinion is a type that represents the Opinion of a node on a certain Conflict.
type Opinion uint8

const (
	// UndefinedOpinion represents the zero value of the Opinion type.
	UndefinedOpinion Opinion = iota
	Like
	Dislike
)

func (o Opinion) String() string {
	switch o {
	case Like:
		return "Like"
	case Dislike:
		return "Dislike"
	default:
		return "UndefinedOpinion"
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
