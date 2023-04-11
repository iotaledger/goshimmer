package vote

import (
	"github.com/iotaledger/hive.go/constraints"
	"github.com/iotaledger/hive.go/crypto/identity"
)

type Vote[Power constraints.Comparable[Power]] struct {
	Voter identity.ID
	Power Power

	liked bool
}

func NewVote[Power constraints.Comparable[Power]](voter identity.ID, power Power) *Vote[Power] {
	return &Vote[Power]{
		Voter: voter,
		Power: power,
		liked: true,
	}
}

func (v *Vote[Power]) IsLiked() bool {
	return v.liked
}

func (v *Vote[Power]) WithLiked(liked bool) *Vote[Power] {
	updatedVote := new(Vote[Power])
	updatedVote.Voter = v.Voter
	updatedVote.Power = v.Power
	updatedVote.liked = liked

	return updatedVote
}
