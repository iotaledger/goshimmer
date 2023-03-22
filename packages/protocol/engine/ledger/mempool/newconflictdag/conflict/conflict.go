package conflict

import (
	"bytes"
	"sync"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/weight"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/syncutils"
	"github.com/iotaledger/hive.go/stringify"
)

type Conflict[ConflictID, ResourceID IDType] struct {
	// PreferredInsteadUpdated is triggered whenever preferred conflict is updated. It carries two values:
	// the new preferred conflict and a set of conflicts visited
	PreferredInsteadUpdated *event.Event1[*Conflict[ConflictID, ResourceID]]

	id      ConflictID
	parents *advancedset.AdvancedSet[ConflictID]

	children             *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID]]
	weight               *weight.Weight
	preferredInstead     *Conflict[ConflictID, ResourceID]
	conflictSets         map[ResourceID]*Set[ConflictID, ResourceID]
	conflictingConflicts *SortedSet[ConflictID, ResourceID]

	mutex sync.RWMutex
}

func New[ConflictID, ResourceID IDType](id ConflictID, parents *advancedset.AdvancedSet[ConflictID], conflictSets map[ResourceID]*Set[ConflictID, ResourceID], initialWeight *weight.Weight, pendingTasksCounter *syncutils.Counter) *Conflict[ConflictID, ResourceID] {
	c := &Conflict[ConflictID, ResourceID]{
		PreferredInsteadUpdated: event.New1[*Conflict[ConflictID, ResourceID]](),
		id:                      id,
		parents:                 parents,
		children:                advancedset.New[*Conflict[ConflictID, ResourceID]](),
		conflictSets:            conflictSets,
		weight:                  initialWeight,
	}
	c.preferredInstead = c
	c.conflictingConflicts = NewSortedSet[ConflictID, ResourceID](c, pendingTasksCounter)

	// add existing conflicts first, so we can correctly determine the preferred instead flag
	for _, conflictSet := range conflictSets {
		_ = conflictSet.Members().ForEach(func(element *Conflict[ConflictID, ResourceID]) (err error) {
			c.conflictingConflicts.Add(element)

			return nil
		})
	}

	// add ourselves to the other conflict sets once we are fully initialized
	for _, conflictSet := range conflictSets {
		conflictSet.Add(c)
	}

	return c
}

func (c *Conflict[ConflictID, ResourceID]) ID() ConflictID {
	return c.id
}

func (c *Conflict[ConflictID, ResourceID]) Weight() *weight.Weight {
	return c.weight
}

// ForEachConflictingConflict iterates over all conflicting Conflicts of the Conflict and calls the given callback for each of them.
func (c *Conflict[ConflictID, ResourceID]) ForEachConflictingConflict(callback func(*Conflict[ConflictID, ResourceID]) error) error {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	for currentMember := c.conflictingConflicts.heaviestMember; currentMember != nil; currentMember = currentMember.lighterMember {
		if currentMember.Conflict != c {
			if err := callback(currentMember.Conflict); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *Conflict[ConflictID, ResourceID]) Compare(other *Conflict[ConflictID, ResourceID]) int {
	if c == other {
		return weight.Equal
	}

	if other == nil {
		return weight.Heavier
	}

	if c == nil {
		return weight.Lighter
	}

	if result := c.weight.Compare(other.weight); result != weight.Equal {
		return result
	}

	return bytes.Compare(lo.PanicOnErr(c.id.Bytes()), lo.PanicOnErr(other.id.Bytes()))
}

func (c *Conflict[ConflictID, ResourceID]) PreferredInstead() *Conflict[ConflictID, ResourceID] {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.preferredInstead
}
func (c *Conflict[ConflictID, ResourceID]) SetPreferredInstead(preferredInstead *Conflict[ConflictID, ResourceID]) bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.preferredInstead == preferredInstead {
		return false
	}

	c.preferredInstead = preferredInstead
	c.PreferredInsteadUpdated.Trigger(preferredInstead)

	return true
}

func (c *Conflict[ConflictID, ResourceID]) IsPreferred() bool {
	return c.PreferredInstead() == c
}

func (c *Conflict[ConflictID, ResourceID]) String() string {
	return stringify.Struct("Conflict",
		stringify.NewStructField("id", c.id),
		stringify.NewStructField("weight", c.weight),
	)
}

func (c *Conflict[ConflictID, ResourceID]) WaitConsistent() {
	c.conflictingConflicts.WaitConsistent()
}
