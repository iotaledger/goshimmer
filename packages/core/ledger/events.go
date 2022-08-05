package ledger

import (
	"context"
	"time"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/set"

	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
)

// region Events ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Events is a container that acts as a dictionary for the existing events of a Ledger.
type Events struct {
	// TransactionStored is an event that gets triggered whenever a new Transaction is stored.
	TransactionStored *event.Event[*TransactionStoredEvent]

	// TransactionBooked is an event that gets triggered whenever a Transaction is booked.
	TransactionBooked *event.Event[*TransactionBookedEvent]

	// TransactionInclusionUpdated is an event that gets triggered whenever the inclusion time of a Transaction changes.
	TransactionInclusionUpdated *event.Event[*TransactionInclusionUpdatedEvent]

	// TransactionAccepted is an event that gets triggered whenever a Transaction is confirmed.
	TransactionAccepted *event.Event[*TransactionAcceptedEvent]

	// TransactionRejected is an event that gets triggered whenever a Transaction is rejected.
	TransactionRejected *event.Event[*TransactionRejectedEvent]

	// TransactionForked is an event that gets triggered whenever a Transaction is forked.
	TransactionForked *event.Event[*TransactionForkedEvent]

	// TransactionConflictIDUpdated is an event that gets triggered whenever the Conflict of a Transaction is updated.
	TransactionConflictIDUpdated *event.Event[*TransactionConflictIDUpdatedEvent]

	// TransactionInvalid is an event that gets triggered whenever a Transaction is found to be invalid.
	TransactionInvalid *event.Event[*TransactionInvalidEvent]

	// Error is event that gets triggered whenever an error occurs while processing a Transaction.
	Error *event.Event[error]
}

// newEvents returns a new Events object.
func newEvents() (new *Events) {
	return &Events{
		TransactionStored:            event.New[*TransactionStoredEvent](),
		TransactionBooked:            event.New[*TransactionBookedEvent](),
		TransactionInclusionUpdated:  event.New[*TransactionInclusionUpdatedEvent](),
		TransactionAccepted:          event.New[*TransactionAcceptedEvent](),
		TransactionRejected:          event.New[*TransactionRejectedEvent](),
		TransactionForked:            event.New[*TransactionForkedEvent](),
		TransactionConflictIDUpdated: event.New[*TransactionConflictIDUpdatedEvent](),
		TransactionInvalid:           event.New[*TransactionInvalidEvent](),
		Error:                        event.New[error](),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionStoredEvent ///////////////////////////////////////////////////////////////////////////////////////

// TransactionStoredEvent is a container that acts as a dictionary for the TransactionStored event related parameters.
type TransactionStoredEvent struct {
	// TransactionID contains the identifier of the stored Transaction.
	TransactionID utxo.TransactionID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionBookedEvent ///////////////////////////////////////////////////////////////////////////////////////

// TransactionBookedEvent is a container that acts as a dictionary for the TransactionBooked event related parameters.
type TransactionBookedEvent struct {
	// TransactionID contains the identifier of the booked Transaction.
	TransactionID utxo.TransactionID

	// Outputs contains the set of Outputs that this Transaction created.
	Outputs *utxo.Outputs

	// Context contains a Context provided by the caller that triggered this event.
	Context context.Context
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionInclusionUpdatedEvent /////////////////////////////////////////////////////////////////////////////

// TransactionInclusionUpdatedEvent is a container that acts as a dictionary for the TransactionInclusionUpdated event
// related parameters.
type TransactionInclusionUpdatedEvent struct {
	// TransactionID contains the identifier of the booked Transaction.
	TransactionID utxo.TransactionID

	// InclusionTime contains the InclusionTime after it was updated.
	InclusionTime time.Time

	// PreviousInclusionTime contains the InclusionTime before it was updated.
	PreviousInclusionTime time.Time
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionAcceptedEvent /////////////////////////////////////////////////////////////////////////////////////

// TransactionAcceptedEvent is a container that acts as a dictionary for the TransactionAccepted event related
// parameters.
type TransactionAcceptedEvent struct {
	// TransactionID contains the identifier of the confirmed Transaction.
	TransactionID utxo.TransactionID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionRejectedEvent /////////////////////////////////////////////////////////////////////////////////////

// TransactionRejectedEvent is a container that acts as a dictionary for the TransactionRejected event related
// parameters.
type TransactionRejectedEvent struct {
	// TransactionID contains the identifier of the rejected Transaction.
	TransactionID utxo.TransactionID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionForkedEvent ///////////////////////////////////////////////////////////////////////////////////////

// TransactionForkedEvent is a container that acts as a dictionary for the TransactionForked event related parameters.
type TransactionForkedEvent struct {
	// TransactionID contains the identifier of the forked Transaction.
	TransactionID utxo.TransactionID

	// ParentConflicts contains the set of ConflictIDs that form the parent Conflicts for the newly forked Transaction.
	ParentConflicts *set.AdvancedSet[utxo.TransactionID]

	// Context contains a Context provided by the caller that triggered this event.
	Context context.Context
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionConflictIDUpdatedEvent //////////////////////////////////////////////////////////////////////////////

// TransactionConflictIDUpdatedEvent is a container that acts as a dictionary for the TransactionConflictIDUpdated event
// related parameters.
type TransactionConflictIDUpdatedEvent struct {
	// TransactionID contains the identifier of the Transaction whose ConflictIDs were updated.
	TransactionID utxo.TransactionID

	// AddedConflictID contains the identifier of the Conflict that was added to the ConflictIDs of the Transaction.
	AddedConflictID utxo.TransactionID

	// RemovedConflictIDs contains the set of the ConflictIDs that were removed while updating the Transaction.
	RemovedConflictIDs *set.AdvancedSet[utxo.TransactionID]

	// Context contains a Context provided by the caller that triggered this event.
	Context context.Context
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionInvalidEvent //////////////////////////////////////////////////////////////////////////////////////

// TransactionInvalidEvent is a container that acts as a dictionary for the TransactionInvalid event related parameters.
type TransactionInvalidEvent struct {
	// TransactionID contains the identifier of the Transaction that was found to be invalid.
	TransactionID utxo.TransactionID

	// Reason contains the error that caused the Transaction to be considered invalid.
	Reason error

	// Context contains a Context provided by the caller that triggered this event.
	Context context.Context
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
