package tangle

import (
	"github.com/iotaledger/goshimmer/packages/ledger/branchdag"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/identity"
)

// region Events ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Events represents events happening in the Tangle.
type Events struct {
	// MessageInvalid is triggered when a Message is detected to be objectively invalid.
	MessageInvalid *event.Event[*MessageInvalidEvent]

	// Error is triggered when the Tangle faces an error from which it can not recover.
	Error *event.Event[error]
}

// newEvents returns a new Events object.
func newEvents() (new *Events) {
	return &Events{
		MessageInvalid: event.New[*MessageInvalidEvent](),
		Error:          event.New[error](),
	}
}

// MessageInvalidEvent is struct that is passed along with triggering a messageInvalidEvent.
type MessageInvalidEvent struct {
	MessageID MessageID
	Error     error
}

// ConfirmationEvents are events entailing confirmation.
type ConfirmationEvents struct {
	MessageConfirmed     *event.Event[*MessageConfirmedEvent]
	BranchConfirmed      *event.Event[*BranchConfirmedEvent]
	TransactionConfirmed *event.Event[*TransactionConfirmedEvent]
}

// NewConfirmationEvents returns a new ConfirmationEvents object.
func NewConfirmationEvents() (new *ConfirmationEvents) {
	return &ConfirmationEvents{
		MessageConfirmed:     event.New[*MessageConfirmedEvent](),
		BranchConfirmed:      event.New[*BranchConfirmedEvent](),
		TransactionConfirmed: event.New[*TransactionConfirmedEvent](),
	}
}

type MessageConfirmedEvent struct {
	MessageID MessageID
}

type BranchConfirmedEvent struct {
	BranchID branchdag.BranchID
}

type TransactionConfirmedEvent struct {
	TransactionID utxo.TransactionID
}

// region MessageFactoryEvents /////////////////////////////////////////////////////////////////////////////////////////

// MessageFactoryEvents represents events happening on a message factory.
type MessageFactoryEvents struct {
	// Fired when a message is built including tips, sequence number and other metadata.
	MessageConstructed *event.Event[*MessageConstructedEvent]

	// MessageReferenceImpossible is fired when references for a message can't be constructed and the message can never become a parent.
	MessageReferenceImpossible *event.Event[*MessageReferenceImpossibleEvent]

	// Fired when an error occurred.
	Error *event.Event[error]
}

// NewConfirmationEvents returns a new MessageFactoryEvents object.
func NewMessageFactoryEvents() (new *MessageFactoryEvents) {
	return &MessageFactoryEvents{
		MessageConstructed:         event.New[*MessageConstructedEvent](),
		MessageReferenceImpossible: event.New[*MessageReferenceImpossibleEvent](),
		Error:                      event.New[error](),
	}
}

type MessageConstructedEvent struct {
	Message *Message
}

type MessageReferenceImpossibleEvent struct {
	MessageID MessageID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BookerEvents /////////////////////////////////////////////////////////////////////////////////////////////////

// BookerEvents represents events happening in the Booker.
type BookerEvents struct {
	// MessageBooked is triggered when a Message was booked (it's Branch, and it's Payload's Branch were determined).
	MessageBooked *event.Event[*MessageBookedEvent]

	// MessageBranchUpdated is triggered when the BranchID of a Message is changed in its MessageMetadata.
	MessageBranchUpdated *event.Event[*MessageBranchUpdatedEvent]

	// MarkerBranchAdded is triggered when a Marker is mapped to a new BranchID.
	MarkerBranchAdded *event.Event[*MarkerBranchAddedEvent]

	// Error gets triggered when the Booker faces an unexpected error.
	Error *event.Event[error]
}

func NewBookerEvents() (new *BookerEvents) {
	return &BookerEvents{
		MessageBooked:        event.New[*MessageBookedEvent](),
		MessageBranchUpdated: event.New[*MessageBranchUpdatedEvent](),
		MarkerBranchAdded:    event.New[*MarkerBranchAddedEvent](),
		Error:                event.New[error](),
	}
}

type MessageBookedEvent struct {
	MessageID MessageID
}

type MessageBranchUpdatedEvent struct {
	MessageID MessageID
	BranchID  branchdag.BranchID
}

type MarkerBranchAddedEvent struct {
	Marker       *markers.Marker
	OldBranchIDs branchdag.BranchIDs
	NewBranchIDs branchdag.BranchIDs
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SchedulerEvents /////////////////////////////////////////////////////////////////////////////////////////////

// SchedulerEvents represents events happening in the Scheduler.
type SchedulerEvents struct {
	// MessageScheduled is triggered when a message is ready to be scheduled.
	MessageScheduled *event.Event[*MessageScheduledEvent]
	// MessageDiscarded is triggered when a message is removed from the longest mana-scaled queue when the buffer is full.
	MessageDiscarded *event.Event[*MessageDiscardedEvent]
	// MessageSkipped is triggered when a message is confirmed before it's scheduled, and is skipped by the scheduler.
	MessageSkipped  *event.Event[*MessageSkippedEvent]
	NodeBlacklisted *event.Event[*NodeBlacklistedEvent]
	Error           *event.Event[error]
}

func NewSchedulerEvent() (new *SchedulerEvents) {
	return &SchedulerEvents{
		MessageScheduled: event.New[*MessageScheduledEvent](),
		MessageDiscarded: event.New[*MessageDiscardedEvent](),
		MessageSkipped:   event.New[*MessageSkippedEvent](),
		NodeBlacklisted:  event.New[*NodeBlacklistedEvent](),
		Error:            event.New[error](),
	}
}

type MessageScheduledEvent struct {
	MessageID MessageID
}

type MessageDiscardedEvent struct {
	MessageID MessageID
}

type MessageSkippedEvent struct {
	MessageID MessageID
}

type NodeBlacklistedEvent struct {
	nodeID identity.ID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
