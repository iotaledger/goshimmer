package fcob

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/tangle"
)

var (
	// TimestampWindow defines the time window for assessing the timestamp quality.
	TimestampWindow time.Duration

	// GratuitousNetworkDelay defines the time after which we assume all messages are delivered.
	GratuitousNetworkDelay time.Duration
)

// region TimestampQuality /////////////////////////////////////////////////////////////////////////////////////////////

// TimestampQuality returns the TimestampOpinion based on the given times (e.g., arrival and current).
func TimestampQuality(messageID tangle.MessageID, target, current time.Time) (timestampOpinion *TimestampOpinion, err error) {
	timestampOpinion = &TimestampOpinion{
		MessageID: messageID,
	}
	timestampOpinion.SetLiked(true)

	diff := current.Sub(target)

	// timestamp is in the future. This point in the code should only be reached in case of edge cases such as updating sync clock.
	if diff < 0 {
		timestampOpinion.SetLiked(true)
		timestampOpinion.SetLevelOfKnowledge(Three)
		// This point in the code should not be reached
		// err = xerrors.Errorf("Timestamp is in the future : %w", err, cerrors.ErrFatal)
		return
	}

	if diff >= TimestampWindow {
		timestampOpinion.SetLiked(false)
	}

	switch {
	case abs(diff-TimestampWindow) < GratuitousNetworkDelay:
		timestampOpinion.SetLevelOfKnowledge(One)
	case abs(diff-TimestampWindow) < 2*GratuitousNetworkDelay:
		timestampOpinion.SetLevelOfKnowledge(Two)
	default:
		timestampOpinion.SetLevelOfKnowledge(Three)
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
