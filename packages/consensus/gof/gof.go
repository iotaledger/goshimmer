package gof

// GradeOfFinality defines the grade of finality of an object.
type GradeOfFinality uint8

const (
	// GradeOfFinalityLow defines a low GradeOfFinality.
	GradeOfFinalityLow GradeOfFinality = iota
	// GradeOfFinalityMiddle defines a medium GradeOfFinality.
	GradeOfFinalityMiddle
	// GradeOfFinalityHigh defines a high GradeOfFinality.
	GradeOfFinalityHigh
)
