package notarization

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/epoch"
)

const (
	// DefaultGenesisTime is the default time (Unix in seconds) of the genesis, i.e., the start of the epochs at 2022-04-27 9:00:00 UTC.
	DefaultGenesisTime int64 = 1655100550

	// defaultDuration is the default epoch duration, and it is 5 minutes (specified in seconds).
	defaultDuration time.Duration = 10 * time.Second
)

// region EpochManager ////////////////////////////////////////////////////////////////////////////////////

// EpochManager is responsible for time/EI conversions.
type EpochManager struct {
	options *EpochManagerOptions
}

// NewEpochManager is the constructor of the EpochManager that takes a KVStore to persist its state.
func NewEpochManager(opts ...EpochManagerOption) *EpochManager {
	options := &EpochManagerOptions{
		GenesisTime: DefaultGenesisTime,
		Duration:    defaultDuration,
	}

	for _, option := range opts {
		option(options)
	}

	return &EpochManager{
		options: options,
	}
}

// TimeToEI calculates the EI for the given time.
func (m *EpochManager) TimeToEI(t time.Time) (ei epoch.Index) {
	elapsedSeconds := t.Unix() - m.options.GenesisTime
	if elapsedSeconds <= 0 {
		return 0
	}

	return epoch.Index(elapsedSeconds / int64(m.options.Duration.Seconds()))
}

// EIToStartTime calculates the start time of the given epoch.
func (m *EpochManager) EIToStartTime(ei epoch.Index) time.Time {
	startUnix := m.options.GenesisTime + int64(ei)*int64(m.options.Duration.Seconds())
	return time.Unix(startUnix, 0)
}

// EIToEndTime calculates the end time of the given epoch.
func (m *EpochManager) EIToEndTime(ei epoch.Index) time.Time {
	endUnix := m.options.GenesisTime + int64(ei)*int64(m.options.Duration.Seconds()) + int64(m.options.Duration.Seconds()) - 1
	return time.Unix(endUnix, 0)
}

// CurrentEI returns the EI at the current synced time.
func (m *EpochManager) CurrentEI() epoch.Index {
	return m.TimeToEI(clock.SyncedTime())
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region EpochManagerOptions ///////////////////////////////////////////////////////////////////////////////////////////////

// EpochManagerOption represents the return type of optional parameters that can be handed into the constructor of the
// Manager to configure its behavior.
type EpochManagerOption func(options *EpochManagerOptions)

// EpochManagerOptions is a container for all configurable parameters of the EpochManager.
type EpochManagerOptions struct {
	GenesisTime      int64
	Duration         time.Duration
	OracleEpochShift epoch.Index
}

// GenesisTime is a EpochManagerOption that allows to define the time of the genesis, i.e., the start of the epochs,
// specified in Unix time (seconds).
func GenesisTime(genesisTime int64) EpochManagerOption {
	return func(options *EpochManagerOptions) {
		options.GenesisTime = genesisTime
	}
}

// Duration is a EpochManagerOption that allows to define the epoch interval, i.e., the duration of each Epoch, specified
// in seconds.
func Duration(duration time.Duration) EpochManagerOption {
	return func(options *EpochManagerOptions) {
		options.Duration = duration
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
