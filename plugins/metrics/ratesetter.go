package metrics

import (
	"time"
)

var (
	// schedulerRate rate at which messages are scheduled.
	ownRate float64

	// readyMessagesCount number of ready messages in the scheduler buffer.
	rateSetterBufferSize int

	// totalMessagesCount number of  messages in the scheduler buffer.
	rateSetterEstimate time.Duration
)

func measureRateSetter() {
	rateSetter := deps.Tangle.RateSetter
	Events.RateSetterUpdated.Trigger(RateSetterMetric{
		Size:     rateSetter.Size(),
		Estimate: rateSetter.Estimate(),
		Rate:     rateSetter.Rate(),
	})
	ownRate = rateSetter.Rate()
	rateSetterBufferSize = rateSetter.Size()
	rateSetterEstimate = rateSetter.Estimate()
}

func OwnRate() float64 {
	return ownRate
}

// RateSetterBufferSize number of ready messages in the rate setter buffer.
func RateSetterBufferSize() int {
	return rateSetterBufferSize
}

// RateSetterEstimate returns the maximum buffer size.
func RateSetterEstimate() int64 {
	return rateSetterEstimate.Milliseconds()
}
