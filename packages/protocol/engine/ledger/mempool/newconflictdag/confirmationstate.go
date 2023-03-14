package newconflictdag

import (
	"strconv"
)

// ConfirmationState is the confirmation state of an entity.
type ConfirmationState uint8

func (c ConfirmationState) String() string {
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
	Pending ConfirmationState = iota

	// Accepted is the state for accepted entities.
	Accepted

	// Rejected is the state for confirmed entities.
	Rejected
)
