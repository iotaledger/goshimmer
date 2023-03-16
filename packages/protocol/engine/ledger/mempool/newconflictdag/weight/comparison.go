package weight

// Comparison is the result of a comparison between two values.
type Comparison = int

const (
	// Lighter is the result of a comparison between two values when the first value is lighter than the second value.
	Lighter Comparison = -1

	// Equal is the result of a comparison between two values when the first value is equal to the second value.
	Equal Comparison = 0

	// Heavier is the result of a comparison between two values when the first value is heavier than the second value.
	Heavier Comparison = 1
)
