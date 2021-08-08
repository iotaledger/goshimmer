package gof

// GradeOfFinality defines the grade of finality of an object.
type GradeOfFinality uint8

const (
	// Low defines a low GradeOfFinality.
	Low GradeOfFinality = iota
	// Middle defines a medium GradeOfFinality.
	Middle
	// High defines a high GradeOfFinality.
	High
)
