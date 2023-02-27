package epoch

import (
	"time"

	"github.com/iotaledger/hive.go/runtime/options"
)

// TimeProvider defines the genesis time of epoch 0 and allows to convert index to and from time.
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
func (e *TimeProvider) GenesisUnixTime() int64 {
	return e.genesisUnixTime
}

// GenesisTime is the time  of the genesis.
func (e *TimeProvider) GenesisTime() time.Time {
	return time.Unix(e.genesisUnixTime, 0)
}

// Duration is the epoch duration in seconds.
func (e *TimeProvider) Duration() int64 {
	return e.duration
}

// IndexFromTime calculates the Index from the given time.
//
// Note: Epochs are counted starting from 1 because 0 is reserved for the genesis which has to be addressable as its own
// epoch as part of the commitment chains.
func (e *TimeProvider) IndexFromTime(t time.Time) Index {
	elapsedSeconds := t.Unix() - e.genesisUnixTime
	if elapsedSeconds < 0 {
		return 0
	}

	return Index(elapsedSeconds/e.duration + 1)
}

// StartTime calculates the start time of the given epoch.
func (e *TimeProvider) StartTime(i Index) time.Time {
	startUnix := e.genesisUnixTime + int64(i-1)*e.duration
	return time.Unix(startUnix, 0)
}

// EndTime returns the latest possible timestamp for an Epoch. Anything with higher timestamp will belong to the next epoch.
func (e *TimeProvider) EndTime(i Index) time.Time {
	endUnix := e.genesisUnixTime + int64(i)*e.duration
	// we subtract 1 nanosecond from the next epoch to get the latest possible timestamp for epoch i
	return time.Unix(endUnix, 0).Add(-1)
}

// WithGenesisUnixTime allows to set the genesis time to use.
func WithGenesisUnixTime(genesisTime int64) options.Option[TimeProvider] {
	return func(e *TimeProvider) {
		e.genesisUnixTime = genesisTime
	}
}

// WithEpochDuration allows to set the duration in seconds of each epoch. Defaults to 10.
func WithEpochDuration(duration int64) options.Option[TimeProvider] {
	return func(e *TimeProvider) {
		e.duration = duration
	}
}
