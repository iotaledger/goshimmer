package acceptance

import (
	"strconv"
)

// State is the confirmation state of an entity.
type State uint8

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

const (
	// Pending is the default confirmation state.
	Pending State = iota

	// Accepted is the state for accepted entities.
	Accepted

	// Rejected is the state for confirmed entities.
	Rejected
)
