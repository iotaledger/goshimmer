package conflictdag

import (
	"github.com/iotaledger/hive.go/generics/model"
	"github.com/iotaledger/hive.go/generics/set"
)

// region Conflict /////////////////////////////////////////////////////////////////////////////////////////////////////

// Conflict represents a container for transactions and outputs spawning off from a conflicting transaction.
type Conflict[ConflictID, ConflictSetID comparable] struct {
	model.Model[ConflictID, conflict[ConflictID, ConflictSetID]] `serix:"0"`
}

type conflict[ConflictID, ConflictSetID comparable] struct {
	// parents contains the parent BranchIDs that this Conflict depends on.
	Parents *set.AdvancedSet[ConflictID] `serix:"0"`

	// conflictIDs contains the identifiers of the conflicts that this Conflict is part of.
	ConflictIDs *set.AdvancedSet[ConflictSetID] `serix:"1"`

	// inclusionState contains the InclusionState of the Conflict.
	InclusionState InclusionState `serix:"2"`
}

func NewConflict[ConflictID comparable, ConflictSetID comparable](id ConflictID, parents *set.AdvancedSet[ConflictID], conflicts *set.AdvancedSet[ConflictSetID]) (new *Conflict[ConflictID, ConflictSetID]) {
	new = &Conflict[ConflictID, ConflictSetID]{model.NewModel[ConflictID](conflict[ConflictID, ConflictSetID]{
		Parents:        parents,
		ConflictIDs:    conflicts,
		InclusionState: Pending,
	})}
	new.SetID(id)

	return new
}

// Parents returns the parent BranchIDs that this Conflict depends on.
func (b *Conflict[ConflictID, ConflictSetID]) Parents() (parents *set.AdvancedSet[ConflictID]) {
	b.RLock()
	defer b.RUnlock()

	return b.M.Parents.Clone()
}

// SetParents updates the parent BranchIDs that this Conflict depends on. It returns true if the Conflict was modified.
func (b *Conflict[ConflictID, ConflictSetID]) SetParents(parents *set.AdvancedSet[ConflictID]) {
	b.Lock()
	defer b.Unlock()

	b.M.Parents = parents
	b.SetModified()

	return
}

// ConflictIDs returns the identifiers of the conflicts that this Conflict is part of.
func (b *Conflict[ConflictID, ConflictSetID]) ConflictIDs() (conflictIDs *set.AdvancedSet[ConflictSetID]) {
	b.RLock()
	defer b.RUnlock()

	return b.M.ConflictIDs.Clone()
}

// InclusionState returns the InclusionState of the Conflict.
func (b *Conflict[ConflictID, ConflictSetID]) InclusionState() (inclusionState InclusionState) {
	b.RLock()
	defer b.RUnlock()

	return b.M.InclusionState
}

// addConflict registers the membership of the Conflict in the given Conflict.
func (b *Conflict[ConflictID, ConflictSetID]) addConflict(conflictID ConflictSetID) (added bool) {
	b.Lock()
	defer b.Unlock()

	if added = b.M.ConflictIDs.Add(conflictID); added {
		b.SetModified()
	}

	return added
}

// setInclusionState sets the InclusionState of the Conflict.
func (b *Conflict[ConflictID, ConflictSetID]) setInclusionState(inclusionState InclusionState) (modified bool) {
	b.Lock()
	defer b.Unlock()

	if modified = b.M.InclusionState != inclusionState; !modified {
		return
	}

	b.M.InclusionState = inclusionState
	b.SetModified()

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ChildBranch //////////////////////////////////////////////////////////////////////////////////////////////////

// ChildBranch represents the reference between a Conflict and its children.
type ChildBranch[ConflictID comparable] struct {
	model.ReferenceModel[ConflictID, ConflictID]
}

// NewChildBranch return a new ChildBranch reference from the named parent to the named child.
func NewChildBranch[ConflictID comparable](parentBranchID, childBranchID ConflictID) (new *ChildBranch[ConflictID]) {
	return &ChildBranch[ConflictID]{model.NewReferenceModel(parentBranchID, childBranchID)}
}

// ParentBranchID returns the identifier of the parent Conflict.
func (c *ChildBranch[ConflictID]) ParentBranchID() (parentBranchID ConflictID) {
	return c.SourceID
}

// ChildBranchID returns the identifier of the child Conflict.
func (c *ChildBranch[ConflictID]) ChildBranchID() (childBranchID ConflictID) {
	return c.TargetID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ConflictMember ///////////////////////////////////////////////////////////////////////////////////////////////

// ConflictMember represents the reference between a Conflict and its contained Conflict.
type ConflictMember[ConflictSetID comparable, ConflictID comparable] struct {
	model.ReferenceModel[ConflictSetID, ConflictID]
}

// NewConflictMember return a new ConflictMember reference from the named conflict to the named Conflict.
func NewConflictMember[ConflictSetID comparable, ConflictID comparable](conflictSetID ConflictSetID, conflictID ConflictID) (new *ConflictMember[ConflictSetID, ConflictID]) {
	return &ConflictMember[ConflictSetID, ConflictID]{model.NewReferenceModel(conflictSetID, conflictID)}
}

// ConflictSetID returns the identifier of the Conflict.
func (c *ConflictMember[ConflictSetID, ConflictID]) ConflictSetID() (conflictID ConflictSetID) {
	return c.SourceID
}

// ConflictID returns the identifier of the Conflict.
func (c *ConflictMember[ConflictSetID, ConflictID]) ConflictID() (branchID ConflictID) {
	return c.TargetID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
