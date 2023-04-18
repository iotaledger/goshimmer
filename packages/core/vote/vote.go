package vote

import (
	"github.com/iotaledger/hive.go/constraints"
	"github.com/iotaledger/hive.go/crypto/identity"
)

// Vote represents a vote that is cast by a voter.
type Vote[Power constraints.Comparable[Power]] struct {
	// Voter is the identity of the voter.
	Voter identity.ID

	// Power is the power of the voter.
	Power Power

	// liked is true if the vote is "positive" (voting "for something").
	liked bool
}

// NewVote creates a new vote.
func NewVote[Power constraints.Comparable[Power]](voter identity.ID, power Power) *Vote[Power] {
	return &Vote[Power]{
		Voter: voter,
		Power: power,
		liked: true,
	}
}

// IsLiked returns true if the vote is "positive" (voting "for something").
func (v *Vote[Power]) IsLiked() bool {
	return v.liked
}

// WithLiked returns a copy of the vote with the given liked value.
func (v *Vote[Power]) WithLiked(liked bool) *Vote[Power] {
	updatedVote := new(Vote[Power])
	updatedVote.Voter = v.Voter
	updatedVote.Power = v.Power
	updatedVote.liked = liked

	return updatedVote
}
