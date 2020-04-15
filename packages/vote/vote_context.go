package vote

// NewContext creates a new vote context.
func NewContext(id string, initOpn Opinion) *Context {
	voteCtx := &Context{ID: id, Liked: likedInit}
	voteCtx.AddOpinion(initOpn)
	return voteCtx
}

const likedInit = -1

// Context is the context of votes from multiple rounds about a given item.
type Context struct {
	ID string
	// The percentage of OpinionGivers who liked this item on the last query.
	Liked float64
	// The number of voting rounds performed.
	Rounds int
	// Append-only list of opinions formed after each round.
	// the first opinion is the initial opinion when this vote context was created.
	Opinions []Opinion
}

// AddOpinion adds the given opinion to this vote context.
func (vc *Context) AddOpinion(opn Opinion) {
	vc.Opinions = append(vc.Opinions, opn)
}

// LastOpinion returns the last formed opinion.
func (vc *Context) LastOpinion() Opinion {
	return vc.Opinions[len(vc.Opinions)-1]
}

// IsFinalized tells whether this vote context is finalized by checking whether the opinion was held
// for finalizationThreshold number of rounds.
func (vc *Context) IsFinalized(coolingOffPeriod int, finalizationThreshold int) bool {
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

// IsNew tells whether the vote context is new.
func (vc *Context) IsNew() bool {
	return vc.Liked == likedInit
}

// HadFirstRound tells whether the vote context just had its first round.
func (vc *Context) HadFirstRound() bool {
	return vc.Rounds == 1
}
