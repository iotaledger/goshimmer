package notarization

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/clock"
)

const (
	// DefaultGenesisTime is the default time (Unix in seconds) of the genesis, i.e., the start of the epochs at 2022-04-27 9:00:00 UTC.
	DefaultGenesisTime int64 = 1651050000

	// defaultInterval is the default interval of epochs, i.e., their duration, and is 5 minutes (specified in seconds).
	defaultInterval int64 = 5 * 60

	// defaultOracleEpochShift is the default shift of the oracle epoch. E.g., current epoch=4 -> oracle epoch=2.
	defaultOracleEpochShift = 2
)

// region EpochManager ////////////////////////////////////////////////////////////////////////////////////

// EpochManager is responsible for time/ECI conversions.
type EpochManager struct {
	options *EpochManagerOptions
}

// NewEpochManager is the constructor of the EpochManager that takes a KVStore to persist its state.
func NewEpochManager(opts ...EpochManagerOption) *EpochManager {
	options := &EpochManagerOptions{
		GenesisTime:      DefaultGenesisTime,
		Interval:         defaultInterval,
		OracleEpochShift: defaultOracleEpochShift,
	}

	for _, option := range opts {
		option(options)
	}

	return &EpochManager{
		options: options,
	}
}

// TimeToECI calculates the ECI for the given time.
func (m *EpochManager) TimeToECI(t time.Time) (eci ECI) {
	elapsedSeconds := t.Unix() - m.options.GenesisTime
	if elapsedSeconds <= 0 {
		return 0
	}

	return ECI(elapsedSeconds / m.options.Interval)
}

// TimeToOracleECI calculates the oracle ECI for the given time.
func (m *EpochManager) TimeToOracleECI(t time.Time) (oracleECI ECI) {
	eci := m.TimeToECI(t)
	oracleECI = eci - m.options.OracleEpochShift

	// default to ECI 0 if oracle epoch < shift as it is not defined
	if eci < m.options.OracleEpochShift || oracleECI < m.options.OracleEpochShift {
		return 0
	}

	return
}

// ECIToStartTime calculates the start time of the given epoch.
func (m *EpochManager) ECIToStartTime(eci ECI) time.Time {
	startUnix := m.options.GenesisTime + int64(eci)*m.options.Interval
	return time.Unix(startUnix, 0)
}

// ECIToEndTime calculates the end time of the given epoch.
func (m *EpochManager) ECIToEndTime(eci ECI) time.Time {
	endUnix := m.options.GenesisTime + int64(eci)*m.options.Interval + m.options.Interval - 1
	return time.Unix(endUnix, 0)
}

// CurrentECI returns the ECI at the current synced time.
func (m *EpochManager) CurrentECI() ECI {
	return m.TimeToECI(clock.SyncedTime())
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region EpochManagerOptions ///////////////////////////////////////////////////////////////////////////////////////////////

// EpochManagerOption represents the return type of optional parameters that can be handed into the constructor of the
// Manager to configure its behavior.
type EpochManagerOption func(options *EpochManagerOptions)

// EpochManagerOptions is a container for all configurable parameters of the EpochManager.
type EpochManagerOptions struct {
	GenesisTime      int64
	Interval         int64
	OracleEpochShift ECI
}

// GenesisTime is a EpochManagerOption that allows to define the time of the genesis, i.e., the start of the epochs,
// specified in Unix time (seconds).
func GenesisTime(genesisTime int64) EpochManagerOption {
	return func(options *EpochManagerOptions) {
		options.GenesisTime = genesisTime
	}
}

// Interval is a EpochManagerOption that allows to define the epoch interval, i.e., the duration of each Epoch, specified
// in seconds.
func Interval(interval int64) EpochManagerOption {
	return func(options *EpochManagerOptions) {
		options.Interval = interval
	}
}

// OracleEpochShift is a EpochManagerOption that allows to define the shift of the oracle epoch.
func OracleEpochShift(shift int) EpochManagerOption {
	return func(options *EpochManagerOptions) {
		options.OracleEpochShift = ECI(shift)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
