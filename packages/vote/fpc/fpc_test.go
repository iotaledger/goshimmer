package fpc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/packages/vote/fpc"
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
			Opinions: []vote.Opinion{vote.Like, vote.Like, vote.Like, vote.Like, vote.Like},
		}, 2, 2, true},
		{vote.Context{
			Opinions: []vote.Opinion{vote.Like, vote.Like, vote.Like, vote.Like, vote.Dislike},
		}, 2, 2, false},
	}

	for _, test := range tests {
		assert.Equal(t, test.want, test.voteCtx.IsFinalized(test.coolOffPeriod, test.finalizationThreshold))
	}
}

func TestVoteContext_LastOpinion(t *testing.T) {
	type testInput struct {
		voteCtx  vote.Context
		expected vote.Opinion
	}
	var tests = []testInput{
		{vote.Context{
			Opinions: []vote.Opinion{vote.Like, vote.Like, vote.Like, vote.Like},
		}, vote.Like},
		{vote.Context{
			Opinions: []vote.Opinion{vote.Like, vote.Like, vote.Like, vote.Dislike},
		}, vote.Dislike},
	}

	for _, test := range tests {
		assert.Equal(t, test.expected, test.voteCtx.LastOpinion())
	}
}

func TestFPCPreventSameIDMultipleTimes(t *testing.T) {
	voter := fpc.New(nil)
	assert.NoError(t, voter.Vote("a", vote.ConflictType, vote.Like))
	// can't add the same item twice
	assert.True(t, errors.Is(voter.Vote("a", vote.ConflictType, vote.Like), fpc.ErrVoteAlreadyOngoing))
}

type opiniongivermock struct {
	roundsReplies []vote.Opinions
	roundIndex    int
}

func (ogm *opiniongivermock) ID() identity.ID {
	return identity.GenerateIdentity().ID()
}

func (ogm *opiniongivermock) Query(_ context.Context, _ []string, _ []string) (vote.Opinions, error) {
	if ogm.roundIndex >= len(ogm.roundsReplies) {
		return ogm.roundsReplies[len(ogm.roundsReplies)-1], nil
	}
	opinions := ogm.roundsReplies[ogm.roundIndex]
	ogm.roundIndex++
	return opinions, nil
}

func TestFPCFinalizedEvent(t *testing.T) {
	opinionGiverMock := &opiniongivermock{
		roundsReplies: []vote.Opinions{
			// 2 cool-off period, 2 finalization threshold
			{vote.Like}, {vote.Like}, {vote.Like}, {vote.Like},
		},
	}
	opinionGiverFunc := func() (givers []vote.OpinionGiver, err error) {
		return []vote.OpinionGiver{opinionGiverMock}, nil
	}

	id := "a"

	paras := fpc.DefaultParameters()
	paras.FinalizationThreshold = 2
	paras.CoolingOffPeriod = 2
	paras.QuerySampleSize = 1
	voter := fpc.New(opinionGiverFunc, paras)
	var finalizedOpinion *vote.Opinion
	voter.Events().Finalized.Attach(events.NewClosure(func(ev *vote.OpinionEvent) {
		finalizedOpinion = &ev.Opinion
	}))
	assert.NoError(t, voter.Vote(id, vote.ConflictType, vote.Like))

	// do 5 rounds of FPC -> 5 because the last one finalizes the vote
	for i := 0; i < 5; i++ {
		assert.NoError(t, voter.Round(0.5))
	}

	require.NotNil(t, finalizedOpinion, "finalized event should have been fired")
	assert.Equal(t, vote.Like, *finalizedOpinion, "the final opinion should have been 'Like'")
}

func TestFPCFailedEvent(t *testing.T) {
	opinionGiverFunc := func() (givers []vote.OpinionGiver, err error) {
		return []vote.OpinionGiver{&opiniongivermock{
			// doesn't matter what we set here
			roundsReplies: []vote.Opinions{{vote.Dislike}},
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
	var failedOpinion *vote.Opinion
	voter.Events().Failed.Attach(events.NewClosure(func(ev *vote.OpinionEvent) {
		failedOpinion = &ev.Opinion
	}))
	assert.NoError(t, voter.Vote(id, vote.ConflictType, vote.Like))

	for i := 0; i < 4; i++ {
		assert.NoError(t, voter.Round(0.5))
	}

	require.NotNil(t, failedOpinion, "failed event should have been fired")
	assert.Equal(t, vote.Dislike, *failedOpinion, "the final opinion should have been 'Dislike'")
}

func TestFPCVotingMultipleOpinionGivers(t *testing.T) {
	type testInput struct {
		id                 string
		initOpinion        vote.Opinion
		expectedRoundsDone int
		expectedOpinion    vote.Opinion
	}
	var tests = []testInput{
		{"1", vote.Like, 5, vote.Like},
		{"2", vote.Dislike, 5, vote.Dislike},
	}

	for _, test := range tests {
		// note that even though we're defining QuerySampleSize times opinion givers,
		// it doesn't mean that FPC will query all of them.
		opinionGiverFunc := func() (givers []vote.OpinionGiver, err error) {
			opinionGivers := make([]vote.OpinionGiver, fpc.DefaultParameters().QuerySampleSize)
			for i := 0; i < len(opinionGivers); i++ {
				opinionGivers[i] = &opiniongivermock{roundsReplies: []vote.Opinions{{test.initOpinion}}}
			}
			return opinionGivers, nil
		}

		paras := fpc.DefaultParameters()
		paras.FinalizationThreshold = 2
		paras.CoolingOffPeriod = 2
		voter := fpc.New(opinionGiverFunc, paras)
		var finalOpinion *vote.Opinion
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
