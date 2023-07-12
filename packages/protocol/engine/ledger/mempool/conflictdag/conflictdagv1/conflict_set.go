package conflictdagv1

import (
	"sync"

	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/conflictdag"
	"github.com/iotaledger/hive.go/ds/advancedset"
)

// ConflictSet represents a set of Conflicts that are conflicting with each other over a common Resource.
type ConflictSet[ConflictID, ResourceID conflictdag.IDType, VotePower conflictdag.VotePowerType[VotePower]] struct {
	// ID is the ID of the Resource that the Conflicts in this ConflictSet are conflicting over.
	ID ResourceID

	// members is the set of Conflicts that are conflicting over the shared resource.
	members *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID, VotePower]]

	allMembersEvicted atomic.Bool

	mutex sync.RWMutex
}

// NewConflictSet creates a new ConflictSet of Conflicts that are conflicting with each other over the given Resource.
func NewConflictSet[ConflictID, ResourceID conflictdag.IDType, VotePower conflictdag.VotePowerType[VotePower]](id ResourceID) *ConflictSet[ConflictID, ResourceID, VotePower] {
	return &ConflictSet[ConflictID, ResourceID, VotePower]{
		ID:      id,
		members: advancedset.New[*Conflict[ConflictID, ResourceID, VotePower]](),
	}
}

// Add adds a Conflict to the ConflictSet and returns all other members of the set.
func (c *ConflictSet[ConflictID, ResourceID, VotePower]) Add(addedConflict *Conflict[ConflictID, ResourceID, VotePower]) (otherMembers *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID, VotePower]]) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if otherMembers = c.members.Clone(); c.members.Add(addedConflict) {
		return otherMembers
	}

	return nil
}

// Remove removes a Conflict from the ConflictSet and returns all remaining members of the set.
func (c *ConflictSet[ConflictID, ResourceID, VotePower]) Remove(removedConflict *Conflict[ConflictID, ResourceID, VotePower]) (removed bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if removed = !c.members.Delete(removedConflict); removed && c.members.IsEmpty() {
		if wasShutdown := c.allMembersEvicted.Swap(true); !wasShutdown {
			// TODO: trigger conflict set removal
		}
	}

	return removed
}

func (c *ConflictSet[ConflictID, ResourceID, VotePower]) ForEach(callback func(parent *Conflict[ConflictID, ResourceID, VotePower]) error) error {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.members.ForEach(callback)
}
