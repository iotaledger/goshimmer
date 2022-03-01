package evilwallet

import "github.com/iotaledger/goshimmer/packages/ledgerstate"

// ConflictManager keeps the conflict set of each double spend.
type ConflictManager struct {
	conflicts *Conflicts
}

// NewConflictManager creates a ConflictManager instance.
func NewConflictManager() *ConflictManager {
	return &ConflictManager{
		conflicts: NewConflicts(),
	}
}

// Conflicts is a map of conflict IDs and its conflict members.
type Conflicts struct {
	conflicts map[ledgerstate.ConflictID][]ledgerstate.OutputID
}

// NewConflicts creates a Conflicts instance.
func NewConflicts() *Conflicts {
	return &Conflicts{
		conflicts: make(map[ledgerstate.ConflictID][]ledgerstate.OutputID),
	}
}
