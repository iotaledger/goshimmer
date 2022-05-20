package conflictdag

import (
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/set"
)

// region Events ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Events is a container that acts as a dictionary for the existing events of a ConflictDAG.
type Events[ConflictID ConflictIDType[ConflictID], ConflictingResourceID ConflictSetIDType[ConflictingResourceID]] struct {
	// ConflictCreated is an event that gets triggered whenever a new Conflict is created.
	ConflictCreated *event.Event[*ConflictCreatedEvent[ConflictID, ConflictingResourceID]]

	// BranchConflictsUpdated is an event that gets triggered whenever the ConflictIDs of a Conflict are updated.
	BranchConflictsUpdated *event.Event[*BranchConflictsUpdatedEvent[ConflictID, ConflictingResourceID]]

	// BranchParentsUpdated is an event that gets triggered whenever the parent BranchIDs of a Conflict are updated.
	BranchParentsUpdated *event.Event[*BranchParentsUpdatedEvent[ConflictID, ConflictingResourceID]]

	// BranchConfirmed is an event that gets triggered whenever a Conflict is confirmed.
	BranchConfirmed *event.Event[*BranchConfirmedEvent[ConflictID]]

	// BranchRejected is an event that gets triggered whenever a Conflict is rejected.
	BranchRejected *event.Event[*BranchRejectedEvent[ConflictID]]
}

// newEvents returns a new Events object.
func newEvents[ConflictID ConflictIDType[ConflictID], ConflictSetID ConflictSetIDType[ConflictSetID]]() *Events[ConflictID, ConflictSetID] {
	return &Events[ConflictID, ConflictSetID]{
		ConflictCreated:        event.New[*ConflictCreatedEvent[ConflictID, ConflictSetID]](),
		BranchConflictsUpdated: event.New[*BranchConflictsUpdatedEvent[ConflictID, ConflictSetID]](),
		BranchParentsUpdated:   event.New[*BranchParentsUpdatedEvent[ConflictID, ConflictSetID]](),
		BranchConfirmed:        event.New[*BranchConfirmedEvent[ConflictID]](),
		BranchRejected:         event.New[*BranchRejectedEvent[ConflictID]](),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ConflictCreatedEvent /////////////////////////////////////////////////////////////////////////////////////////

// ConflictCreatedEvent is an event that gets triggered when a new Conflict was created.
type ConflictCreatedEvent[ConflictID ConflictIDType[ConflictID], ConflictingResourceID ConflictSetIDType[ConflictingResourceID]] struct {
	// ID contains the identifier of the newly created Conflict.
	ID ConflictID

	// ParentConflictIDs contains the identifiers of the parents of the newly created Conflict.
	ParentConflictIDs *set.AdvancedSet[ConflictID]

	// ConflictingResourceIDs contains the identifiers of the conflicting resources that this Conflict is associated to.
	ConflictingResourceIDs *set.AdvancedSet[ConflictingResourceID]
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchConflictsUpdatedEvent //////////////////////////////////////////////////////////////////////////////////

// BranchConflictsUpdatedEvent is a container that acts as a dictionary for the BranchConflictsUpdated event related
// parameters.
type BranchConflictsUpdatedEvent[ConflictID ConflictIDType[ConflictID], ConflictSetID ConflictSetIDType[ConflictSetID]] struct {
	// BranchID contains the identifier of the updated Conflict.
	BranchID ConflictID

	// NewConflictIDs contains the set of conflicts that this Conflict was added to.
	NewConflictIDs *set.AdvancedSet[ConflictSetID]
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchParentsUpdatedEvent ////////////////////////////////////////////////////////////////////////////////////

// BranchParentsUpdatedEvent is a container that acts as a dictionary for the BranchParentsUpdated event related
// parameters.
type BranchParentsUpdatedEvent[ConflictID ConflictIDType[ConflictID], ConflictSetID ConflictSetIDType[ConflictSetID]] struct {
	// BranchID contains the identifier of the updated Conflict.
	BranchID ConflictID

	// AddedBranch contains the forked parent Conflict that replaces the removed parents.
	AddedBranch ConflictID

	// RemovedBranches contains the parent BranchIDs that were replaced by the newly forked Conflict.
	RemovedBranches *set.AdvancedSet[ConflictID]

	// ParentsBranchIDs contains the updated list of parent BranchIDs.
	ParentsBranchIDs *set.AdvancedSet[ConflictID]
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchConfirmedEvent /////////////////////////////////////////////////////////////////////////////////////////

// BranchConfirmedEvent is a container that acts as a dictionary for the BranchConfirmed event related parameters.
type BranchConfirmedEvent[ConflictID ConflictIDType[ConflictID]] struct {
	// BranchID contains the identifier of the confirmed Conflict.
	BranchID ConflictID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchRejectedEvent //////////////////////////////////////////////////////////////////////////////////////////

// BranchRejectedEvent is a container that acts as a dictionary for the BranchRejected event related parameters.
type BranchRejectedEvent[ConflictID ConflictIDType[ConflictID]] struct {
	// BranchID contains the identifier of the rejected Conflict.
	BranchID ConflictID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
