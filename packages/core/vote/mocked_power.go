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

// Increase increases the MockedPower by one step
func (m MockedPower) Increase() MockedPower {
	return m + 1
}

// Decrease decreases the MockedPower by one step.
func (m MockedPower) Decrease() MockedPower {
	return m - 1
}
