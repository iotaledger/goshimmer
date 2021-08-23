package gof

// GradeOfFinality defines the grade of finality of an object.
type GradeOfFinality uint8

const (
	// None defines the zero value for GradeOfFinality.
	None GradeOfFinality = iota
	// Low defines a low GradeOfFinality.
	Low
	// Medium defines a medium GradeOfFinality.
	Medium
	// High defines a high GradeOfFinality.
	High
)

func (gof GradeOfFinality) String() string {
	return []string{
		"GoF(None)",
		"GoF(Low)",
		"GoF(Medium)",
		"GoF(High)",
	}[gof]
}

// Bytes returns the byte representation of the GradeOfFinality.
func (gof GradeOfFinality) Bytes() []byte {
	return []byte{byte(gof)}
}
