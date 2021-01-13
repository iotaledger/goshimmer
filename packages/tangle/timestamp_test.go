package tangle

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/magiconair/properties/assert"
)

func TestTimestampQuality(t *testing.T) {
	TimestampWindow = 1 * time.Minute
	GratuitousNetworkDelay = 15 * time.Second

	current := time.Now()

	// Testing Like
	issuedTime := current.Add(-200 * time.Millisecond)
	o := TimestampQuality(issuedTime, current)
	assert.Equal(t, o, TimestampOpinion{opinion.Like, Three})

	issuedTime = current.Add(200 * time.Millisecond)
	o = TimestampQuality(issuedTime, current)
	assert.Equal(t, o, TimestampOpinion{opinion.Like, Three})

	issuedTime = current.Add(-(2 * GratuitousNetworkDelay))
	o = TimestampQuality(issuedTime, current)
	assert.Equal(t, o, TimestampOpinion{opinion.Like, Two})

	issuedTime = current.Add((2 * GratuitousNetworkDelay))
	o = TimestampQuality(issuedTime, current)
	assert.Equal(t, o, TimestampOpinion{opinion.Like, Two})

	issuedTime = current.Add(-(3 * GratuitousNetworkDelay))
	o = TimestampQuality(issuedTime, current)
	assert.Equal(t, o, TimestampOpinion{opinion.Like, One})

	issuedTime = current.Add((3 * GratuitousNetworkDelay))
	o = TimestampQuality(issuedTime, current)
	assert.Equal(t, o, TimestampOpinion{opinion.Like, One})

	//Testing Dislike
	issuedTime = current.Add(-200*time.Millisecond - TimestampWindow)
	o = TimestampQuality(issuedTime, current)
	assert.Equal(t, o, TimestampOpinion{opinion.Dislike, One})

	issuedTime = current.Add(200*time.Millisecond + TimestampWindow)
	o = TimestampQuality(issuedTime, current)
	assert.Equal(t, o, TimestampOpinion{opinion.Dislike, One})

	issuedTime = current.Add(-(GratuitousNetworkDelay) - TimestampWindow)
	o = TimestampQuality(issuedTime, current)
	assert.Equal(t, o, TimestampOpinion{opinion.Dislike, Two})

	issuedTime = current.Add(GratuitousNetworkDelay + TimestampWindow)
	o = TimestampQuality(issuedTime, current)
	assert.Equal(t, o, TimestampOpinion{opinion.Dislike, Two})

	issuedTime = current.Add(-(2 * GratuitousNetworkDelay) - TimestampWindow)
	o = TimestampQuality(issuedTime, current)
	assert.Equal(t, o, TimestampOpinion{opinion.Dislike, Three})

	issuedTime = current.Add((2 * GratuitousNetworkDelay) + TimestampWindow)
	o = TimestampQuality(issuedTime, current)
	assert.Equal(t, o, TimestampOpinion{opinion.Dislike, Three})
}
