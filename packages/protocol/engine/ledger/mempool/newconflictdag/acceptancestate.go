package newconflictdag

import (
	"strconv"
)

// AcceptanceState is the confirmation state of an entity.
type AcceptanceState uint8

func (c AcceptanceState) String() string {
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
	Pending AcceptanceState = iota

	// Accepted is the state for accepted entities.
	Accepted

	// Rejected is the state for confirmed entities.
	Rejected
)
