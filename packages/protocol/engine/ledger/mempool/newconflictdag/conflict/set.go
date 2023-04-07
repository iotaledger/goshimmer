package conflict

import (
	"sync"

	"github.com/iotaledger/hive.go/ds/advancedset"
)

// Set represents a set of Conflicts that are conflicting with each other over a common Resource.
type Set[ConflictID, ResourceID IDType] struct {
	// ID is the ID of the Resource that the Conflicts in this Set are conflicting over.
	ID ResourceID

	// members is the set of Conflicts that are conflicting over the shared resource.
	members *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID]]

	mutex sync.RWMutex
}

// NewSet creates a new Set of Conflicts that are conflicting with each other over the given Resource.
func NewSet[ConflictID, ResourceID IDType](id ResourceID) *Set[ConflictID, ResourceID] {
	return &Set[ConflictID, ResourceID]{
		ID:      id,
		members: advancedset.New[*Conflict[ConflictID, ResourceID]](),
	}
}

// Add adds a newMember to the conflict set and all existing members of the set.
func (c *Set[ConflictID, ResourceID]) Add(addedConflict *Conflict[ConflictID, ResourceID]) (otherMembers []*Conflict[ConflictID, ResourceID]) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	otherMembers = c.members.Slice()

	if !c.members.Add(addedConflict) {
		return nil
	}

	return otherMembers
}
