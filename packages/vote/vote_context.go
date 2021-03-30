package vote

import "github.com/iotaledger/goshimmer/packages/vote/opinion"

// NewContext creates a new vote context.
func NewContext(id string, objectType ObjectType, initOpn opinion.Opinion) *Context {
	voteCtx := &Context{ID: id, Type: objectType, ProportionLiked: likedInit}
	voteCtx.AddOpinion(initOpn)
	return voteCtx
}

const likedInit = -1

// ObjectType is the object type of a voting (e.g., conflict or timestamp)
type ObjectType uint8

const (
	// ConflictType defines an object type conflict.
	ConflictType = iota
	// TimestampType defines an object type timestamp.
	TimestampType
)

// Context is the context of votes from multiple rounds about a given item.
type Context struct {
	ID   string
	Type ObjectType
	// The percentage of OpinionGivers who liked this item on the last query.
	ProportionLiked float64
	// The number of voting rounds performed.
	Rounds int
	// Append-only list of opinions formed after each round.
	// the first opinion is the initial opinion when this vote context was created.
	Opinions []opinion.Opinion
	// Weights used for voting
	Weights VotingWeights
}

// VotingWeights stores parameters used for weighted voting calculation
type VotingWeights struct {
	// Total base mana of opinion givers from the last query
	TotalWeights float64
	// Own mana from the last query
	OwnWeight float64
}

// AddOpinion adds the given opinion to this vote context.
func (vc *Context) AddOpinion(opn opinion.Opinion) {
	vc.Opinions = append(vc.Opinions, opn)
}

// LastOpinion returns the last formed opinion.
func (vc *Context) LastOpinion() opinion.Opinion {
	return vc.Opinions[len(vc.Opinions)-1]
}

// IsFinalized tells whether this vote context is finalized by checking whether the opinion was held
// for totalRoundsFinalization number of rounds.
func (vc *Context) IsFinalized(coolingOffPeriod, totalRoundsFinalization int) bool {
	// check whether we have enough opinions to say whether this vote context is finalized.
	if len(vc.Opinions) < coolingOffPeriod+totalRoundsFinalization+1 {
		return false
	}

	// grab opinion which needs to be held for TotalRoundsFinalization number of rounds
	candidateOpinion := vc.Opinions[len(vc.Opinions)-totalRoundsFinalization]

	// check whether it was held for the subsequent rounds
	for _, subsequentOpinion := range vc.Opinions[len(vc.Opinions)-totalRoundsFinalization+1:] {
		if subsequentOpinion != candidateOpinion {
			return false
		}
	}
	return true
}

// IsNew tells whether the vote context is new.
func (vc *Context) IsNew() bool {
	return vc.ProportionLiked == likedInit
}

// HadFirstRound tells whether the vote context just had its first round.
func (vc *Context) HadFirstRound() bool {
	return vc.Rounds == 1
}

// HadFixedRound tells whether the vote context is in the last l2 rounds of fixed threshold
func (vc *Context) HadFixedRound(coolingOffPeriod, totalRoundsFinalization, totalRoundsFixedThreshold int) bool {
	totalRoundsRandomThreshold := totalRoundsFinalization - totalRoundsFixedThreshold // l - l2
	// check whether we have enough opinions to say whether the random threshold for this vote context can be fixed.
	if len(vc.Opinions) < coolingOffPeriod+totalRoundsRandomThreshold+1 {
		return false
	}
	// check whether we have enough opinions and parameters are valid
	if len(vc.Opinions) < totalRoundsRandomThreshold || totalRoundsRandomThreshold < 0 {
		return false
	}
	// grab opinion which needs to be held for totalRoundsRandomThreshold number of rounds
	candidateOpinion := vc.Opinions[len(vc.Opinions)-totalRoundsRandomThreshold]

	// check whether it was held for the subsequent rounds
	for _, subsequentOpinion := range vc.Opinions[len(vc.Opinions)-totalRoundsRandomThreshold+1:] {
		if subsequentOpinion != candidateOpinion {
			return false
		}
	}
	return true
}
