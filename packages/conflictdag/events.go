package conflictdag

import (
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/set"
)

// region Events ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Events is a container that acts as a dictionary for the events of a ConflictDAG.
type Events[ConflictID, ConflictingResourceID comparable] struct {
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
func newEvents[ConflictID, ConflictingResourceID comparable]() *Events[ConflictID, ConflictingResourceID] {
	return &Events[ConflictID, ConflictingResourceID]{
		ConflictCreated:        event.New[*ConflictCreatedEvent[ConflictID, ConflictingResourceID]](),
		BranchConflictsUpdated: event.New[*BranchConflictsUpdatedEvent[ConflictID, ConflictingResourceID]](),
		BranchParentsUpdated:   event.New[*BranchParentsUpdatedEvent[ConflictID, ConflictingResourceID]](),
		BranchConfirmed:        event.New[*BranchConfirmedEvent[ConflictID]](),
		BranchRejected:         event.New[*BranchRejectedEvent[ConflictID]](),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ConflictCreatedEvent /////////////////////////////////////////////////////////////////////////////////////////

// ConflictCreatedEvent is an event that gets triggered when a new Conflict was created.
type ConflictCreatedEvent[ConflictID, ConflictingResourceID comparable] struct {
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
type BranchConflictsUpdatedEvent[ConflictID, ConflictingResourceID comparable] struct {
	// BranchID contains the identifier of the updated Conflict.
	BranchID ConflictID

	// NewConflictIDs contains the set of conflicts that this Conflict was added to.
	NewConflictIDs *set.AdvancedSet[ConflictingResourceID]
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchParentsUpdatedEvent ////////////////////////////////////////////////////////////////////////////////////

// BranchParentsUpdatedEvent is a container that acts as a dictionary for the BranchParentsUpdated event related
// parameters.
type BranchParentsUpdatedEvent[ConflictID, ConflictingResourceID comparable] struct {
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
type BranchConfirmedEvent[ConflictID comparable] struct {
	// ID contains the identifier of the confirmed Conflict.
	ID ConflictID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchRejectedEvent //////////////////////////////////////////////////////////////////////////////////////////

// BranchRejectedEvent is a container that acts as a dictionary for the BranchRejected event related parameters.
type BranchRejectedEvent[ConflictID comparable] struct {
	// ID contains the identifier of the rejected Conflict.
	ID ConflictID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
