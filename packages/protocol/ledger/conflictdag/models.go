package conflictdag

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/types/confirmation"
)

// region Conflict /////////////////////////////////////////////////////////////////////////////////////////////////////

type Conflict[ConflictIDType, ResourceIDType comparable] struct {
	id ConflictIDType

	parents  *set.AdvancedSet[ConflictIDType]
	children *set.AdvancedSet[*Conflict[ConflictIDType, ResourceIDType]]

	conflictSets *set.AdvancedSet[*ConflictSet[ConflictIDType, ResourceIDType]]

	confirmationState confirmation.State

	m sync.RWMutex
}

func NewConflict[ConflictIDType comparable, ResourceIDType comparable](id ConflictIDType, parents *set.AdvancedSet[ConflictIDType], conflictSets *set.AdvancedSet[*ConflictSet[ConflictIDType, ResourceIDType]]) (c *Conflict[ConflictIDType, ResourceIDType]) {
	c = &Conflict[ConflictIDType, ResourceIDType]{
		id:                id,
		parents:           parents,
		children:          set.NewAdvancedSet[*Conflict[ConflictIDType, ResourceIDType]](),
		conflictSets:      conflictSets,
		confirmationState: confirmation.Pending,
	}

	return c
}

func (c *Conflict[ConflictIDType, ResourceIDType]) ID() ConflictIDType {
	return c.id
}

// Parents returns the parent ConflictIDs that this Conflict depends on.
func (c *Conflict[ConflictIDType, ResourceIDType]) Parents() (parents *set.AdvancedSet[ConflictIDType]) {
	c.m.RLock()
	defer c.m.RUnlock()

	return c.parents.Clone()
}

// SetParents updates the parent ConflictIDs that this Conflict depends on. It returns true if the Conflict was modified.
func (c *Conflict[ConflictIDType, ResourceIDType]) setParents(parents *set.AdvancedSet[ConflictIDType]) {
	c.m.Lock()
	defer c.m.Unlock()

	c.parents = parents
}

// ConflictSets returns the identifiers of the conflict sets that this Conflict is part of.
func (c *Conflict[ConflictIDType, ResourceIDType]) ConflictSets() (conflictSets *set.AdvancedSet[*ConflictSet[ConflictIDType, ResourceIDType]]) {
	c.m.RLock()
	defer c.m.RUnlock()

	return c.conflictSets.Clone()
}

func (c *Conflict[ConflictIDType, ResourceIDType]) Children() (children *set.AdvancedSet[*Conflict[ConflictIDType, ResourceIDType]]) {
	c.m.RLock()
	defer c.m.RUnlock()

	return c.children.Clone()
}

// addConflictSet registers the membership of the Conflict in the given conflict set.
func (c *Conflict[ConflictIDType, ResourceIDType]) addConflictSet(conflictSet *ConflictSet[ConflictIDType, ResourceIDType]) (added bool) {
	c.m.Lock()
	defer c.m.Unlock()

	return c.conflictSets.Add(conflictSet)
}

// ConfirmationState returns the ConfirmationState of the Conflict.
func (c *Conflict[ConflictIDType, ResourceIDType]) ConfirmationState() (confirmationState confirmation.State) {
	c.m.RLock()
	defer c.m.RUnlock()

	return c.confirmationState
}

// setConfirmationState sets the ConfirmationState of the Conflict.
func (c *Conflict[ConflictIDType, ResourceIDType]) setConfirmationState(confirmationState confirmation.State) (modified bool) {
	c.m.Lock()
	defer c.m.Unlock()

	if modified = c.confirmationState != confirmationState; !modified {
		return
	}

	c.confirmationState = confirmationState

	return
}

func (c *Conflict[ConflictIDType, ResourceIDType]) addChild(child *Conflict[ConflictIDType, ResourceIDType]) (added bool) {
	c.m.Lock()
	defer c.m.Unlock()

	return c.children.Add(child)
}

func (c *Conflict[ConflictIDType, ResourceIDType]) ForEachConflictingConflict(consumer func(conflictingConflict *Conflict[ConflictIDType, ResourceIDType]) bool) {
	for it := c.ConflictSets().Iterator(); it.HasNext(); {
		conflictSet := it.Next()
		for itConflictSets := conflictSet.Conflicts().Iterator(); itConflictSets.HasNext(); {
			conflictingConflict := itConflictSets.Next()
			if conflictingConflict.ID() == c.ID() {
				continue
			}

			if !consumer(conflictingConflict) {
				return
			}
		}
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ConflictSet //////////////////////////////////////////////////////////////////////////////////////////////////

type ConflictSet[ConflictIDType, ResourceIDType comparable] struct {
	id        ResourceIDType
	conflicts *set.AdvancedSet[*Conflict[ConflictIDType, ResourceIDType]]

	m sync.RWMutex
}

func NewConflictSet[ConflictIDType comparable, ResourceIDType comparable](id ResourceIDType) (c *ConflictSet[ConflictIDType, ResourceIDType]) {
	return &ConflictSet[ConflictIDType, ResourceIDType]{
		id:        id,
		conflicts: set.NewAdvancedSet[*Conflict[ConflictIDType, ResourceIDType]](),
	}
}

func (c *ConflictSet[ConflictIDType, ResourceIDType]) ID() (id ResourceIDType) {
	return c.id
}

func (c *ConflictSet[ConflictIDType, ResourceIDType]) Conflicts() *set.AdvancedSet[*Conflict[ConflictIDType, ResourceIDType]] {
	c.m.RLock()
	defer c.m.RUnlock()

	return c.conflicts.Clone()
}

func (c *ConflictSet[ConflictIDType, ResourceIDType]) AddConflictMember(conflict *Conflict[ConflictIDType, ResourceIDType]) (added bool) {
	c.m.Lock()
	defer c.m.Unlock()

	return c.conflicts.Add(conflict)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
