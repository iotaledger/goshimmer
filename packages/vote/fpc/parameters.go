package fpc

import "time"

// Parameters define the parameters of an FPC instance.
type Parameters struct {
	// The lower bound liked percentage threshold at the first round. Also called 'a'.
	FirstRoundLowerBoundThreshold float64
	// The upper bound liked percentage threshold at the first round. Also called 'b'.
	FirstRoundUpperBoundThreshold float64
	// The liked percentage threshold used after the first round. Also called 'beta'.
	SubsequentRoundsLowerBoundThreshold float64
	// The amount of opinions to query on each round for a given vote context. Also called 'k'.
	QuerySampleSize int
	// The amount of rounds a vote context's opinion needs to stay the same to be considered final. Also called 'l'.
	FinalizationThreshold int
	// The amount of rounds for which to ignore any finalization checks for. Also called 'm'.
	CoolingOffPeriod int
	// The max amount of rounds to execute per vote context before aborting them.
	MaxRoundsPerVoteContext int
	// The max amount of time a query is allowed to take.
	QueryTimeout time.Duration
}

// DefaultParameters returns the default parameters used in FPC.
func DefaultParameters() *Parameters {
	return &Parameters{
		FirstRoundLowerBoundThreshold:       0.75,
		FirstRoundUpperBoundThreshold:       0.85,
		SubsequentRoundsLowerBoundThreshold: 0.33,
		QuerySampleSize:                     10,
		FinalizationThreshold:               2,
		CoolingOffPeriod:                    2,
		MaxRoundsPerVoteContext:             100,
		QueryTimeout:                        1500 * time.Millisecond,
	}
}

// RandUniformThreshold returns random threshold between the given lower/upper bound values.
func RandUniformThreshold(rand float64, thresholdLowerBound float64, thresholdUpperBound float64) float64 {
	return thresholdLowerBound + rand*(thresholdUpperBound-thresholdLowerBound)
}
