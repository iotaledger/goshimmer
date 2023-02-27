package slot

import (
	"time"

	"github.com/iotaledger/hive.go/runtime/options"
)

// TimeProvider defines the genesis time of slot 0 and allows to convert index to and from time.
type TimeProvider struct {
	// genesisUnixTime is the time (Unix in seconds) of the genesis.
	genesisUnixTime int64

	// duration is the default epoch duration in seconds.
	duration int64
}

// NewTimeProvider creates a new time provider.
func NewTimeProvider(opts ...options.Option[TimeProvider]) *TimeProvider {
	return options.Apply(&TimeProvider{
		genesisUnixTime: 1666037700,
		duration:        10,
	}, opts)
}

// GenesisUnixTime is the time (Unix in seconds) of the genesis.
func (t *TimeProvider) GenesisUnixTime() int64 {
	return t.genesisUnixTime
}

// GenesisTime is the time  of the genesis.
func (t *TimeProvider) GenesisTime() time.Time {
	return time.Unix(t.genesisUnixTime, 0)
}

// Duration is the epoch duration in seconds.
func (t *TimeProvider) Duration() int64 {
	return t.duration
}

// IndexFromTime calculates the Index from the given time.
//
// Note: slots are counted starting from 1 because 0 is reserved for the genesis which has to be addressable as its own
// slot as part of the commitment chains.
func (t *TimeProvider) IndexFromTime(time time.Time) Index {
	elapsedSeconds := time.Unix() - t.genesisUnixTime
	if elapsedSeconds < 0 {
		return 0
	}

	return Index(elapsedSeconds/t.duration + 1)
}

// StartTime calculates the start time of the given slot.
func (t *TimeProvider) StartTime(i Index) time.Time {
	startUnix := t.genesisUnixTime + int64(i-1)*t.duration
	return time.Unix(startUnix, 0)
}

// EndTime returns the latest possible timestamp for a slot. Anything with higher timestamp will belong to the next slot.
func (t *TimeProvider) EndTime(i Index) time.Time {
	endUnix := t.genesisUnixTime + int64(i)*t.duration
	// we subtract 1 nanosecond from the next epoch to get the latest possible timestamp for epoch i
	return time.Unix(endUnix, 0).Add(-1)
}

// WithGenesisUnixTime allows to set the genesis time to use.
func WithGenesisUnixTime(genesisTime int64) options.Option[TimeProvider] {
	return func(e *TimeProvider) {
		e.genesisUnixTime = genesisTime
	}
}

// WithSlotDuration allows to set the duration in seconds of each slot. Defaults to 10.
func WithSlotDuration(duration int64) options.Option[TimeProvider] {
	return func(e *TimeProvider) {
		e.duration = duration
	}
}
