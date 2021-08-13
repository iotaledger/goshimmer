package gof

// GradeOfFinality defines the grade of finality of an object.
type GradeOfFinality uint8

const (
	// None defines the zero value for GradeOfFinality.
	None GradeOfFinality = iota
	// Low defines a low GradeOfFinality.
	Low
	// Middle defines a medium GradeOfFinality.
	Middle
	// High defines a high GradeOfFinality.
	High
)

func (gof GradeOfFinality) String() string {
	return []string{
		"GoF(None)",
		"GoF(Low)",
		"GoF(Middle)",
		"GoF(High)",
	}[gof]
}

func (gof GradeOfFinality) Bytes() []byte {
	return []byte{byte(gof)}
}
