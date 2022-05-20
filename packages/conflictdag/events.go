package conflictdag

import (
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/set"
)

// region Events ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Events is a container that acts as a dictionary for the existing events of a ConflictDAG.
type Events[ConflictID ConflictIDType[ConflictID], ConflictSetID ConflictSetIDType[ConflictSetID]] struct {
	// BranchCreated is an event that gets triggered whenever a new Branch is created.
	BranchCreated *event.Event[*BranchCreatedEvent[ConflictID, ConflictSetID]]

	// BranchConflictsUpdated is an event that gets triggered whenever the ConflictIDs of a Branch are updated.
	BranchConflictsUpdated *event.Event[*BranchConflictsUpdatedEvent[ConflictID, ConflictSetID]]

	// BranchParentsUpdated is an event that gets triggered whenever the parent BranchIDs of a Branch are updated.
	BranchParentsUpdated *event.Event[*BranchParentsUpdatedEvent[ConflictID, ConflictSetID]]

	// BranchConfirmed is an event that gets triggered whenever a Branch is confirmed.
	BranchConfirmed *event.Event[*BranchConfirmedEvent[ConflictID]]

	// BranchRejected is an event that gets triggered whenever a Branch is rejected.
	BranchRejected *event.Event[*BranchRejectedEvent[ConflictID]]
}

// newEvents returns a new Events object.
func newEvents[ConflictID ConflictIDType[ConflictID], ConflictSetID ConflictSetIDType[ConflictSetID]]() *Events[ConflictID, ConflictSetID] {
	return &Events[ConflictID, ConflictSetID]{
		BranchCreated:          event.New[*BranchCreatedEvent[ConflictID, ConflictSetID]](),
		BranchConflictsUpdated: event.New[*BranchConflictsUpdatedEvent[ConflictID, ConflictSetID]](),
		BranchParentsUpdated:   event.New[*BranchParentsUpdatedEvent[ConflictID, ConflictSetID]](),
		BranchConfirmed:        event.New[*BranchConfirmedEvent[ConflictID]](),
		BranchRejected:         event.New[*BranchRejectedEvent[ConflictID]](),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchCreatedEvent ///////////////////////////////////////////////////////////////////////////////////////////

// BranchCreatedEvent is a container that acts as a dictionary for the BranchCreated event related parameters.
type BranchCreatedEvent[ConflictID ConflictIDType[ConflictID], ConflictSetID ConflictSetIDType[ConflictSetID]] struct {
	// BranchID contains the identifier of the newly created Branch.
	BranchID ConflictID

	// ParentBranchIDs contains the parent Branches of the newly created Branch.
	ParentBranchIDs *set.AdvancedSet[ConflictID]

	// ConflictIDs contains the set of conflicts that this Branch is involved with.
	ConflictIDs *set.AdvancedSet[ConflictSetID]
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchConflictsUpdatedEvent //////////////////////////////////////////////////////////////////////////////////

// BranchConflictsUpdatedEvent is a container that acts as a dictionary for the BranchConflictsUpdated event related
// parameters.
type BranchConflictsUpdatedEvent[ConflictID ConflictIDType[ConflictID], ConflictSetID ConflictSetIDType[ConflictSetID]] struct {
	// BranchID contains the identifier of the updated Branch.
	BranchID ConflictID

	// NewConflictIDs contains the set of conflicts that this Branch was added to.
	NewConflictIDs *set.AdvancedSet[ConflictSetID]
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchParentsUpdatedEvent ////////////////////////////////////////////////////////////////////////////////////

// BranchParentsUpdatedEvent is a container that acts as a dictionary for the BranchParentsUpdated event related
// parameters.
type BranchParentsUpdatedEvent[ConflictID ConflictIDType[ConflictID], ConflictSetID ConflictSetIDType[ConflictSetID]] struct {
	// BranchID contains the identifier of the updated Branch.
	BranchID ConflictID

	// AddedBranch contains the forked parent Branch that replaces the removed parents.
	AddedBranch ConflictID

	// RemovedBranches contains the parent BranchIDs that were replaced by the newly forked Branch.
	RemovedBranches *set.AdvancedSet[ConflictID]

	// ParentsBranchIDs contains the updated list of parent BranchIDs.
	ParentsBranchIDs *set.AdvancedSet[ConflictID]
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchConfirmedEvent /////////////////////////////////////////////////////////////////////////////////////////

// BranchConfirmedEvent is a container that acts as a dictionary for the BranchConfirmed event related parameters.
type BranchConfirmedEvent[ConflictID ConflictIDType[ConflictID]] struct {
	// BranchID contains the identifier of the confirmed Branch.
	BranchID ConflictID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchRejectedEvent //////////////////////////////////////////////////////////////////////////////////////////

// BranchRejectedEvent is a container that acts as a dictionary for the BranchRejected event related parameters.
type BranchRejectedEvent[ConflictID ConflictIDType[ConflictID]] struct {
	// BranchID contains the identifier of the rejected Branch.
	BranchID ConflictID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
