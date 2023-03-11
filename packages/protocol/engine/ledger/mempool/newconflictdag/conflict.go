package newconflictdag

import (
	"sync"

	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/ds/shrinkingmap"
)

type Conflict[ConflictIDType, ResourceIDType comparable] struct {
	id                        ConflictIDType
	parents                   *advancedset.AdvancedSet[ConflictIDType]
	children                  *advancedset.AdvancedSet[*Conflict[ConflictIDType, ResourceIDType]]
	conflictSets              *advancedset.AdvancedSet[*ConflictSet[ConflictIDType, ResourceIDType]]
	conflictsWithHigherWeight *shrinkingmap.ShrinkingMap[ConflictIDType, *Conflict[ConflictIDType, ResourceIDType]]
	weight                    Weight
	preferredInstead          *Conflict[ConflictIDType, ResourceIDType]

	m sync.RWMutex
}

func NewConflict[ConflictIDType comparable, ResourceIDType comparable](id ConflictIDType, parents *advancedset.AdvancedSet[ConflictIDType], conflictSets *advancedset.AdvancedSet[*ConflictSet[ConflictIDType, ResourceIDType]], weight Weight) *Conflict[ConflictIDType, ResourceIDType] {
	c := &Conflict[ConflictIDType, ResourceIDType]{
		id:                        id,
		parents:                   parents,
		children:                  advancedset.NewAdvancedSet[*Conflict[ConflictIDType, ResourceIDType]](),
		conflictSets:              conflictSets,
		conflictsWithHigherWeight: shrinkingmap.New[ConflictIDType, *Conflict[ConflictIDType, ResourceIDType]](),
		weight:                    weight,
	}

	_ = c.conflictSets.ForEach(func(conflictSet *ConflictSet[ConflictIDType, ResourceIDType]) error {
		conflictSet.addConflict(c)
		return nil
	})

	return c
}

func (c *Conflict[ConflictIDType, ResourceIDType]) addConflictsWithHigherWeight(conflicts ...*Conflict[ConflictIDType, ResourceIDType]) {
	if len(conflicts) == 0 {
		return
	}

	c.m.Lock()
	defer c.m.Unlock()

	for _, conflict := range conflicts {
		c.conflictsWithHigherWeight.Set(conflict.ID(), conflict)
	}

	// TODO: determine preferred instead
}

func (c *Conflict[ConflictIDType, ResourceIDType]) PreferredInstead() *Conflict[ConflictIDType, ResourceIDType] {
	return c.preferredInstead
}

func (c *Conflict[ConflictIDType, ResourceIDType]) IsPreferred() bool {
	return c.PreferredInstead() == nil
}

func (c *Conflict[ConflictIDType, ResourceIDType]) determinePreferredInstead() *Conflict[ConflictIDType, ResourceIDType] {
	for _, conflict := range c.conflictsWithHigherWeightsDesc() {
		if conflict.IsPreferred() {
			return conflict
		}
	}

	return nil
}

func (c *Conflict[ConflictIDType, ResourceIDType]) conflictsWithHigherWeightsDesc() (conflicts []*Conflict[ConflictIDType, ResourceIDType]) {
	return nil
}

func (c *Conflict[ConflictIDType, ResourceIDType]) isPreferred() bool {

	// something is preferred if all conflicts from all of its conflict sets with a higher weight have their preferredInstead set to other conflicts
	return c.PreferredInstead == nil
}
