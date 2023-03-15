package newconflictdag

import (
	"bytes"
	"sync"

	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/stringify"
)

type Conflict[ConflictID, ResourceID IDType] struct {
	OnWeightUpdated *event.Event1[*Conflict[ConflictID, ResourceID]]

	id              ConflictID
	parents         *advancedset.AdvancedSet[ConflictID]
	children        *advancedset.AdvancedSet[*Conflict[ConflictID, ResourceID]]
	conflictSets    *advancedset.AdvancedSet[*ConflictSet[ConflictID, ResourceID]]
	sortedConflicts *SortedConflicts[ConflictID, ResourceID]
	weight          *Weight

	heavierConflicts *shrinkingmap.ShrinkingMap[ConflictID, *Conflict[ConflictID, ResourceID]]
	preferredInstead *Conflict[ConflictID, ResourceID]

	m sync.RWMutex
}

func NewConflict[ConflictID, ResourceID IDType](id ConflictID, parents *advancedset.AdvancedSet[ConflictID], conflictSets *advancedset.AdvancedSet[*ConflictSet[ConflictID, ResourceID]], weight *Weight) *Conflict[ConflictID, ResourceID] {
	c := &Conflict[ConflictID, ResourceID]{
		OnWeightUpdated: event.New1[*Conflict[ConflictID, ResourceID]](),

		id:               id,
		parents:          parents,
		children:         advancedset.NewAdvancedSet[*Conflict[ConflictID, ResourceID]](),
		conflictSets:     conflictSets,
		heavierConflicts: shrinkingmap.New[ConflictID, *Conflict[ConflictID, ResourceID]](),
		weight:           weight,
	}

	for _, conflictSet := range conflictSets.Slice() {
		conflictSet.RegisterConflict(c)
	}

	c.weight.OnUpdate.Hook(func() { c.OnWeightUpdated.Trigger(c) })

	return c
}

func (c *Conflict[ConflictID, ResourceID]) ID() ConflictID {
	return c.id
}

func (c *Conflict[ConflictID, ResourceID]) Weight() *Weight {
	return c.weight
}

func (c *Conflict[ConflictID, ResourceID]) registerHeavierConflict(heavierConflict *Conflict[ConflictID, ResourceID]) bool {
	if heavierConflict.CompareTo(c) != Larger {
		return false
	}

	c.m.Lock()
	defer c.m.Unlock()

	if c.heavierConflicts.Set(heavierConflict.id, heavierConflict) {
		_ = heavierConflict.OnWeightUpdated.Hook(c.onWeightUpdated)
		// subscribe to events of the heavier conflicts

		c.onWeightUpdated(heavierConflict)
	}

	return true
}

func (c *Conflict[ConflictID, ResourceID]) onWeightUpdated(heavierConflict *Conflict[ConflictID, ResourceID]) {
	c.m.Lock()
	defer c.m.Unlock()

	if heavierConflict.IsPreferred() && heavierConflict.CompareTo(c.preferredInstead) == Larger {
		c.preferredInstead = heavierConflict
	}
}

func (c *Conflict[ConflictID, ResourceID]) CompareTo(other *Conflict[ConflictID, ResourceID]) int {
	if c == other {
		return Equal
	}

	if other == nil {
		return Larger
	}

	if c == nil {
		return Smaller
	}

	if result := c.weight.Compare(other.weight); result != Equal {
		return result
	}

	return bytes.Compare(lo.PanicOnErr(c.id.Bytes()), lo.PanicOnErr(other.id.Bytes()))
}

func (c *Conflict[ConflictID, ResourceID]) PreferredInstead() *Conflict[ConflictID, ResourceID] {
	c.m.RLock()
	defer c.m.RUnlock()

	return c.preferredInstead
}

func (c *Conflict[ConflictID, ResourceID]) IsPreferred() bool {
	return c.PreferredInstead() == nil
}

func (c *Conflict[ConflictID, ResourceID]) String() string {
	return stringify.Struct("Conflict",
		stringify.NewStructField("id", c.id),
		stringify.NewStructField("weight", c.weight),
	)
}
