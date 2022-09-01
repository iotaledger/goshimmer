package conflictdag

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/set"
)

// region Events ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Events is a container that acts as a dictionary for the events of a ConflictDAG.
type Events[ConflictID, ConflictingResourceID comparable] struct {
	// ConflictCreated is an event that gets triggered whenever a new Conflict is created.
	ConflictCreated *event.Event[*ConflictCreatedEvent[ConflictID, ConflictingResourceID]]

	// ConflictConflictsUpdated is an event that gets triggered whenever the ConflictIDs of a Conflict are updated.
	ConflictConflictsUpdated *event.Event[*ConflictConflictsUpdatedEvent[ConflictID, ConflictingResourceID]]

	// ConflictParentsUpdated is an event that gets triggered whenever the parent ConflictIDs of a Conflict are updated.
	ConflictParentsUpdated *event.Event[*ConflictParentsUpdatedEvent[ConflictID, ConflictingResourceID]]

	// ConflictAccepted is an event that gets triggered whenever a Conflict is confirmed.
	ConflictAccepted *event.Event[*ConflictAcceptedEvent[ConflictID]]

	// ConflictRejected is an event that gets triggered whenever a Conflict is rejected.
	ConflictRejected *event.Event[*ConflictRejectedEvent[ConflictID]]
}

// newEvents returns a new Events object.
func newEvents[ConflictID, ConflictingResourceID comparable]() *Events[ConflictID, ConflictingResourceID] {
	return &Events[ConflictID, ConflictingResourceID]{
		ConflictCreated:          event.New[*ConflictCreatedEvent[ConflictID, ConflictingResourceID]](),
		ConflictConflictsUpdated: event.New[*ConflictConflictsUpdatedEvent[ConflictID, ConflictingResourceID]](),
		ConflictParentsUpdated:   event.New[*ConflictParentsUpdatedEvent[ConflictID, ConflictingResourceID]](),
		ConflictAccepted:         event.New[*ConflictAcceptedEvent[ConflictID]](),
		ConflictRejected:         event.New[*ConflictRejectedEvent[ConflictID]](),
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

// region ConflictConflictsUpdatedEvent //////////////////////////////////////////////////////////////////////////////////

// ConflictConflictsUpdatedEvent is a container that acts as a dictionary for the ConflictConflictsUpdated event related
// parameters.
type ConflictConflictsUpdatedEvent[ConflictID, ConflictingResourceID comparable] struct {
	// ConflictID contains the identifier of the updated Conflict.
	ConflictID ConflictID

	// NewConflictIDs contains the set of conflicts that this Conflict was added to.
	NewConflictIDs *set.AdvancedSet[ConflictingResourceID]
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ConflictParentsUpdatedEvent ////////////////////////////////////////////////////////////////////////////////////

// ConflictParentsUpdatedEvent is a container that acts as a dictionary for the ConflictParentsUpdated event related
// parameters.
type ConflictParentsUpdatedEvent[ConflictID, ConflictingResourceID comparable] struct {
	// ConflictID contains the identifier of the updated Conflict.
	ConflictID ConflictID

	// AddedConflict contains the forked parent Conflict that replaces the removed parents.
	AddedConflict ConflictID

	// RemovedConflicts contains the parent ConflictIDs that were replaced by the newly forked Conflict.
	RemovedConflicts *set.AdvancedSet[ConflictID]

	// ParentsConflictIDs contains the updated list of parent ConflictIDs.
	ParentsConflictIDs *set.AdvancedSet[ConflictID]
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ConflictAcceptedEvent /////////////////////////////////////////////////////////////////////////////////////////

// ConflictAcceptedEvent is a container that acts as a dictionary for the ConflictAccepted event related parameters.
type ConflictAcceptedEvent[ConflictID comparable] struct {
	// ID contains the identifier of the confirmed Conflict.
	ID ConflictID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ConflictRejectedEvent //////////////////////////////////////////////////////////////////////////////////////////

// ConflictRejectedEvent is a container that acts as a dictionary for the ConflictRejected event related parameters.
type ConflictRejectedEvent[ConflictID comparable] struct {
	// ID contains the identifier of the rejected Conflict.
	ID ConflictID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
