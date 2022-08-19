package otv

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/orderedmap"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/validator"
)

type Vote struct {
	Voter      *validator.Validator
	ConflictID utxo.TransactionID
	Opinion    Opinion
	VotePower  VotePower
}

// NewConflictVote derives a Vote for th.
func NewConflictVote(voter *validator.Validator, votePower VotePower, conflictID utxo.TransactionID, opinion Opinion) (voteWithOpinion *Vote) {
	return &Vote{
		Voter:      voter,
		VotePower:  votePower,
		ConflictID: conflictID,
		Opinion:    opinion,
	}
}

// WithOpinion derives a vote for the given Opinion.
func (v *Vote) WithOpinion(opinion Opinion) (voteWithOpinion *Vote) {
	return &Vote{
		Voter:      v.Voter,
		ConflictID: v.ConflictID,
		Opinion:    opinion,
		VotePower:  v.VotePower,
	}
}

// WithConflictID derives a vote for the given ConflictID.
func (v *Vote) WithConflictID(conflictID utxo.TransactionID) (voteWithConflictID *Vote) {
	return &Vote{
		Voter:      v.Voter,
		ConflictID: conflictID,
		Opinion:    v.Opinion,
		VotePower:  v.VotePower,
	}
}

// region Votes ////////////////////////////////////////////////////////////////////////////////////////////////////////

type Votes struct {
	o orderedmap.OrderedMap[identity.ID, *Vote]

	m sync.RWMutex
}

func NewVotes() *Votes {
	return &Votes{
		o: *orderedmap.New[identity.ID, *Vote](),
	}
}

func (v *Votes) Add(vote *Vote) (added bool, opinionChanged bool) {
	v.m.Lock()
	defer v.m.Unlock()

	previousVote, exists := v.o.Get(vote.Voter.ID())
	if !exists {
		return v.o.Set(vote.Voter.ID(), vote), true
	}
	if vote.VotePower < previousVote.VotePower {
		return false, false
	}

	return v.o.Set(vote.Voter.ID(), vote), previousVote.Opinion != vote.Opinion
}

func (v *Votes) Delete(vote *Vote) (deleted bool) {
	v.m.Lock()
	defer v.m.Unlock()

	return v.o.Delete(vote.Voter.ID())
}

func (v *Votes) Voters() (voters *set.AdvancedSet[*validator.Validator]) {
	voters = set.NewAdvancedSet[*validator.Validator]()

	v.m.RLock()
	defer v.m.RUnlock()

	v.o.ForEach(func(id identity.ID, vote *Vote) bool {
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
type VotePower = uint64

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
