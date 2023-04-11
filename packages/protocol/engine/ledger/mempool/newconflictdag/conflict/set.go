package conflict

import (
	"sync"

	"github.com/iotaledger/hive.go/constraints"
	"github.com/iotaledger/hive.go/ds/advancedset"
)

// Set represents a set of Conflicts that are conflicting with each other over a common Resource.
type Set[ConflictID, ResourceID IDType, VotePower constraints.Comparable[VotePower]] struct {
	// ID is the ID of the Resource that the Conflicts in this Set are conflicting over.
	ID ResourceID

	// members is the set of Conflicts that are conflicting over the shared resource.
	members *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID, VotePower]]

	mutex sync.RWMutex
}

// NewSet creates a new Set of Conflicts that are conflicting with each other over the given Resource.
func NewSet[ConflictID, ResourceID IDType, VotePower constraints.Comparable[VotePower]](id ResourceID) *Set[ConflictID, ResourceID, VotePower] {
	return &Set[ConflictID, ResourceID, VotePower]{
		ID:      id,
		members: advancedset.New[*Conflict[ConflictID, ResourceID, VotePower]](),
	}
}

// Add adds a newMember to the conflict set and all existing members of the set.
func (c *Set[ConflictID, ResourceID, VotePower]) Add(addedConflict *Conflict[ConflictID, ResourceID, VotePower]) (otherMembers []*Conflict[ConflictID, ResourceID, VotePower]) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	otherMembers = c.members.Slice()

	if !c.members.Add(addedConflict) {
		return nil
	}

	return otherMembers
}
