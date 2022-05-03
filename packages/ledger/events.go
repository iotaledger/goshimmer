package ledger

import (
	"context"
	"time"

	"github.com/iotaledger/hive.go/generics/event"

	"github.com/iotaledger/goshimmer/packages/ledger/branchdag"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
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

	// TransactionConfirmed is an event that gets triggered whenever a Transaction is confirmed.
	TransactionConfirmed *event.Event[*TransactionConfirmedEvent]

	// TransactionRejected is an event that gets triggered whenever a Transaction is rejected.
	TransactionRejected *event.Event[*TransactionRejectedEvent]

	// TransactionForked is an event that gets triggered whenever a Transaction is forked.
	TransactionForked *event.Event[*TransactionForkedEvent]

	// TransactionBranchIDUpdated is an event that gets triggered whenever the Branch of a Transaction is updated.
	TransactionBranchIDUpdated *event.Event[*TransactionBranchIDUpdatedEvent]

	// TransactionInvalid is an event that gets triggered whenever a Transaction is found to be invalid.
	TransactionInvalid *event.Event[*TransactionInvalidEvent]

	// Error is event that gets triggered whenever an error occurs while processing a Transaction.
	Error *event.Event[error]
}

// newEvents returns a new Events object.
func newEvents() (new *Events) {
	return &Events{
		TransactionStored:           event.New[*TransactionStoredEvent](),
		TransactionBooked:           event.New[*TransactionBookedEvent](),
		TransactionInclusionUpdated: event.New[*TransactionInclusionUpdatedEvent](),
		TransactionConfirmed:        event.New[*TransactionConfirmedEvent](),
		TransactionRejected:         event.New[*TransactionRejectedEvent](),
		TransactionForked:           event.New[*TransactionForkedEvent](),
		TransactionBranchIDUpdated:  event.New[*TransactionBranchIDUpdatedEvent](),
		TransactionInvalid:          event.New[*TransactionInvalidEvent](),
		Error:                       event.New[error](),
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
	Outputs utxo.Outputs

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

// region TransactionConfirmedEvent ////////////////////////////////////////////////////////////////////////////////////

// TransactionConfirmedEvent is a container that acts as a dictionary for the TransactionConfirmed event related
// parameters.
type TransactionConfirmedEvent struct {
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

	// ParentBranches contains the set of BranchIDs that form the parent Branches for the newly forked Transaction.
	ParentBranches branchdag.BranchIDs

	// ForkedBranchID contains the newly forked BranchID that trigger this event.
	ForkedBranchID branchdag.BranchID

	// Context contains a Context provided by the caller that triggered this event.
	Context context.Context
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionBranchIDUpdatedEvent //////////////////////////////////////////////////////////////////////////////

// TransactionBranchIDUpdatedEvent is a container that acts as a dictionary for the TransactionBranchIDUpdated event
// related parameters.
type TransactionBranchIDUpdatedEvent struct {
	// TransactionID contains the identifier of the Transaction whose BranchIDs were updated.
	TransactionID utxo.TransactionID

	// AddedBranchID contains the identifier of the Branch that was added to the BranchIDs of the Transaction.
	AddedBranchID branchdag.BranchID

	// RemovedBranchIDs contains the set of the BranchIDs that were removed while updating the Transaction.
	RemovedBranchIDs branchdag.BranchIDs

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