package newconflictdag

// ConfirmationState is the confirmation state of an entity.
type ConfirmationState uint8

const (
	// Pending is the default confirmation state.
	Pending ConfirmationState = iota

	// Accepted is the state for accepted entities.
	Accepted

	// Rejected is the state for confirmed entities.
	Rejected
)
