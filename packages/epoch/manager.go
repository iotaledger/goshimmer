package epoch

import (
	"github.com/iotaledger/goshimmer/packages/clock"
	"time"
)

const (
	// DefaultGenesisTime is the default time (Unix in seconds) of the genesis, i.e., the start of the epochs at 2022-04-27 9:00:00 UTC.
	DefaultGenesisTime int64 = 1655100550

	// defaultDuration is the default epoch duration, and it is 5 minutes (specified in seconds).
	defaultDuration time.Duration = 10 * time.Second
)

// region EpochManager ////////////////////////////////////////////////////////////////////////////////////

// Manager is responsible for time/EI conversions.
type Manager struct {
	options *ManagerOptions
}

// NewManager is the constructor of the EpochManager that takes a KVStore to persist its state.
func NewManager(opts ...ManagerOption) *Manager {
	options := &ManagerOptions{
		GenesisTime: DefaultGenesisTime,
		Duration:    defaultDuration,
	}

	for _, option := range opts {
		option(options)
	}

	return &Manager{
		options: options,
	}
}

// TimeToEI calculates the EI for the given time.
func (m *Manager) TimeToEI(t time.Time) (ei Index) {
	elapsedSeconds := t.Unix() - m.options.GenesisTime
	if elapsedSeconds <= 0 {
		return 0
	}

	return Index(elapsedSeconds / int64(m.options.Duration.Seconds()))
}

// EIToStartTime calculates the start time of the given epoch.
func (m *Manager) EIToStartTime(ei Index) time.Time {
	startUnix := m.options.GenesisTime + int64(ei)*int64(m.options.Duration.Seconds())
	return time.Unix(startUnix, 0)
}

// EIToEndTime calculates the end time of the given epoch.
func (m *Manager) EIToEndTime(ei Index) time.Time {
	endUnix := m.options.GenesisTime + int64(ei)*int64(m.options.Duration.Seconds()) + int64(m.options.Duration.Seconds()) - 1
	return time.Unix(endUnix, 0)
}

// CurrentEI returns the EI at the current synced time.
func (m *Manager) CurrentEI() Index {
	return m.TimeToEI(clock.SyncedTime())
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ManagerOptions ///////////////////////////////////////////////////////////////////////////////////////////////

// ManagerOption represents the return type of optional parameters that can be handed into the constructor of the
// Manager to configure its behavior.
type ManagerOption func(options *ManagerOptions)

// ManagerOptions is a container for all configurable parameters of the EpochManager.
type ManagerOptions struct {
	GenesisTime      int64
	Duration         time.Duration
	OracleEpochShift Index
}

// GenesisTime is a EpochManagerOption that allows to define the time of the genesis, i.e., the start of the epochs,
// specified in Unix time (seconds).
func GenesisTime(genesisTime int64) ManagerOption {
	return func(options *ManagerOptions) {
		options.GenesisTime = genesisTime
	}
}

// Duration is a EpochManagerOption that allows to define the epoch interval, i.e., the duration of each Epoch, specified
// in seconds.
func Duration(duration time.Duration) ManagerOption {
	return func(options *ManagerOptions) {
		options.Duration = duration
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
