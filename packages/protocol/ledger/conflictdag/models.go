package conflictdag

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/types/confirmation"
)

// region Conflict /////////////////////////////////////////////////////////////////////////////////////////////////////

type Conflict[ConflictIDType, ResourceIDType comparable] struct {
	id ConflictIDType

	parents  *set.AdvancedSet[*Conflict[ConflictIDType, ResourceIDType]]
	children *set.AdvancedSet[*Conflict[ConflictIDType, ResourceIDType]]

	conflictSets *set.AdvancedSet[*ConflictSet[ConflictIDType, ResourceIDType]]

	confirmationState confirmation.State

	m sync.RWMutex
}

func NewConflict[ConflictIDType comparable, ResourceIDType comparable](id ConflictIDType, parents *set.AdvancedSet[*Conflict[ConflictIDType, ResourceIDType]], conflictSets *set.AdvancedSet[*ConflictSet[ConflictIDType, ResourceIDType]]) (c *Conflict[ConflictIDType, ResourceIDType]) {
	c = &Conflict[ConflictIDType, ResourceIDType]{
		id:                id,
		parents:           parents,
		children:          set.NewAdvancedSet[*Conflict[ConflictIDType, ResourceIDType]](),
		conflictSets:      conflictSets,
		confirmationState: confirmation.Pending,
	}

	return c
}

// Parents returns the parent ConflictIDs that this Conflict depends on.
func (c *Conflict[ConflictIDType, ResourceIDType]) Parents() (parents *set.AdvancedSet[*Conflict[ConflictIDType, ResourceIDType]]) {
	c.m.RLock()
	defer c.m.RUnlock()

	return c.parents.Clone()
}

// SetParents updates the parent ConflictIDs that this Conflict depends on. It returns true if the Conflict was modified.
func (c *Conflict[ConflictIDType, ResourceIDType]) setParents(parents *set.AdvancedSet[*Conflict[ConflictIDType, ResourceIDType]]) {
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ConflictSet //////////////////////////////////////////////////////////////////////////////////////////////////

type ConflictSet[ConflictIDType, ResourceIDType comparable] struct {
	id        ResourceIDType
	conflicts *set.AdvancedSet[*Conflict[ConflictIDType, ResourceIDType]]
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

func (c *ConflictSet[ConflictIDType, ResourceIDType]) AddMember(conflict *Conflict[ConflictIDType, ResourceIDType]) (added bool) {
	return c.conflicts.Add(conflict)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
