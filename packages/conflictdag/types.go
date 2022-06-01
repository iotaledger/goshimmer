package conflictdag

import (
	"fmt"

	"github.com/iotaledger/hive.go/generics/model"
	"github.com/iotaledger/hive.go/generics/set"
)

// region Conflict /////////////////////////////////////////////////////////////////////////////////////////////////////

// Conflict represents a container for transactions and outputs spawning off from a conflicting transaction.
type Conflict[ConflictID, ConflictSetID comparable] struct {
	model.Storable[ConflictID, conflict[ConflictID, ConflictSetID]] `serix:"0"`
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
	new = &Conflict[ConflictID, ConflictSetID]{model.NewStorable[ConflictID](conflict[ConflictID, ConflictSetID]{
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
	model.StorableReference[ConflictID, ConflictID] `serix:"0"`
}

// NewChildBranch return a new ChildBranch reference from the named parent to the named child.
func NewChildBranch[ConflictID comparable](parentBranchID, childBranchID ConflictID) (new *ChildBranch[ConflictID]) {
	return &ChildBranch[ConflictID]{model.NewStorableReference(parentBranchID, childBranchID)}
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
	model.StorableReference[ConflictSetID, ConflictID] `serix:"0"`
}

// NewConflictMember return a new ConflictMember reference from the named conflict to the named Conflict.
func NewConflictMember[ConflictSetID comparable, ConflictID comparable](conflictSetID ConflictSetID, conflictID ConflictID) (new *ConflictMember[ConflictSetID, ConflictID]) {
	return &ConflictMember[ConflictSetID, ConflictID]{model.NewStorableReference(conflictSetID, conflictID)}
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

// region InclusionState ///////////////////////////////////////////////////////////////////////////////////////////////

// InclusionState represents the confirmation status of branches in the ConflictDAG.
type InclusionState uint8

const (
	// Pending represents elements that have neither been confirmed nor rejected.
	Pending InclusionState = iota

	// Confirmed represents elements that have been confirmed and will stay part of the ledger state forever.
	Confirmed

	// Rejected represents elements that have been rejected and will not be included in the ledger state.
	Rejected
)

// String returns a human-readable representation of the InclusionState.
func (i InclusionState) String() string {
	switch i {
	case Pending:
		return "InclusionState(Pending)"
	case Confirmed:
		return "InclusionState(Confirmed)"
	case Rejected:
		return "InclusionState(Rejected)"
	default:
		return fmt.Sprintf("InclusionState(%X)", uint8(i))
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
