package mana

const (
	// Description: Taking (x * EBM1 + (1-x) * EBM2) into account when getting the mana value.

	// OnlyMana1 takes only EBM1 into account when getting the mana values.
	OnlyMana1 float64 = 1.0
	// OnlyMana2 takes only EBM2 into account when getting the mana values.
	OnlyMana2 float64 = 0.0
	// Mixed takes both EBM1 and EBM2 into account (50-50) when getting the mana values.
	Mixed float64 = 0.5
)

var (
	// default values are for half life of 6 hours, unit is 1/s
	emaCoeff1 = 0.00003209
	emaCoeff2 = 0.00003209
	decay     = 0.00003209
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
	decay = dec
}
