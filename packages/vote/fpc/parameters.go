package fpc

import "time"

// Parameters define the parameters of an FPC instance.
type Parameters struct {
	// The lower bound liked percentage threshold at the first round. Also called 'a'.
	FirstRoundLowerBoundThreshold float64
	// The upper bound liked percentage threshold at the first round. Also called 'b'.
	FirstRoundUpperBoundThreshold float64
	// The lower bound liked percentage threshold used after the first round.
	SubsequentRoundsLowerBoundThreshold float64
	// The upper bound liked percentage threshold used after the first round.
	SubsequentRoundsUpperBoundThreshold float64
	// The fixed liked percentage threshold used in last 'l2' rounds.
	EndingRoundsFixedThreshold float64
	// The amount of opinions to query on each round for a given vote context. Also called 'k'.
	QuerySampleSize int
	// The maximum amount of votes to collect on each round for a given vote context. Naive implementation of 'k_diff' from the paper.
	MaxQuerySampleSize int
	// The amount of rounds a vote context's opinion needs to stay the same to be considered final. Also called 'l'.
	TotalRoundsFinalization int
	// The amount of last rounds for the fixed threshold.
	TotalRoundsFixedThreshold int
	// The amount of rounds for which to ignore any finalization checks for. Also called 'm'.
	TotalRoundsCoolingOffPeriod int
	// The max amount of rounds to execute per vote context before aborting them.
	MaxRoundsPerVoteContext int
	// The max amount of time a query is allowed to take.
	QueryTimeout time.Duration
	// MinOpinionsReceived defines the minimum amount of opinions to receive in order to consider an FPC round valid.
	MinOpinionsReceived int
}

// DefaultParameters returns the default parameters used in FPC.
func DefaultParameters() *Parameters {
	p := &Parameters{
		FirstRoundLowerBoundThreshold:       0.67,
		FirstRoundUpperBoundThreshold:       0.67,
		SubsequentRoundsLowerBoundThreshold: 0.50,
		SubsequentRoundsUpperBoundThreshold: 0.67,
		EndingRoundsFixedThreshold:          0.50,
		QuerySampleSize:                     21,
		MaxQuerySampleSize:                  100,
		MinOpinionsReceived:                 1,
		TotalRoundsFinalization:             10,
		TotalRoundsFixedThreshold:           3,
		TotalRoundsCoolingOffPeriod:         0,
		MaxRoundsPerVoteContext:             100,
		QueryTimeout:                        1500 * time.Millisecond,
	}

	return p
}

// RandUniformThreshold returns random threshold between the given lower/upper bound values.
func RandUniformThreshold(rand float64, thresholdLowerBound float64, thresholdUpperBound float64) float64 {
	return thresholdLowerBound + rand*(thresholdUpperBound-thresholdLowerBound)
}
