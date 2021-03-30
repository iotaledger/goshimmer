package fpc_test

import (
	"context"
	"errors"
	"math/rand"
	"testing"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/packages/vote/fpc"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
)

func TestVoteContext_IsFinalized(t *testing.T) {
	type testInput struct {
		voteCtx                     vote.Context
		totalRoundsCoolingOffPeriod int
		totalRoundsFinalization     int
		want                        bool
	}
	tests := []testInput{
		{vote.Context{
			Opinions: []opinion.Opinion{opinion.Like, opinion.Like, opinion.Like, opinion.Like, opinion.Like},
		}, 2, 2, true},
		{vote.Context{
			Opinions: []opinion.Opinion{opinion.Like, opinion.Like, opinion.Like, opinion.Like, opinion.Dislike},
		}, 2, 2, false},
	}

	for _, test := range tests {
		assert.Equal(t, test.want, test.voteCtx.IsFinalized(test.totalRoundsCoolingOffPeriod, test.totalRoundsFinalization))
	}
}

func TestVoteContext_HadFixedRound(t *testing.T) {
	type testInput struct {
		voteCtx                     vote.Context
		totalRoundsCoolingOffPeriod int
		totalRoundsFinalization     int
		totalRoundsFixedThreshold   int
		want                        bool
	}
	tests := []testInput{
		{vote.Context{Opinions: []opinion.Opinion{opinion.Like, opinion.Like, opinion.Like, opinion.Like, opinion.Like}}, 2, 4, 2, true},
		{vote.Context{Opinions: []opinion.Opinion{opinion.Like, opinion.Like, opinion.Like, opinion.Like, opinion.Dislike}}, 2, 4, 2, false},
	}
	for _, test := range tests {
		assert.Equal(t, test.want, test.voteCtx.HadFixedRound(test.totalRoundsCoolingOffPeriod, test.totalRoundsFinalization, test.totalRoundsFixedThreshold))
	}
}

func TestVoteContext_LastOpinion(t *testing.T) {
	type testInput struct {
		voteCtx  vote.Context
		expected opinion.Opinion
	}
	tests := []testInput{
		{vote.Context{
			Opinions: []opinion.Opinion{opinion.Like, opinion.Like, opinion.Like, opinion.Like},
		}, opinion.Like},
		{vote.Context{
			Opinions: []opinion.Opinion{opinion.Like, opinion.Like, opinion.Like, opinion.Dislike},
		}, opinion.Dislike},
	}

	for _, test := range tests {
		assert.Equal(t, test.expected, test.voteCtx.LastOpinion())
	}
}

func TestFPCPreventSameIDMultipleTimes(t *testing.T) {
	voter := fpc.New(nil, nil)
	assert.NoError(t, voter.Vote("a", vote.ConflictType, opinion.Like))
	// can't add the same item twice
	assert.True(t, errors.Is(voter.Vote("a", vote.ConflictType, opinion.Like), fpc.ErrVoteAlreadyOngoing))
}

type opiniongivermock struct {
	id            identity.ID
	roundsReplies []opinion.Opinions
	roundIndex    int
	mana          float64
}

func (ogm *opiniongivermock) ID() identity.ID {
	var zeroVal identity.ID
	// if zero value, assign identity
	if ogm.id == zeroVal {
		ogm.id = identity.GenerateIdentity().ID()
	}
	return ogm.id
}

func (ogm *opiniongivermock) Query(_ context.Context, _ []string, _ []string) (opinion.Opinions, error) {
	if ogm.roundIndex >= len(ogm.roundsReplies) {
		return ogm.roundsReplies[len(ogm.roundsReplies)-1], nil
	}
	opinions := ogm.roundsReplies[ogm.roundIndex]
	ogm.roundIndex++
	return opinions, nil
}

func (ogm *opiniongivermock) Mana() float64 {
	return ogm.mana
}

func TestFPCFinalizedEvent(t *testing.T) {
	opinionGiverMock := &opiniongivermock{
		roundsReplies: []opinion.Opinions{
			// 2 cool-off period, 2 finalization threshold
			{opinion.Like}, {opinion.Like}, {opinion.Like}, {opinion.Like},
		},
	}
	opinionGiverFunc := func() (givers []opinion.OpinionGiver, err error) {
		return []opinion.OpinionGiver{opinionGiverMock}, nil
	}

	ownWeightRetrieverFunc := func() (float64, error) {
		return 0, nil
	}

	id := "a"

	paras := fpc.DefaultParameters()
	paras.TotalRoundsFinalization = 2
	paras.TotalRoundsCoolingOffPeriod = 2
	paras.QuerySampleSize = 1
	voter := fpc.New(opinionGiverFunc, ownWeightRetrieverFunc, paras)
	paras.MinOpinionsReceived = 1

	var finalizedOpinion *opinion.Opinion
	voter.Events().Finalized.Attach(events.NewClosure(func(ev *vote.OpinionEvent) {
		finalizedOpinion = &ev.Opinion
	}))
	assert.NoError(t, voter.Vote(id, vote.ConflictType, opinion.Like))

	// do 5 rounds of FPC -> 5 because the last one finalizes the vote
	for i := 0; i < 5; i++ {
		assert.NoError(t, voter.Round(0.5))
	}

	require.NotNil(t, finalizedOpinion, "finalized event should have been fired")
	assert.Equal(t, opinion.Like, *finalizedOpinion, "the final opinion should have been 'Like'")
}

func TestFPCFailedEvent(t *testing.T) {
	opinionGiverFunc := func() (givers []opinion.OpinionGiver, err error) {
		return []opinion.OpinionGiver{&opiniongivermock{
			// doesn't matter what we set here
			roundsReplies: []opinion.Opinions{{opinion.Dislike}},
		}}, nil
	}
	ownWeightRetrieverFunc := func() (float64, error) {
		return 0, nil
	}

	id := "a"

	paras := fpc.DefaultParameters()
	paras.QuerySampleSize = 1
	paras.MinOpinionsReceived = 1
	paras.MaxRoundsPerVoteContext = 3
	paras.TotalRoundsCoolingOffPeriod = 0
	// since the finalization threshold is over max rounds it will
	// always fail finalizing an opinion
	paras.TotalRoundsFinalization = 4
	voter := fpc.New(opinionGiverFunc, ownWeightRetrieverFunc, paras)
	var failedOpinion *opinion.Opinion
	voter.Events().Failed.Attach(events.NewClosure(func(ev *vote.OpinionEvent) {
		failedOpinion = &ev.Opinion
	}))
	assert.NoError(t, voter.Vote(id, vote.ConflictType, opinion.Like))

	for i := 0; i < 4; i++ {
		assert.NoError(t, voter.Round(0.5))
	}

	require.NotNil(t, failedOpinion, "failed event should have been fired")
	assert.Equal(t, opinion.Dislike, *failedOpinion, "the final opinion should have been 'Dislike'")
}

func TestFPCVotingMultipleOpinionGivers(t *testing.T) {
	type testInput struct {
		id                 string
		initOpinion        opinion.Opinion
		expectedRoundsDone int
		expectedOpinion    opinion.Opinion
	}
	tests := []testInput{
		{"1", opinion.Like, 5, opinion.Like},
		{"2", opinion.Dislike, 5, opinion.Dislike},
	}

	for _, test := range tests {
		// note that even though we're defining QuerySampleSize times opinion givers,
		// it doesn't mean that FPC will query all of them.
		opinionGiverFunc := func() (givers []opinion.OpinionGiver, err error) {
			opinionGivers := make([]opinion.OpinionGiver, fpc.DefaultParameters().QuerySampleSize)
			for i := 0; i < len(opinionGivers); i++ {
				opinionGivers[i] = &opiniongivermock{roundsReplies: []opinion.Opinions{{test.initOpinion}}}
			}
			return opinionGivers, nil
		}
		ownWeightRetrieverFunc := func() (float64, error) {
			return 0, nil
		}

		paras := fpc.DefaultParameters()
		paras.TotalRoundsFinalization = 2
		paras.TotalRoundsCoolingOffPeriod = 2
		voter := fpc.New(opinionGiverFunc, ownWeightRetrieverFunc, paras)
		var finalOpinion *opinion.Opinion
		voter.Events().Finalized.Attach(events.NewClosure(func(ev *vote.OpinionEvent) {
			finalOpinion = &ev.Opinion
		}))

		assert.NoError(t, voter.Vote(test.id, vote.ConflictType, test.initOpinion))

		var roundsDone int
		for finalOpinion == nil {
			assert.NoError(t, voter.Round(0.7))
			roundsDone++
		}

		assert.Equal(t, test.expectedRoundsDone, roundsDone)
		require.NotNil(t, finalOpinion)
		assert.Equal(t, test.expectedOpinion, *finalOpinion)
	}
}

func TestManaBasedSamplingFallback(t *testing.T) {
	// opinion givers without mana
	opinionGivers := make([]opinion.OpinionGiver, fpc.DefaultParameters().QuerySampleSize)
	for i := 0; i < len(opinionGivers); i++ {
		opinionGivers[i] = &opiniongivermock{id: identity.GenerateIdentity().ID()}
	}

	// nodes sampled uniformly
	expectedOpinionGivers := map[opinion.OpinionGiver]int{
		opinionGivers[5]:  3,
		opinionGivers[11]: 3,
		opinionGivers[15]: 1,
		opinionGivers[10]: 1,
		opinionGivers[17]: 3,
		opinionGivers[20]: 1,
		opinionGivers[2]:  1,
		opinionGivers[19]: 2,
		opinionGivers[7]:  2,
		opinionGivers[6]:  2,
		opinionGivers[9]:  1,
		opinionGivers[4]:  1,
	}

	// provide custom rng with fixed seed, so that execution is deterministic
	opinionGiversToQuery, _ := fpc.ManaBasedSampling(opinionGivers, fpc.DefaultParameters().MaxQuerySampleSize, fpc.DefaultParameters().QuerySampleSize, rand.New(rand.NewSource(42)))
	sumVotes := 0
	for _, v := range opinionGiversToQuery {
		sumVotes += v
	}
	assert.Equal(t, fpc.DefaultParameters().QuerySampleSize, sumVotes)
	assert.Equal(t, expectedOpinionGivers, opinionGiversToQuery)
}

func TestManaBasedSampling(t *testing.T) {
	// opinion givers with exponentially distributed mana
	opinionGivers := make([]opinion.OpinionGiver, fpc.DefaultParameters().QuerySampleSize)
	for i := 0; i < len(opinionGivers); i++ {
		opinionGivers[i] = &opiniongivermock{mana: float64(i * i), id: identity.GenerateIdentity().ID()}
	}

	// nodes with more mana are sampled more often (index represents sqrt(mana)
	expectedOpinionGivers := map[opinion.OpinionGiver]int{
		opinionGivers[20]: 17,
		opinionGivers[14]: 5,
		opinionGivers[9]:  1,
		opinionGivers[15]: 7,
		opinionGivers[7]:  3,
		opinionGivers[18]: 15,
		opinionGivers[10]: 3,
		opinionGivers[16]: 7,
		opinionGivers[13]: 8,
		opinionGivers[8]:  4,
		opinionGivers[12]: 7,
		opinionGivers[11]: 3,
		opinionGivers[17]: 9,
		opinionGivers[19]: 11,
	}

	// provide custom rng with fixed seed, so that execution is deterministic
	opinionGiversToQuery, _ := fpc.ManaBasedSampling(opinionGivers, fpc.DefaultParameters().MaxQuerySampleSize, fpc.DefaultParameters().QuerySampleSize, rand.New(rand.NewSource(42)))
	sumVotes := 0
	for _, v := range opinionGiversToQuery {
		sumVotes += v
	}
	assert.Equal(t, fpc.DefaultParameters().MaxQuerySampleSize, sumVotes)
	assert.Equal(t, expectedOpinionGivers, opinionGiversToQuery)
}

func TestFPCVotingMultipleOpinionGiversWithMana(t *testing.T) {
	type testInput struct {
		id                  string
		minorityManaOpinion opinion.Opinion
		majorityManaOpinion opinion.Opinion
		expectedRoundsDone  int
		expectedOpinion     opinion.Opinion
	}
	tests := []testInput{
		{"1", opinion.Like, opinion.Dislike, 5, opinion.Dislike},
		{"2", opinion.Dislike, opinion.Like, 5, opinion.Like},
	}

	for _, test := range tests {
		// majority of nodes has minority of mana
		// finalize with opinion of majority mana holders
		opinionGiverFunc := func() (givers []opinion.OpinionGiver, err error) {
			opinionGivers := make([]opinion.OpinionGiver, fpc.DefaultParameters().QuerySampleSize)
			for i := 0; i < len(opinionGivers); i++ {
				if i < 12 {
					opinionGivers[i] = &opiniongivermock{mana: float64(i * i), id: identity.GenerateIdentity().ID(), roundsReplies: []opinion.Opinions{{test.minorityManaOpinion}}}
				} else {
					opinionGivers[i] = &opiniongivermock{mana: float64(i * i), id: identity.GenerateIdentity().ID(), roundsReplies: []opinion.Opinions{{test.majorityManaOpinion}}}
				}
			}
			return opinionGivers, nil
		}

		ownWeightRetrieverFunc := func() (float64, error) {
			return 0, nil
		}

		paras := fpc.DefaultParameters()
		paras.TotalRoundsFinalization = 2
		paras.TotalRoundsCoolingOffPeriod = 2
		voter := fpc.New(opinionGiverFunc, ownWeightRetrieverFunc, paras)
		// set custom rng with fixed seed to make runs deterministic
		voter.SetOpinionGiverRng(rand.New(rand.NewSource(42)))
		var finalOpinion *opinion.Opinion
		voter.Events().Finalized.Attach(events.NewClosure(func(ev *vote.OpinionEvent) {
			finalOpinion = &ev.Opinion
		}))

		assert.NoError(t, voter.Vote(test.id, vote.ConflictType, test.minorityManaOpinion))

		var roundsDone int
		for finalOpinion == nil {
			assert.NoError(t, voter.Round(0.7))
			roundsDone++
		}

		assert.Equal(t, test.expectedRoundsDone, roundsDone)
		require.NotNil(t, finalOpinion)
		assert.Equal(t, test.expectedOpinion, *finalOpinion)
	}
}
