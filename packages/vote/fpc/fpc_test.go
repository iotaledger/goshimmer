package fpc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/packages/vote/fpc"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVoteContext_IsFinalized(t *testing.T) {
	type testInput struct {
		voteCtx               vote.Context
		coolOffPeriod         int
		finalizationThreshold int
		want                  bool
	}
	var tests = []testInput{
		{vote.Context{
			Opinions: []opinion.Opinion{opinion.Like, opinion.Like, opinion.Like, opinion.Like, opinion.Like},
		}, 2, 2, true},
		{vote.Context{
			Opinions: []opinion.Opinion{opinion.Like, opinion.Like, opinion.Like, opinion.Like, opinion.Dislike},
		}, 2, 2, false},
	}

	for _, test := range tests {
		assert.Equal(t, test.want, test.voteCtx.IsFinalized(test.coolOffPeriod, test.finalizationThreshold))
	}
}

func TestVoteContext_LastOpinion(t *testing.T) {
	type testInput struct {
		voteCtx  vote.Context
		expected opinion.Opinion
	}
	var tests = []testInput{
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
	voter := fpc.New(nil)
	assert.NoError(t, voter.Vote("a", vote.ConflictType, opinion.Like))
	// can't add the same item twice
	assert.True(t, errors.Is(voter.Vote("a", vote.ConflictType, opinion.Like), fpc.ErrVoteAlreadyOngoing))
}

type opiniongivermock struct {
	roundsReplies []opinion.Opinions
	roundIndex    int
}

func (ogm *opiniongivermock) ID() identity.ID {
	return identity.GenerateIdentity().ID()
}

func (ogm *opiniongivermock) Query(_ context.Context, _ []string, _ []string) (opinion.Opinions, error) {
	if ogm.roundIndex >= len(ogm.roundsReplies) {
		return ogm.roundsReplies[len(ogm.roundsReplies)-1], nil
	}
	opinions := ogm.roundsReplies[ogm.roundIndex]
	ogm.roundIndex++
	return opinions, nil
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

	id := "a"

	paras := fpc.DefaultParameters()
	paras.FinalizationThreshold = 2
	paras.CoolingOffPeriod = 2
	paras.QuerySampleSize = 1
	voter := fpc.New(opinionGiverFunc, paras)
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

	id := "a"

	paras := fpc.DefaultParameters()
	paras.QuerySampleSize = 1
	paras.MaxRoundsPerVoteContext = 3
	paras.CoolingOffPeriod = 0
	// since the finalization threshold is over max rounds it will
	// always fail finalizing an opinion
	paras.FinalizationThreshold = 4
	voter := fpc.New(opinionGiverFunc, paras)
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
	var tests = []testInput{
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

		paras := fpc.DefaultParameters()
		paras.FinalizationThreshold = 2
		paras.CoolingOffPeriod = 2
		voter := fpc.New(opinionGiverFunc, paras)
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
