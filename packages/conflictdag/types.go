package conflictdag

import (
	"github.com/iotaledger/hive.go/generics/model"
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/types/confirmation"
)

// region Conflict /////////////////////////////////////////////////////////////////////////////////////////////////////

// Conflict represents a container for transactions and outputs spawning off from a conflicting transaction.
type Conflict[ConflictID, ConflictSetID comparable] struct {
	model.Storable[ConflictID, Conflict[ConflictID, ConflictSetID], *Conflict[ConflictID, ConflictSetID], conflict[ConflictID, ConflictSetID]] `serix:"0"`
}

type conflict[ConflictID, ConflictSetID comparable] struct {
	// Parents contains the parent BranchIDs that this Conflict depends on.
	Parents *set.AdvancedSet[ConflictID] `serix:"0"`

	// ConflictIDs contains the identifiers of the conflicts that this Conflict is part of.
	ConflictIDs *set.AdvancedSet[ConflictSetID] `serix:"1"`

	// ConfirmationState contains the ConfirmationState of the Conflict.
	ConfirmationState confirmation.State `serix:"2"`
}

func NewConflict[ConflictID comparable, ConflictSetID comparable](id ConflictID, parents *set.AdvancedSet[ConflictID], conflicts *set.AdvancedSet[ConflictSetID]) (new *Conflict[ConflictID, ConflictSetID]) {
	new = model.NewStorable[ConflictID, Conflict[ConflictID, ConflictSetID]](&conflict[ConflictID, ConflictSetID]{
		Parents:           parents,
		ConflictIDs:       conflicts,
		ConfirmationState: confirmation.Pending,
	})
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

// ConfirmationState returns the ConfirmationState of the Conflict.
func (b *Conflict[ConflictID, ConflictSetID]) ConfirmationState() (confirmationState confirmation.State) {
	b.RLock()
	defer b.RUnlock()

	return b.M.ConfirmationState
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

// setConfirmationState sets the ConfirmationState of the Conflict.
func (b *Conflict[ConflictID, ConflictSetID]) setConfirmationState(confirmationState confirmation.State) (modified bool) {
	b.Lock()
	defer b.Unlock()

	if modified = b.M.ConfirmationState != confirmationState; !modified {
		return
	}

	b.M.ConfirmationState = confirmationState
	b.SetModified()

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ChildBranch //////////////////////////////////////////////////////////////////////////////////////////////////

// ChildBranch represents the reference between a Conflict and its children.
type ChildBranch[ConflictID comparable] struct {
	model.StorableReference[ChildBranch[ConflictID], *ChildBranch[ConflictID], ConflictID, ConflictID] `serix:"0"`
}

// NewChildBranch return a new ChildBranch reference from the named parent to the named child.
func NewChildBranch[ConflictID comparable](parentBranchID, childBranchID ConflictID) *ChildBranch[ConflictID] {
	return model.NewStorableReference[ChildBranch[ConflictID]](parentBranchID, childBranchID)
}

// ParentBranchID returns the identifier of the parent Conflict.
func (c *ChildBranch[ConflictID]) ParentBranchID() (parentBranchID ConflictID) {
	return c.SourceID()
}

// ChildBranchID returns the identifier of the child Conflict.
func (c *ChildBranch[ConflictID]) ChildBranchID() (childBranchID ConflictID) {
	return c.TargetID()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ConflictMember ///////////////////////////////////////////////////////////////////////////////////////////////

// ConflictMember represents the reference between a Conflict and its contained Conflict.
type ConflictMember[ConflictSetID comparable, ConflictID comparable] struct {
	model.StorableReference[ConflictMember[ConflictSetID, ConflictID], *ConflictMember[ConflictSetID, ConflictID], ConflictSetID, ConflictID] `serix:"0"`
}

// NewConflictMember return a new ConflictMember reference from the named conflict to the named Conflict.
func NewConflictMember[ConflictSetID comparable, ConflictID comparable](conflictSetID ConflictSetID, conflictID ConflictID) (new *ConflictMember[ConflictSetID, ConflictID]) {
	return model.NewStorableReference[ConflictMember[ConflictSetID, ConflictID]](conflictSetID, conflictID)
}

// ConflictSetID returns the identifier of the Conflict.
func (c *ConflictMember[ConflictSetID, ConflictID]) ConflictSetID() (conflictID ConflictSetID) {
	return c.SourceID()
}

// ConflictID returns the identifier of the Conflict.
func (c *ConflictMember[ConflictSetID, ConflictID]) ConflictID() (branchID ConflictID) {
	return c.TargetID()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
