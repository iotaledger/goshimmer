package acceptance

import (
	"strconv"
)

const (
	// Pending is the state of pending conflicts.
	Pending State = iota

	// Accepted is the state of accepted conflicts.
	Accepted

	// Rejected is the state of rejected conflicts.
	Rejected
)

// State represents the acceptance state of an entity.
type State uint8

// String returns a human-readable representation of the State.
func (c State) String() string {
	switch c {
	case Pending:
		return "Pending"
	case Accepted:
		return "Accepted"
	case Rejected:
		return "Rejected"
	default:
		return "Unknown (" + strconv.Itoa(int(c)) + ")"
	}
}
