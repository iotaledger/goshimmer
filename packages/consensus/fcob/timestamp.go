package fcob

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
)

var (
	// TimestampWindow defines the time window for assessing the timestamp quality.
	TimestampWindow time.Duration

	// GratuitousNetworkDelay defines the time after which we assume all messages are delivered.
	GratuitousNetworkDelay time.Duration
)

// region TimestampQuality /////////////////////////////////////////////////////////////////////////////////////////////

// TimestampQuality returns the TimestampOpinion based on the given times (e.g., arrival and current).
func TimestampQuality(messageID tangle.MessageID, target, current time.Time) (timestampOpinion *TimestampOpinion) {
	diff := abs(current.Sub(target))

	timestampOpinion = &TimestampOpinion{
		MessageID: messageID,
		Value:     opinion.Like,
	}
	if diff >= TimestampWindow {
		timestampOpinion.Value = opinion.Dislike
	}

	switch {
	case abs(diff-TimestampWindow) < GratuitousNetworkDelay:
		timestampOpinion.LoK = One
	case abs(diff-TimestampWindow) < 2*GratuitousNetworkDelay:
		timestampOpinion.LoK = Two
	default:
		timestampOpinion.LoK = Three
	}

	return
}

func abs(a time.Duration) time.Duration {
	if a >= 0 {
		return a
	}
	return -a
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
