package mana

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
