package tangle

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/vote/opinion"
)

// LevelOfKnowledge defines the Leve lOf Knowledge type.
type LevelOfKnowledge uint8

// The different levels of knowledge.
const (
	// One implies that voting is required.
	One LevelOfKnowledge = iota + 1
	// Two implies that we have finalized our opinion but we can still reply to eventual queries.
	Two
	// Three implies that we have finalized our opinion and we do not reply to eventual queries.
	Three
)

var (
	// TimestampWindow defines the time window for assessing the timestamp quality.
	TimestampWindow time.Duration
	// GratuitousNetworkDelay defines the time after which we assume all messages are delivered.
	GratuitousNetworkDelay time.Duration
)

// TimestampOpinion contains the value of a timestamp opinion as well as its level of knowledge.
type TimestampOpinion struct {
	Value opinion.Opinion
	LoK   LevelOfKnowledge
}

// TimestampQuality returns the timestamp opinion based on the given times (e.g., arrival and current).
func TimestampQuality(target, current time.Time) (timestampOpinion TimestampOpinion) {
	diff := abs(current.Sub(target))

	timestampOpinion.Value = opinion.Like
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
