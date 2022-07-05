package tangle

import (
	"time"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/markers"
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
	MessageConfirmed *event.Event[*MessageConfirmedEvent]
	MessageOrphaned  *event.Event[*MessageConfirmedEvent]
}

// NewConfirmationEvents returns a new ConfirmationEvents object.
func NewConfirmationEvents() (new *ConfirmationEvents) {
	return &ConfirmationEvents{
		MessageConfirmed: event.New[*MessageConfirmedEvent](),
		MessageOrphaned:  event.New[*MessageConfirmedEvent](),
	}
}

type MessageConfirmedEvent struct {
	Message *Message
}

// region MessageFactoryEvents /////////////////////////////////////////////////////////////////////////////////////////

// MessageFactoryEvents represents events happening on a message factory.
type MessageFactoryEvents struct {
	// Fired when a message is built including tips, sequence number and other metadata.
	MessageConstructed *event.Event[*MessageConstructedEvent]

	// Fired when an error occurred.
	Error *event.Event[error]
}

// NewMessageFactoryEvents returns a new MessageFactoryEvents object.
func NewMessageFactoryEvents() (new *MessageFactoryEvents) {
	return &MessageFactoryEvents{
		MessageConstructed: event.New[*MessageConstructedEvent](),
		Error:              event.New[error](),
	}
}

type MessageConstructedEvent struct {
	Message *Message
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
	BranchID  utxo.TransactionID
}

type MarkerBranchAddedEvent struct {
	Marker      markers.Marker
	NewBranchID utxo.TransactionID
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

func NewSchedulerEvents() (new *SchedulerEvents) {
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
	NodeID identity.ID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ApprovalWeightManagerEvents //////////////////////////////////////////////////////////////////////////////////

// ApprovalWeightManagerEvents represents events happening in the ApprovalWeightManager.
type ApprovalWeightManagerEvents struct {
	// MessageProcessed is triggered once a message is finished being processed by the ApprovalWeightManager.
	MessageProcessed *event.Event[*MessageProcessedEvent]
	// BranchWeightChanged is triggered when a branch's weight changed.
	BranchWeightChanged *event.Event[*BranchWeightChangedEvent]
	// MarkerWeightChanged is triggered when a marker's weight changed.
	MarkerWeightChanged *event.Event[*MarkerWeightChangedEvent]
}

func newApprovalWeightManagerEvents() (new *ApprovalWeightManagerEvents) {
	return &ApprovalWeightManagerEvents{
		MessageProcessed:    event.New[*MessageProcessedEvent](),
		BranchWeightChanged: event.New[*BranchWeightChangedEvent](),
		MarkerWeightChanged: event.New[*MarkerWeightChangedEvent](),
	}
}

// MessageProcessedEvent holds information about a processed message.
type MessageProcessedEvent struct {
	MessageID MessageID
}

// MarkerWeightChangedEvent holds information about a marker and its updated weight.
type MarkerWeightChangedEvent struct {
	Marker markers.Marker
	Weight float64
}

// BranchWeightChangedEvent holds information about a branch and its updated weight.
type BranchWeightChangedEvent struct {
	BranchID utxo.TransactionID
	Weight   float64
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SolidifierEvents /////////////////////////////////////////////////////////////////////////////////////////////

// SolidifierEvents represents events happening in the Solidifier.
type SolidifierEvents struct {
	// MessageSolid is triggered when a message becomes solid, i.e. its past cone is known and solid.
	MessageSolid *event.Event[*MessageSolidEvent]

	// MessageMissing is triggered when a message references an unknown parent Message.
	MessageMissing *event.Event[*MessageMissingEvent]
}

func newSolidifierEvents() (new *SolidifierEvents) {
	return &SolidifierEvents{
		MessageSolid:   event.New[*MessageSolidEvent](),
		MessageMissing: event.New[*MessageMissingEvent](),
	}
}

type MessageSolidEvent struct {
	Message *Message
}

type MessageMissingEvent struct {
	MessageID MessageID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region StorageEvents ////////////////////////////////////////////////////////////////////////////////////////////////

// StorageEvents represents events happening on the message store.
type StorageEvents struct {
	// Fired when a message has been stored.
	MessageStored *event.Event[*MessageStoredEvent]

	// Fired when a message was removed from storage.
	MessageRemoved *event.Event[*MessageRemovedEvent]

	// Fired when a message which was previously marked as missing was received.
	MissingMessageStored *event.Event[*MissingMessageStoredEvent]
}

func newStorageEvents() (new *StorageEvents) {
	return &StorageEvents{
		MessageStored:        event.New[*MessageStoredEvent](),
		MessageRemoved:       event.New[*MessageRemovedEvent](),
		MissingMessageStored: event.New[*MissingMessageStoredEvent](),
	}
}

type MessageStoredEvent struct {
	Message *Message
}

type MessageRemovedEvent struct {
	MessageID MessageID
}

type MissingMessageStoredEvent struct {
	MessageID MessageID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region DispatcherEvents ////////////////////////////////////////////////////////////////////////////////////////////////

// DispatcherEvents represents events happening in the Dispatcher.
type DispatcherEvents struct {
	// MessageDispatched is triggered when a message is already scheduled and thus ready to be dispatched.
	MessageDispatched *event.Event[*MessageDispatchedEvent]
}

func newDispatcherEvents() (new *DispatcherEvents) {
	return &DispatcherEvents{
		MessageDispatched: event.New[*MessageDispatchedEvent](),
	}
}

type MessageDispatchedEvent struct {
	MessageID MessageID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ParserEvents /////////////////////////////////////////////////////////////////////////////////////////////////

// ParserEvents represents events happening in the Parser.
type ParserEvents struct {
	// Fired when a message was parsed.
	MessageParsed *event.Event[*MessageParsedEvent]

	// Fired when submitted bytes are rejected by a filter.
	BytesRejected *event.Event[*BytesRejectedEvent]

	// Fired when a message got rejected by a filter.
	MessageRejected *event.Event[*MessageRejectedEvent]
}

func newParserEvents() (new *ParserEvents) {
	return &ParserEvents{
		MessageParsed:   event.New[*MessageParsedEvent](),
		BytesRejected:   event.New[*BytesRejectedEvent](),
		MessageRejected: event.New[*MessageRejectedEvent](),
	}
}

// BytesRejectedEvent holds the information provided by the BytesRejected event that gets triggered when the bytes of a
// Message did not pass the parsing step.
type BytesRejectedEvent struct {
	Bytes []byte
	Peer  *peer.Peer
	Error error
}

// MessageRejectedEvent holds the information provided by the MessageRejected event that gets triggered when the Message
// was detected to be invalid.
type MessageRejectedEvent struct {
	Message *Message
	Peer    *peer.Peer
	Error   error
}

// MessageParsedEvent holds the information provided by the MessageParsed event that gets triggered when a message was
// fully parsed and syntactically validated.
type MessageParsedEvent struct {
	// Message contains the parsed Message.
	Message *Message

	// Peer contains the node that sent this Message to the node.
	Peer *peer.Peer
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region RequesterEvents //////////////////////////////////////////////////////////////////////////////////////////////

// RequesterEvents represents events happening on a message requester.
type RequesterEvents struct {
	// RequestIssued is an event that is triggered when the requester wants to request the given Message from its
	// neighbors.
	RequestIssued *event.Event[*RequestIssuedEvent]

	// RequestStarted is an event that is triggered when a new request is started.
	RequestStarted *event.Event[*RequestStartedEvent]

	// RequestStopped is an event that is triggered when a request is stopped.
	RequestStopped *event.Event[*RequestStoppedEvent]

	// RequestFailed is an event that is triggered when a request is stopped after too many attempts.
	RequestFailed *event.Event[*RequestFailedEvent]
}

func newRequesterEvents() (new *RequesterEvents) {
	return &RequesterEvents{
		RequestIssued:  event.New[*RequestIssuedEvent](),
		RequestStarted: event.New[*RequestStartedEvent](),
		RequestStopped: event.New[*RequestStoppedEvent](),
		RequestFailed:  event.New[*RequestFailedEvent](),
	}
}

type RequestIssuedEvent struct {
	MessageID MessageID
}

type RequestStartedEvent struct {
	MessageID MessageID
}

type RequestStoppedEvent struct {
	MessageID MessageID
}

type RequestFailedEvent struct {
	MessageID MessageID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TimeManagerEvents ////////////////////////////////////////////////////////////////////////////////////////////

// TimeManagerEvents represents events happening in the TimeManager.
type TimeManagerEvents struct {
	// Fired when the nodes sync status changes.
	SyncChanged           *event.Event[*SyncChangedEvent]
	AcceptanceTimeUpdated *event.Event[*TimeUpdate]
	ConfirmedTimeUpdated  *event.Event[*TimeUpdate]
}

func newTimeManagerEvents() (new *TimeManagerEvents) {
	return &TimeManagerEvents{
		SyncChanged:           event.New[*SyncChangedEvent](),
		AcceptanceTimeUpdated: event.New[*TimeUpdate](),
		ConfirmedTimeUpdated:  event.New[*TimeUpdate](),
	}
}

// SyncChangedEvent represents a sync changed event.
type SyncChangedEvent struct {
	Synced bool
}

// TimeUpdate represents an update in Tangle Time.
type TimeUpdate struct {
	// MessageID is the ID of the message that caused the time update.
	MessageID  MessageID
	// ATT is the new Acceptance Tangle Time.
	ATT        time.Time
	// UpdateTime is the wall clock time when the update has occurred.
	UpdateTime time.Time
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TipManagerEvents /////////////////////////////////////////////////////////////////////////////////////////////

// TipManagerEvents represents events happening on the TipManager.
type TipManagerEvents struct {
	// Fired when a tip is added.
	TipAdded *event.Event[*TipEvent]

	// Fired when a tip is removed.
	TipRemoved *event.Event[*TipEvent]
}

func newTipManagerEvents() (new *TipManagerEvents) {
	return &TipManagerEvents{
		TipAdded:   event.New[*TipEvent](),
		TipRemoved: event.New[*TipEvent](),
	}
}

// TipEvent holds the information provided by the TipEvent event that gets triggered when a message gets added or
// removed as tip.
type TipEvent struct {
	// MessageID of the added/removed tip.
	MessageID MessageID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
