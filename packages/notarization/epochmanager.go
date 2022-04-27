package notarization

import (
	"time"
)

const (
	// DefaultGenesisTime is the default time (Unix in seconds) of the genesis, i.e., the start of the epochs at 2022-04-27 9:00:00 UTC.
	DefaultGenesisTime int64 = 1651050000

	// defaultInterval is the default interval of epochs, i.e., their duration, and is 5 minutes (specified in seconds).
	defaultInterval int64 = 5 * 60

	// defaultOracleEpochShift is the default shift of the oracle epoch. E.g., current epoch=4 -> oracle epoch=2
	defaultOracleEpochShift = 2
)

// region EpochManager ////////////////////////////////////////////////////////////////////////////////////

// EpochManager is responsible for time/ECI conversions.
type EpochManager struct {
	GenesisTime      int64
	Interval         int64
	OracleEpochShift ECI
}

// NewEpochManager is the constructor of the EpochManager that takes a KVStore to persist its state.
func NewEpochManager() *EpochManager {
	return &EpochManager{
		GenesisTime:      DefaultGenesisTime,
		Interval:         defaultInterval,
		OracleEpochShift: defaultOracleEpochShift,
	}
}

// TimeToECI calculates the ECI for the given time.
func (m *EpochManager) TimeToECI(t time.Time) (eci ECI) {
	elapsedSeconds := t.Unix() - m.GenesisTime
	if elapsedSeconds <= 0 {
		return 0
	}

	return ECI(elapsedSeconds / m.Interval)
}

// TimeToOracleECI calculates the oracle ECI for the given time.
func (m *EpochManager) TimeToOracleECI(t time.Time) (oracleECI ECI) {
	eci := m.TimeToECI(t)
	oracleECI = eci - m.OracleEpochShift

	// default to ECI 0 if oracle epoch < shift as it is not defined
	if eci < m.OracleEpochShift || oracleECI < m.OracleEpochShift {
		return 0
	}

	return
}

// ECIToStartTime calculates the start time of the given epoch.
func (m *EpochManager) ECIToStartTime(eci ECI) time.Time {
	startUnix := m.GenesisTime + int64(eci)*m.Interval
	return time.Unix(startUnix, 0)
}

// ECIToEndTime calculates the end time of the given epoch.
func (m *EpochManager) ECIToEndTime(eci ECI) time.Time {
	endUnix := m.GenesisTime + int64(eci)*m.Interval + m.Interval - 1
	return time.Unix(endUnix, 0)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
