package newconflictdag

import (
	"sync"

	"github.com/iotaledger/hive.go/constraints"
	"github.com/iotaledger/hive.go/ds/advancedset"
)

// ConflictSet represents a set of Conflicts that are conflicting with each other over a common Resource.
type ConflictSet[ConflictID, ResourceID IDType, VotePower constraints.Comparable[VotePower]] struct {
	// ID is the ID of the Resource that the Conflicts in this ConflictSet are conflicting over.
	ID ResourceID

	// members is the set of Conflicts that are conflicting over the shared resource.
	members *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID, VotePower]]

	mutex sync.RWMutex
}

// NewConflictSet creates a new ConflictSet of Conflicts that are conflicting with each other over the given Resource.
func NewConflictSet[ConflictID, ResourceID IDType, VotePower constraints.Comparable[VotePower]](id ResourceID) *ConflictSet[ConflictID, ResourceID, VotePower] {
	return &ConflictSet[ConflictID, ResourceID, VotePower]{
		ID:      id,
		members: advancedset.New[*Conflict[ConflictID, ResourceID, VotePower]](),
	}
}

// Add adds a Conflict to the ConflictSet and returns all other members of the set.
func (c *ConflictSet[ConflictID, ResourceID, VotePower]) Add(addedConflict *Conflict[ConflictID, ResourceID, VotePower]) (otherMembers []*Conflict[ConflictID, ResourceID, VotePower]) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	otherMembers = c.members.Slice()

	if !c.members.Add(addedConflict) {
		return nil
	}

	return otherMembers
}

// Remove removes a Conflict from the ConflictSet and returns all remaining members of the set.
func (c *ConflictSet[ConflictID, ResourceID, VotePower]) Remove(removedConflict *Conflict[ConflictID, ResourceID, VotePower]) (otherMembers []*Conflict[ConflictID, ResourceID, VotePower]) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if !c.members.Delete(removedConflict) {
		return nil
	}

	if c.members.IsEmpty() {
		// TODO: trigger conflict set removal
	}

	return c.members.Slice()
}
