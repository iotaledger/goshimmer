package newconflictdag

const (
	// Undefined is the default confirmation state.
	Undefined ConfirmationState = iota

	// Accepted is the state for accepted entities.
	Accepted

	// Rejected is the state for confirmed entities.
	Rejected
)

// ConfirmationState is the confirmation state of an entity.
type ConfirmationState uint8
