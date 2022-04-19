package ledger

// Snapshot is a snapshot of the ledger state.
type Snapshot struct {
	Outputs Outputs
}

// NewSnapshot creates a new snapshot.
func NewSnapshot(outputs Outputs) *Snapshot {
	return &Snapshot{
		Outputs: outputs,
	}
}

func (s *Snapshot) String() string {
	// TODO
}
