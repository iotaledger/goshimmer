package confirmation

const (
	// Undefined is the default confirmation state.
	Undefined State = iota

	// Rejected is the state for rejected entities.
	Rejected

	// Pending is the state for pending entities.
	Pending

	// Accepted is the state for accepted entities.
	Accepted

	// NotConflicting is the state for a conflict, whose all conflicting conflicts are orphaned and rejected.
	NotConflicting

	// Confirmed is the state for confirmed entities.
	Confirmed
)

// State is the confirmation state of an entity.
type State uint8

// IsAccepted returns true if the state is Accepted or Confirmed.
func (s State) IsAccepted() bool {
	return s >= Accepted
}

// IsConfirmed returns true if the state is Confirmed.
func (s State) IsConfirmed() bool {
	return s >= Confirmed
}

// IsRejected returns true if the state is Rejected.
func (s State) IsRejected() bool {
	return s == Rejected
}

// IsPending returns true if the state is Pending.
func (s State) IsPending() bool {
	return s == Pending
}

// Aggregate returns the lowest confirmation state of all given states.
func (s State) Aggregate(o State) State {
	if o == Undefined || o == NotConflicting {
		return s
	}

	if o < s {
		return o
	}

	return s
}

// String returns a human-readable representation of the State.
func (s State) String() (humanReadable string) {
	switch s {
	case Pending:
		return "Pending"
	case Rejected:
		return "Rejected"
	case Accepted:
		return "Accepted"
	case NotConflicting:
		return "NotConflicting"
	case Confirmed:
		return "Confirmed"
	default:
		return "Undefined"
	}
}
