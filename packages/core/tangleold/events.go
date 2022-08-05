package tangleold

import (
	"time"

	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/markers"
)

// region Events ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Events represents events happening in the Tangle.
type Events struct {
	// BlockInvalid is triggered when a Block is detected to be objectively invalid.
	BlockInvalid *event.Event[*BlockInvalidEvent]

	// Error is triggered when the Tangle faces an error from which it can not recover.
	Error *event.Event[error]
}

// newEvents returns a new Events object.
func newEvents() (new *Events) {
	return &Events{
		BlockInvalid: event.New[*BlockInvalidEvent](),
		Error:        event.New[error](),
	}
}

// BlockInvalidEvent is struct that is passed along with triggering a blockInvalidEvent.
type BlockInvalidEvent struct {
	BlockID BlockID
	Error   error
}

// ConfirmationEvents are events entailing confirmation.
type ConfirmationEvents struct {
	BlockAccepted *event.Event[*BlockAcceptedEvent]
	BlockOrphaned *event.Event[*BlockAcceptedEvent]
}

// NewConfirmationEvents returns a new ConfirmationEvents object.
func NewConfirmationEvents() (new *ConfirmationEvents) {
	return &ConfirmationEvents{
		BlockAccepted: event.New[*BlockAcceptedEvent](),
		BlockOrphaned: event.New[*BlockAcceptedEvent](),
	}
}

type BlockAcceptedEvent struct {
	Block *Block
}

// region BlockFactoryEvents /////////////////////////////////////////////////////////////////////////////////////////

// BlockFactoryEvents represents events happening on a block factory.
type BlockFactoryEvents struct {
	// Fired when a block is built including tips, sequence number and other metadata.
	BlockConstructed *event.Event[*BlockConstructedEvent]

	// Fired when an error occurred.
	Error *event.Event[error]
}

// NewBlockFactoryEvents returns a new BlockFactoryEvents object.
func NewBlockFactoryEvents() (new *BlockFactoryEvents) {
	return &BlockFactoryEvents{
		BlockConstructed: event.New[*BlockConstructedEvent](),
		Error:            event.New[error](),
	}
}

type BlockConstructedEvent struct {
	Block *Block
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BookerEvents /////////////////////////////////////////////////////////////////////////////////////////////////

// BookerEvents represents events happening in the Booker.
type BookerEvents struct {
	// BlockBooked is triggered when a Block was booked (it's Conflict, and it's Payload's Conflict were determined).
	BlockBooked *event.Event[*BlockBookedEvent]

	// BlockConflictUpdated is triggered when the ConflictID of a Block is changed in its BlockMetadata.
	BlockConflictUpdated *event.Event[*BlockConflictUpdatedEvent]

	// MarkerConflictAdded is triggered when a Marker is mapped to a new ConflictID.
	MarkerConflictAdded *event.Event[*MarkerConflictAddedEvent]

	// Error gets triggered when the Booker faces an unexpected error.
	Error *event.Event[error]
}

func NewBookerEvents() (new *BookerEvents) {
	return &BookerEvents{
		BlockBooked:          event.New[*BlockBookedEvent](),
		BlockConflictUpdated: event.New[*BlockConflictUpdatedEvent](),
		MarkerConflictAdded:  event.New[*MarkerConflictAddedEvent](),
		Error:                event.New[error](),
	}
}

type BlockBookedEvent struct {
	BlockID BlockID
}

type BlockConflictUpdatedEvent struct {
	BlockID    BlockID
	ConflictID utxo.TransactionID
}

type MarkerConflictAddedEvent struct {
	Marker        markers.Marker
	NewConflictID utxo.TransactionID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SchedulerEvents /////////////////////////////////////////////////////////////////////////////////////////////

// SchedulerEvents represents events happening in the Scheduler.
type SchedulerEvents struct {
	// BlockScheduled is triggered when a block is ready to be scheduled.
	BlockScheduled *event.Event[*BlockScheduledEvent]
	// BlockDiscarded is triggered when a block is removed from the longest mana-scaled queue when the buffer is full.
	BlockDiscarded *event.Event[*BlockDiscardedEvent]
	// BlockSkipped is triggered when a block is confirmed before it's scheduled, and is skipped by the scheduler.
	BlockSkipped    *event.Event[*BlockSkippedEvent]
	NodeBlacklisted *event.Event[*NodeBlacklistedEvent]
	Error           *event.Event[error]
}

func NewSchedulerEvents() (new *SchedulerEvents) {
	return &SchedulerEvents{
		BlockScheduled:  event.New[*BlockScheduledEvent](),
		BlockDiscarded:  event.New[*BlockDiscardedEvent](),
		BlockSkipped:    event.New[*BlockSkippedEvent](),
		NodeBlacklisted: event.New[*NodeBlacklistedEvent](),
		Error:           event.New[error](),
	}
}

type BlockScheduledEvent struct {
	BlockID BlockID
}

type BlockDiscardedEvent struct {
	BlockID BlockID
}

type BlockSkippedEvent struct {
	BlockID BlockID
}

type NodeBlacklistedEvent struct {
	NodeID identity.ID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ApprovalWeightManagerEvents //////////////////////////////////////////////////////////////////////////////////

// ApprovalWeightManagerEvents represents events happening in the ApprovalWeightManager.
type ApprovalWeightManagerEvents struct {
	// BlockProcessed is triggered once a block is finished being processed by the ApprovalWeightManager.
	BlockProcessed *event.Event[*BlockProcessedEvent]
	// ConflictWeightChanged is triggered when a conflict's weight changed.
	ConflictWeightChanged *event.Event[*ConflictWeightChangedEvent]
	// MarkerWeightChanged is triggered when a marker's weight changed.
	MarkerWeightChanged *event.Event[*MarkerWeightChangedEvent]
}

func newApprovalWeightManagerEvents() (new *ApprovalWeightManagerEvents) {
	return &ApprovalWeightManagerEvents{
		BlockProcessed:        event.New[*BlockProcessedEvent](),
		ConflictWeightChanged: event.New[*ConflictWeightChangedEvent](),
		MarkerWeightChanged:   event.New[*MarkerWeightChangedEvent](),
	}
}

// BlockProcessedEvent holds information about a processed block.
type BlockProcessedEvent struct {
	BlockID BlockID
}

// MarkerWeightChangedEvent holds information about a marker and its updated weight.
type MarkerWeightChangedEvent struct {
	Marker markers.Marker
	Weight float64
}

// ConflictWeightChangedEvent holds information about a conflict and its updated weight.
type ConflictWeightChangedEvent struct {
	ConflictID utxo.TransactionID
	Weight     float64
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SolidifierEvents /////////////////////////////////////////////////////////////////////////////////////////////

// SolidifierEvents represents events happening in the Solidifier.
type SolidifierEvents struct {
	// BlockSolid is triggered when a block becomes solid, i.e. its past cone is known and solid.
	BlockSolid *event.Event[*BlockSolidEvent]

	// BlockMissing is triggered when a block references an unknown parent Block.
	BlockMissing *event.Event[*BlockMissingEvent]
}

func newSolidifierEvents() (new *SolidifierEvents) {
	return &SolidifierEvents{
		BlockSolid:   event.New[*BlockSolidEvent](),
		BlockMissing: event.New[*BlockMissingEvent](),
	}
}

type BlockSolidEvent struct {
	Block *Block
}

type BlockMissingEvent struct {
	BlockID BlockID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region StorageEvents ////////////////////////////////////////////////////////////////////////////////////////////////

// StorageEvents represents events happening on the block store.
type StorageEvents struct {
	// Fired when a block has been stored.
	BlockStored *event.Event[*BlockStoredEvent]

	// Fired when a block was removed from storage.
	BlockRemoved *event.Event[*BlockRemovedEvent]

	// Fired when a block which was previously marked as missing was received.
	MissingBlockStored *event.Event[*MissingBlockStoredEvent]
}

func newStorageEvents() (new *StorageEvents) {
	return &StorageEvents{
		BlockStored:        event.New[*BlockStoredEvent](),
		BlockRemoved:       event.New[*BlockRemovedEvent](),
		MissingBlockStored: event.New[*MissingBlockStoredEvent](),
	}
}

type BlockStoredEvent struct {
	Block *Block
}

type BlockRemovedEvent struct {
	BlockID BlockID
}

type MissingBlockStoredEvent struct {
	BlockID BlockID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region DispatcherEvents ////////////////////////////////////////////////////////////////////////////////////////////////

// DispatcherEvents represents events happening in the Dispatcher.
type DispatcherEvents struct {
	// BlockDispatched is triggered when a block is already scheduled and thus ready to be dispatched.
	BlockDispatched *event.Event[*BlockDispatchedEvent]
}

func newDispatcherEvents() (new *DispatcherEvents) {
	return &DispatcherEvents{
		BlockDispatched: event.New[*BlockDispatchedEvent](),
	}
}

type BlockDispatchedEvent struct {
	BlockID BlockID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ParserEvents /////////////////////////////////////////////////////////////////////////////////////////////////

// ParserEvents represents events happening in the Parser.
type ParserEvents struct {
	// Fired when a block was parsed.
	BlockParsed *event.Event[*BlockParsedEvent]

	// Fired when submitted bytes are rejected by a filter.
	BytesRejected *event.Event[*BytesRejectedEvent]

	// Fired when a block got rejected by a filter.
	BlockRejected *event.Event[*BlockRejectedEvent]
}

func newParserEvents() (new *ParserEvents) {
	return &ParserEvents{
		BlockParsed:   event.New[*BlockParsedEvent](),
		BytesRejected: event.New[*BytesRejectedEvent](),
		BlockRejected: event.New[*BlockRejectedEvent](),
	}
}

// BytesRejectedEvent holds the information provided by the BytesRejected event that gets triggered when the bytes of a
// Block did not pass the parsing step.
type BytesRejectedEvent struct {
	Bytes []byte
	Peer  *peer.Peer
	Error error
}

// BlockRejectedEvent holds the information provided by the BlockRejected event that gets triggered when the Block
// was detected to be invalid.
type BlockRejectedEvent struct {
	Block *Block
	Peer  *peer.Peer
	Error error
}

// BlockParsedEvent holds the information provided by the BlockParsed event that gets triggered when a block was
// fully parsed and syntactically validated.
type BlockParsedEvent struct {
	// Block contains the parsed Block.
	Block *Block

	// Peer contains the node that sent this Block to the node.
	Peer *peer.Peer
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region RequesterEvents //////////////////////////////////////////////////////////////////////////////////////////////

// RequesterEvents represents events happening on a block requester.
type RequesterEvents struct {
	// RequestIssued is an event that is triggered when the requester wants to request the given Block from its
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
	BlockID BlockID
}

type RequestStartedEvent struct {
	BlockID BlockID
}

type RequestStoppedEvent struct {
	BlockID BlockID
}

type RequestFailedEvent struct {
	BlockID BlockID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TimeManagerEvents ////////////////////////////////////////////////////////////////////////////////////////////

// TimeManagerEvents represents events happening in the TimeManager.
type TimeManagerEvents struct {
	// Fired when the nodes sync status changes.
	SyncChanged           *event.Event[*SyncChangedEvent]
	Bootstrapped          *event.Event[*BootstrappedEvent]
	AcceptanceTimeUpdated *event.Event[*TimeUpdate]
	ConfirmedTimeUpdated  *event.Event[*TimeUpdate]
}

func newTimeManagerEvents() (new *TimeManagerEvents) {
	return &TimeManagerEvents{
		SyncChanged:           event.New[*SyncChangedEvent](),
		AcceptanceTimeUpdated: event.New[*TimeUpdate](),
		ConfirmedTimeUpdated:  event.New[*TimeUpdate](),
		Bootstrapped:          event.New[*BootstrappedEvent](),
	}
}

// SyncChangedEvent represents a sync changed event.
type SyncChangedEvent struct {
	Synced bool
}

// BootstrappedEvent represents a bootstrapped event.
type BootstrappedEvent struct{}

// TimeUpdate represents an update in Tangle Time.
type TimeUpdate struct {
	// BlockID is the ID of the block that caused the time update.
	BlockID BlockID
	// ATT is the new Acceptance Tangle Time.
	ATT time.Time
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

// TipEvent holds the information provided by the TipEvent event that gets triggered when a block gets added or
// removed as tip.
type TipEvent struct {
	// BlockID of the added/removed tip.
	BlockID BlockID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
