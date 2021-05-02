package fcob

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/tangle"
)

func TestTimestampQuality(t *testing.T) {
	TimestampWindow = 1 * time.Minute
	GratuitousNetworkDelay = 15 * time.Second

	offset := 200 * time.Millisecond

	current := time.Now()

	// Testing Future
	issuedTime := current.Add(offset)
	o, _ := TimestampQuality(tangle.EmptyMessageID, issuedTime, current)
	assert.True(t, o.Equals(&TimestampOpinion{MessageID: tangle.EmptyMessageID, OpinionEssence: OpinionEssence{levelOfKnowledge: Three, liked: true}}))

	// Testing Like
	issuedTime = current.Add(-offset)
	o, _ = TimestampQuality(tangle.EmptyMessageID, issuedTime, current)
	assert.True(t, o.Equals(&TimestampOpinion{MessageID: tangle.EmptyMessageID, OpinionEssence: OpinionEssence{levelOfKnowledge: Three, liked: true}}))

	issuedTime = current.Add(-offset - (2 * GratuitousNetworkDelay))
	o, _ = TimestampQuality(tangle.EmptyMessageID, issuedTime, current)
	assert.True(t, o.Equals(&TimestampOpinion{MessageID: tangle.EmptyMessageID, OpinionEssence: OpinionEssence{levelOfKnowledge: Two, liked: true}}))

	issuedTime = current.Add(-offset - (3 * GratuitousNetworkDelay))
	o, _ = TimestampQuality(tangle.EmptyMessageID, issuedTime, current)
	assert.True(t, o.Equals(&TimestampOpinion{MessageID: tangle.EmptyMessageID, OpinionEssence: OpinionEssence{levelOfKnowledge: One, liked: true}}))

	// Testing Dislike
	issuedTime = current.Add(-offset - TimestampWindow)
	o, _ = TimestampQuality(tangle.EmptyMessageID, issuedTime, current)
	assert.True(t, o.Equals(&TimestampOpinion{MessageID: tangle.EmptyMessageID, OpinionEssence: OpinionEssence{levelOfKnowledge: One, liked: false}}))

	issuedTime = current.Add(-(GratuitousNetworkDelay) - TimestampWindow)
	o, _ = TimestampQuality(tangle.EmptyMessageID, issuedTime, current)
	assert.True(t, o.Equals(&TimestampOpinion{MessageID: tangle.EmptyMessageID, OpinionEssence: OpinionEssence{levelOfKnowledge: Two, liked: false}}))

	issuedTime = current.Add(-(2 * GratuitousNetworkDelay) - TimestampWindow)
	o, _ = TimestampQuality(tangle.EmptyMessageID, issuedTime, current)
	assert.True(t, o.Equals(&TimestampOpinion{MessageID: tangle.EmptyMessageID, OpinionEssence: OpinionEssence{levelOfKnowledge: Three, liked: false}}))

}
