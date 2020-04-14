package fpc

import "github.com/iotaledger/goshimmer/packages/vote"

func newVoteContext(id string, initOpn vote.Opinion) *VoteContext {
	voteCtx := &VoteContext{id: id, Liked: likedInit}
	voteCtx.AddOpinion(initOpn)
	return voteCtx
}

const likedInit = -1

// VoteContext is the context of votes from multiple rounds about a given item.
type VoteContext struct {
	id string
	// The percentage of OpinionGivers who liked this item on the last query.
	Liked float64
	// The number of voting rounds performed.
	Rounds int
	// Append-only list of opinions formed after each round.
	// the first opinion is the initial opinion when this vote context was created.
	Opinions []vote.Opinion
}

// adds the given opinion to this vote context
func (vc *VoteContext) AddOpinion(opn vote.Opinion) {
	vc.Opinions = append(vc.Opinions, opn)
}

func (vc *VoteContext) LastOpinion() vote.Opinion {
	return vc.Opinions[len(vc.Opinions)-1]
}

// tells whether this vote context is finalized by checking whether the opinion was held
// for finalizationThreshold number of rounds.
func (vc *VoteContext) IsFinalized(coolingOffPeriod int, finalizationThreshold int) bool {
	// check whether we have enough opinions to say whether this vote context is finalized.
	// we start from the 2nd opinion since the first one is the initial opinion.
	if len(vc.Opinions[1:]) < coolingOffPeriod+finalizationThreshold {
		return false
	}

	// grab opinion which needs to be held for finalizationThreshold number of rounds
	candidateOpinion := vc.Opinions[len(vc.Opinions)-finalizationThreshold]

	// check whether it was held for the subsequent rounds
	for _, subsequentOpinion := range vc.Opinions[len(vc.Opinions)-finalizationThreshold+1:] {
		if subsequentOpinion != candidateOpinion {
			return false
		}
	}
	return true
}

// tells whether the vote context is new.
func (vc *VoteContext) IsNew() bool {
	return vc.Liked == likedInit
}

// tells whether the vote context just had its first round.
func (vc *VoteContext) HadFirstRound() bool {
	return vc.Rounds == 1
}
