package newconflictdag

import (
	"sync"

	"github.com/iotaledger/hive.go/ds/advancedset"
)

type Conflict[ConflictIDType, ResourceIDType comparable] struct {
	id ConflictIDType

	weight Weight

	preferredInstead *Conflict[ConflictIDType, ResourceIDType]
	parents          *advancedset.AdvancedSet[ConflictIDType]
	children         *advancedset.AdvancedSet[*Conflict[ConflictIDType, ResourceIDType]]

	conflictSets *advancedset.AdvancedSet[*ConflictSet[ConflictIDType, ResourceIDType]]

	confirmationState ConfirmationState

	m sync.RWMutex
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
