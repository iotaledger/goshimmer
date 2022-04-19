package branchdag

import (
	"github.com/iotaledger/hive.go/generics/event"
)

// region Events ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Events is a container that acts as a dictionary for the existing events of a BranchDAG.
type Events struct {
	// BranchCreated is an event that gets triggered whenever a new Branch is created.
	BranchCreated *event.Event[*BranchCreatedEvent]

	// BranchConflictsUpdated is an event that gets triggered whenever the ConflictIDs of a Branch are updated.
	BranchConflictsUpdated *event.Event[*BranchConflictsUpdatedEvent]

	// BranchParentsUpdated is an event that gets triggered whenever the parent BranchIDs of a Branch are updated.
	BranchParentsUpdated *event.Event[*BranchParentsUpdatedEvent]
}

// newEvents returns a new Events object.
func newEvents() *Events {
	return &Events{
		BranchCreated:          event.New[*BranchCreatedEvent](),
		BranchConflictsUpdated: event.New[*BranchConflictsUpdatedEvent](),
		BranchParentsUpdated:   event.New[*BranchParentsUpdatedEvent](),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchCreatedEvent ///////////////////////////////////////////////////////////////////////////////////////////

// BranchCreatedEvent is a container that acts as a dictionary for the BranchCreated event related parameters.
type BranchCreatedEvent struct {
	// BranchID contains the identifier of the newly created Branch.
	BranchID BranchID

	// ParentBranchIDs contains the parent Branches of the newly created Branch.
	ParentBranchIDs BranchIDs

	// ConflictIDs contains the set of conflicts that this Branch is involved with.
	ConflictIDs ConflictIDs
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchConflictsUpdatedEvent //////////////////////////////////////////////////////////////////////////////////

// BranchConflictsUpdatedEvent is a container that acts as a dictionary for the BranchConflictsUpdated event related
// parameters.
type BranchConflictsUpdatedEvent struct {
	// BranchID contains the identifier of the updated Branch.
	BranchID BranchID

	// NewConflictIDs contains the set of conflicts that this Branch was added to.
	NewConflictIDs ConflictIDs
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchParentsUpdatedEvent ////////////////////////////////////////////////////////////////////////////////////

// BranchParentsUpdatedEvent is a container that acts as a dictionary for the BranchParentsUpdated event related
// parameters.
type BranchParentsUpdatedEvent struct {
	// BranchID contains the identifier of the updated Branch.
	BranchID BranchID

	// AddedBranch contains the forked parent Branch that replaces the removed parents.
	AddedBranch BranchID

	// RemovedBranches contains the parent BranchIDs that were replaced by the newly forked Branch.
	RemovedBranches BranchIDs

	// ParentsBranchIDs contains the updated list of parent BranchIDs.
	ParentsBranchIDs BranchIDs
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
