package fcob

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
)

func TestTimestampQuality(t *testing.T) {
	TimestampWindow = 1 * time.Minute
	GratuitousNetworkDelay = 15 * time.Second

	offset := 200 * time.Millisecond

	current := time.Now()

	// Testing Future
	issuedTime := current.Add(offset)
	o, _ := TimestampQuality(tangle.EmptyMessageID, issuedTime, current)
	assert.True(t, o.Equals(&TimestampOpinion{MessageID: tangle.EmptyMessageID, Value: opinion.Like, LoK: Three}))

	// Testing Like
	issuedTime = current.Add(-offset)
	o, _ = TimestampQuality(tangle.EmptyMessageID, issuedTime, current)
	assert.True(t, o.Equals(&TimestampOpinion{MessageID: tangle.EmptyMessageID, Value: opinion.Like, LoK: Three}))

	issuedTime = current.Add(-offset - (2 * GratuitousNetworkDelay))
	o, _ = TimestampQuality(tangle.EmptyMessageID, issuedTime, current)
	assert.True(t, o.Equals(&TimestampOpinion{MessageID: tangle.EmptyMessageID, Value: opinion.Like, LoK: Two}))

	issuedTime = current.Add(-offset - (3 * GratuitousNetworkDelay))
	o, _ = TimestampQuality(tangle.EmptyMessageID, issuedTime, current)
	assert.True(t, o.Equals(&TimestampOpinion{MessageID: tangle.EmptyMessageID, Value: opinion.Like, LoK: One}))

	// Testing Dislike
	issuedTime = current.Add(-offset - TimestampWindow)
	o, _ = TimestampQuality(tangle.EmptyMessageID, issuedTime, current)
	assert.True(t, o.Equals(&TimestampOpinion{MessageID: tangle.EmptyMessageID, Value: opinion.Dislike, LoK: One}))

	issuedTime = current.Add(-(GratuitousNetworkDelay) - TimestampWindow)
	o, _ = TimestampQuality(tangle.EmptyMessageID, issuedTime, current)
	assert.True(t, o.Equals(&TimestampOpinion{MessageID: tangle.EmptyMessageID, Value: opinion.Dislike, LoK: Two}))

	issuedTime = current.Add(-(2 * GratuitousNetworkDelay) - TimestampWindow)
	o, _ = TimestampQuality(tangle.EmptyMessageID, issuedTime, current)
	assert.True(t, o.Equals(&TimestampOpinion{MessageID: tangle.EmptyMessageID, Value: opinion.Dislike, LoK: Three}))

}
