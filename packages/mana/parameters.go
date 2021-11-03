package mana

const (
	// Description: Taking (x * EBM1 + (1-x) * EBM2) into account when getting the mana value.

	// OnlyMana1 takes only EBM1 into account when getting the mana values.
	OnlyMana1 float64 = 1.0
	// OnlyMana2 takes only EBM2 into account when getting the mana values.
	OnlyMana2 float64 = 0.0
	// Mixed takes both EBM1 and EBM2 into account (50-50) when getting the mana values.
	Mixed float64 = 0.5

	// DeltaStopUpdate stops the update of the effective mana value if it is in the delta interval of the the base mana
	// value. Base mana can only be an integer, and effective mana tends to this integer value over time if there are no
	// pledges or revokes.
	// It is used primarily for consensus mana, since for access mana, base mana changes wrt to time. The updates for
	// access mana stop when the base mana value AND the effective value is in DeltaStopUpdates interval of 0.
	DeltaStopUpdate float64 = 0.001

	// MinEffectiveMana defines the threshold to consider an effective mana value zero.
	MinEffectiveMana = 0.001
	// MinBaseMana defines the threshold to consider the base mana value zero.
	MinBaseMana = 0.001
)

var (
	// default values are for half life of 6 hours, unit is 1/s
	// exponential moving average coefficient for Mana 1 calculation (used in consensus mana).
	emaCoeff1 = 0.00003209
	// exponential moving average coefficient for Mana 2 calculation (used in access mana).
	emaCoeff2 = 0.00003209
	// Decay is the mana decay (gamma) (used in access mana), in 1/sec.
	Decay = 0.00003209
)

// SetCoefficients sets the coefficients for mana calculation.
func SetCoefficients(ema1 float64, ema2 float64, dec float64) {
	if ema1 <= 0.0 {
		panic("invalid emaCoefficient1 parameter, value must be greater than 0.")
	}
	if ema2 <= 0.0 {
		panic("invalid emaCoefficient2 parameter, value must be greater than 0.")
	}
	if dec <= 0.0 {
		panic("invalid decay (gamma) parameter, value must be greater than 0.")
	}
	emaCoeff1 = ema1
	emaCoeff2 = ema2
	Decay = dec
}
