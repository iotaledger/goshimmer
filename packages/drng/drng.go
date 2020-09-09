package drng

// DRNG holds the state and events of a drng instance.
type DRNG struct {
	State  *State // The state of the DRNG.
	Events *Event // The events fired on the DRNG.
}

// New creates a new DRNG instance.
func New(setters ...Option) *DRNG {
	return &DRNG{
		State:  NewState(setters...),
		Events: newEvent(),
	}
}
