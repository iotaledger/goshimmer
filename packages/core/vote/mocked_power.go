package vote

// MockedPower is a mocked power implementation that is used for testing.
type MockedPower int

// Compare compares the MockedPower to another MockedPower.
func (m MockedPower) Compare(other MockedPower) int {
	switch {
	case m < other:
		return -1
	case m > other:
		return 1
	default:
		return 0
	}
}
