package conflict

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/reentrantmutex"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/weight"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/stringify"
)

type Conflict[ConflictID, ResourceID IDType] struct {
	// PreferredInsteadUpdated is triggered whenever preferred conflict is updated. It carries two values:
	// the new preferred conflict and a set of conflicts visited
	PreferredInsteadUpdated *event.Event2[*Conflict[ConflictID, ResourceID], reentrantmutex.ThreadID]

	id      ConflictID
	parents *advancedset.AdvancedSet[ConflictID]

	children             *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID]]
	weight               *weight.Weight
	preferredInstead     *Conflict[ConflictID, ResourceID]
	conflictSets         map[ResourceID]*Set[ConflictID, ResourceID]
	conflictingConflicts *SortedSet[ConflictID, ResourceID]

	mutex sync.RWMutex
}

func (c *Conflict[ConflictID, ResourceID]) PreferredInstead() *Conflict[ConflictID, ResourceID] {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.preferredInstead
}
func (c *Conflict[ConflictID, ResourceID]) SetPreferredInstead(preferredInstead *Conflict[ConflictID, ResourceID]) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.preferredInstead = preferredInstead
}

func New[ConflictID, ResourceID IDType](id ConflictID, parents *advancedset.AdvancedSet[ConflictID], conflictSets map[ResourceID]*Set[ConflictID, ResourceID], initialWeight *weight.Weight) *Conflict[ConflictID, ResourceID] {
	c := &Conflict[ConflictID, ResourceID]{
		PreferredInsteadUpdated: event.New2[*Conflict[ConflictID, ResourceID], reentrantmutex.ThreadID](),
		id:                      id,
		parents:                 parents,
		children:                advancedset.New[*Conflict[ConflictID, ResourceID]](),
		conflictSets:            conflictSets,
		weight:                  initialWeight,
	}

	c.conflictingConflicts = NewSortedSet[ConflictID, ResourceID](c)
	c.conflictingConflicts.HeaviestPreferredMemberUpdated.Hook(func(eventConflict *Conflict[ConflictID, ResourceID], threadID reentrantmutex.ThreadID) {
		fmt.Println(c.ID(), "prefers", eventConflict.ID(), threadID)
		c.PreferredInsteadUpdated.Trigger(eventConflict, threadID)
	})

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

func (c *Conflict[ConflictID, ResourceID]) onWeightUpdated(newWeight weight.Value) {
	// c.m.Lock()
	// defer c.m.Unlock()
	//
	// if heavierConflict.IsPreferred() && heavierConflict.Compare(c.preferredInstead) == weight.Heavier {
	// 	c.preferredInstead = heavierConflict
	// }
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

func (c *Conflict[ConflictID, ResourceID]) PreferredInstead(optThreadID ...reentrantmutex.ThreadID) *Conflict[ConflictID, ResourceID] {
	return c.conflictingConflicts.HeaviestPreferredConflict(optThreadID...)
}

func (c *Conflict[ConflictID, ResourceID]) IsPreferred(optThreadID ...reentrantmutex.ThreadID) bool {
	return c.PreferredInstead(optThreadID...) == c
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
