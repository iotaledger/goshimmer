package fpc

// Parameters define the parameters of an FPC instance
type Parameters struct {
	a           float64 // parameter a of the initial threshold
	b           float64 // parameter b of the initial threshold
	beta        float64 // parameter beta of the subsequents thresholds
	k           int     // number of nodes to query at each round
	l           int     // number of equal consecutive rounds (opinion doesn't change)
	m           int     // cooling off period
	MaxDuration int     // maximum number of rounds before aborting FPC
}

// NewParameters returns the default parameters
func NewParameters() *Parameters {
	return &Parameters{
		a:           0.75,
		b:           0.85,
		beta:        0.33,
		k:           10,
		m:           2,
		l:           2,
		MaxDuration: 100,
	}
}
