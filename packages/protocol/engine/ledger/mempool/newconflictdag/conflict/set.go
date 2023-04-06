package conflict

import (
	"github.com/iotaledger/hive.go/ds/advancedset"
)

// Set represents a set of Conflicts that are conflicting with each other over a common Resource.
type Set[ConflictID, ResourceID IDType] struct {
	// ID is the ID of the Resource that the Conflicts in this Set are conflicting over.
	ID ResourceID

	// members is the set of Conflicts that are conflicting over the shared resource.
	members *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID]]
}

// NewSet creates a new Set of Conflicts that are conflicting with each other over the given Resource.
func NewSet[ConflictID, ResourceID IDType](id ResourceID) *Set[ConflictID, ResourceID] {
	return &Set[ConflictID, ResourceID]{
		ID:      id,
		members: advancedset.New[*Conflict[ConflictID, ResourceID]](),
	}
}

// Members returns the Conflicts that are conflicting over the shared resource.
func (c *Set[ConflictID, ResourceID]) Members() *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID]] {
	return c.members
}

// Add adds a newMember to the conflict set and all existing members of the set.
func (c *Set[ConflictID, ResourceID]) Add(newMember *Conflict[ConflictID, ResourceID]) {
	if c.members.Add(newMember) {
		_ = c.Members().ForEach(func(conflict *Conflict[ConflictID, ResourceID]) (err error) {
			newMember.addConflictingConflict(conflict)
			conflict.addConflictingConflict(newMember)

			return nil
		})
	}
}
