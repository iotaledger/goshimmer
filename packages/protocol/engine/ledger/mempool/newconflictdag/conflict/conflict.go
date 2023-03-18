package conflict

import (
	"bytes"
	"sync"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/newconflictdag/weight"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/stringify"
)

type Conflict[ConflictID, ResourceID IDType] struct {
	PreferredInsteadUpdated *event.Event1[*Conflict[ConflictID, ResourceID]]

	id                   ConflictID
	parents              *advancedset.AdvancedSet[ConflictID]
	children             *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID]]
	weight               *weight.Weight
	conflictSets         map[ResourceID]*Set[ConflictID, ResourceID]
	conflictingConflicts *SortedSet[ConflictID, ResourceID]

	mutex sync.RWMutex
}

func New[ConflictID, ResourceID IDType](id ConflictID, parents *advancedset.AdvancedSet[ConflictID], conflictSets map[ResourceID]*Set[ConflictID, ResourceID], initialWeight *weight.Weight) *Conflict[ConflictID, ResourceID] {
	c := &Conflict[ConflictID, ResourceID]{
		PreferredInsteadUpdated: event.New1[*Conflict[ConflictID, ResourceID]](),
		id:                      id,
		parents:                 parents,
		children:                advancedset.New[*Conflict[ConflictID, ResourceID]](),
		conflictSets:            conflictSets,
		weight:                  initialWeight,
	}

	c.conflictingConflicts = NewSortedSet[ConflictID, ResourceID](c)
	c.conflictingConflicts.HeaviestPreferredMemberUpdated.Hook(c.PreferredInsteadUpdated.Trigger)

	// add existing conflicts first, so we can correctly determine the preferred instead flag
	for _, conflictSet := range conflictSets {
		_ = conflictSet.Members().ForEach(func(element *Conflict[ConflictID, ResourceID]) (err error) {
			c.conflictingConflicts.Add(element)

			return nil
		})
	}

	// add ourselves to the other conflict sets once we are fully initialized
	for _, conflictSet := range conflictSets {
		conflictSet.Members().Add(c)
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

func (c *Conflict[ConflictID, ResourceID]) PreferredInstead() *Conflict[ConflictID, ResourceID] {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.conflictingConflicts.HeaviestPreferredConflict()
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
