package metrics

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/magiconair/properties/assert"
)

func TestActiveConflicts(t *testing.T) {
	// initialized to 0
	assert.Equal(t, ActiveConflicts(), (uint64)(0))
	stats := &vote.RoundStats{
		ActiveVoteContexts: map[string]*vote.Context{
			"test1": {},
			"test2": {},
			"test3": {},
		},
	}
	processRoundStats(stats)
	assert.Equal(t, ActiveConflicts(), (uint64)(3))
}

func TestFailedConflicts(t *testing.T) {
	// initialized to 0
	assert.Equal(t, FailedConflicts(), (uint64)(0))
	// simulate 10 failed conflicts
	for i := 0; i < 10; i++ {
		processFailed(vote.Context{})
	}
	assert.Equal(t, FailedConflicts(), (uint64)(10))
	// simulate 10 failed conflicts
	for i := 0; i < 10; i++ {
		processFailed(vote.Context{})
	}
	assert.Equal(t, FailedConflicts(), (uint64)(20))
}

func TestFinalize(t *testing.T) {
	assert.Equal(t, AverageRoundsToFinalize(), 0.0)
	assert.Equal(t, FinalizedConflict(), (uint64)(0))
	// simulate 5 finalized conflicts with 5 rounds
	for i := 0; i < 5; i++ {
		processFinalized(vote.Context{
			Rounds: 5,
		})
	}
	assert.Equal(t, FinalizedConflict(), (uint64)(5))
	// simulate 5 finalized conflicts with 10 rounds
	for i := 0; i < 5; i++ {
		processFinalized(vote.Context{
			Rounds: 10,
		})
	}
	assert.Equal(t, FinalizedConflict(), (uint64)(10))
	// => average should be 7.5
	assert.Equal(t, AverageRoundsToFinalize(), 7.5)
}

func TestQueryReceived(t *testing.T) {
	assert.Equal(t, FPCQueryReceived(), (uint64)(0))
	assert.Equal(t, FPCOpinionQueryReceived(), (uint64)(0))

	processQueryReceived(&metrics.QueryReceivedEvent{OpinionCount: 5})

	assert.Equal(t, FPCQueryReceived(), (uint64)(1))
	assert.Equal(t, FPCOpinionQueryReceived(), (uint64)(5))

	processQueryReceived(&metrics.QueryReceivedEvent{OpinionCount: 5})

	assert.Equal(t, FPCQueryReceived(), (uint64)(2))
	assert.Equal(t, FPCOpinionQueryReceived(), (uint64)(10))
}

func TestQueryReplyError(t *testing.T) {
	assert.Equal(t, FPCQueryReplyErrors(), (uint64)(0))
	assert.Equal(t, FPCOpinionQueryReplyErrors(), (uint64)(0))

	processQueryReplyError(&metrics.QueryReplyErrorEvent{OpinionCount: 5})

	assert.Equal(t, FPCQueryReplyErrors(), (uint64)(1))
	assert.Equal(t, FPCOpinionQueryReplyErrors(), (uint64)(5))

	processQueryReplyError(&metrics.QueryReplyErrorEvent{OpinionCount: 5})

	assert.Equal(t, FPCQueryReplyErrors(), (uint64)(2))
	assert.Equal(t, FPCOpinionQueryReplyErrors(), (uint64)(10))
}
