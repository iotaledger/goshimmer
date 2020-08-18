package tangle

import (
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/branchmanager"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/hive.go/events"
)

// Events is a container for the different kind of events of the Tangle.
type Events struct {
	// Get's called whenever a transaction
	PayloadAttached        *events.Event
	PayloadSolid           *events.Event
	PayloadLiked           *events.Event
	PayloadConfirmed       *events.Event
	PayloadRejected        *events.Event
	PayloadDisliked        *events.Event
	MissingPayloadReceived *events.Event
	PayloadMissing         *events.Event
	PayloadInvalid         *events.Event

	// TransactionReceived gets triggered whenever a transaction was received for the first time (not solid yet).
	TransactionReceived *events.Event

	// TransactionInvalid gets triggered whenever we receive an invalid transaction.
	TransactionInvalid *events.Event

	// TransactionSolid gets triggered whenever a transaction becomes solid for the first time.
	TransactionSolid *events.Event

	// TransactionBooked gets triggered whenever a transactions becomes solid and gets booked into a particular branch.
	TransactionBooked *events.Event

	TransactionPreferred *events.Event

	TransactionUnpreferred *events.Event

	TransactionLiked *events.Event

	TransactionDisliked *events.Event

	TransactionConfirmed *events.Event

	TransactionRejected *events.Event

	TransactionFinalized *events.Event

	// Fork gets triggered when a previously un-conflicting transaction get's some of its inputs double spend, so that a
	// new Branch is created.
	Fork *events.Event

	Error *events.Event
}

// EventSource is a type that contains information from where a specific change was triggered (the branch manager or
// the tangle).
type EventSource int

const (
	// EventSourceTangle indicates that a change was issued by the Tangle.
	EventSourceTangle EventSource = iota

	// EventSourceBranchManager indicates that a change was issued by the BranchManager.
	EventSourceBranchManager
)

func (eventSource EventSource) String() string {
	return [...]string{"EventSourceTangle", "EventSourceBranchManager"}[eventSource]
}

// CachedPayloadEvent represents the parameters of cachedPayloadEvent
type CachedPayloadEvent struct {
	Payload         *payload.CachedPayload
	PayloadMetadata *CachedPayloadMetadata
}

// CachedTxnEvent represents the parameters of cachedTxnEvent
type CachedTxnEvent struct {
	Txn         *transaction.CachedTransaction
	TxnMetadata *CachedTransactionMetadata
}

// CachedTxnBookEvent represents the parameters of transactionBookedEvent
type CachedTxnBookEvent struct {
	Txn         *transaction.CachedTransaction
	TxnMetadata *CachedTransactionMetadata
	Pending     bool
}

// ForkEvent represents the parameters of forkEvent
type ForkEvent struct {
	Txn         *transaction.CachedTransaction
	TxnMetadata *CachedTransactionMetadata
	Branch      *branchmanager.CachedBranch
	OutputIDs   []transaction.OutputID
}

// CachedAttachmentsEvent represents the parameters of cachedTransactionAttachmentEvent
type CachedAttachmentsEvent struct {
	Txn         *transaction.CachedTransaction
	TxnMetadata *CachedTransactionMetadata
	Attachments *CachedAttachment
}

func newEvents() *Events {
	return &Events{
		PayloadAttached:        events.NewEvent(cachedPayloadEvent),
		PayloadSolid:           events.NewEvent(cachedPayloadEvent),
		PayloadLiked:           events.NewEvent(cachedPayloadEvent),
		PayloadConfirmed:       events.NewEvent(cachedPayloadEvent),
		PayloadRejected:        events.NewEvent(cachedPayloadEvent),
		PayloadDisliked:        events.NewEvent(cachedPayloadEvent),
		MissingPayloadReceived: events.NewEvent(cachedPayloadEvent),
		PayloadMissing:         events.NewEvent(payloadIDEvent),
		PayloadInvalid:         events.NewEvent(cachedPayloadErrorEvent),
		TransactionReceived:    events.NewEvent(cachedTransactionAttachmentEvent),
		TransactionInvalid:     events.NewEvent(cachedTransactionErrorEvent),
		TransactionSolid:       events.NewEvent(cachedTransactionEvent),
		TransactionBooked:      events.NewEvent(transactionBookedEvent),
		TransactionPreferred:   events.NewEvent(cachedTransactionEvent),
		TransactionUnpreferred: events.NewEvent(cachedTransactionEvent),
		TransactionLiked:       events.NewEvent(cachedTransactionEvent),
		TransactionDisliked:    events.NewEvent(cachedTransactionEvent),
		TransactionFinalized:   events.NewEvent(cachedTransactionEvent),
		TransactionConfirmed:   events.NewEvent(cachedTransactionEvent),
		TransactionRejected:    events.NewEvent(cachedTransactionEvent),
		Fork:                   events.NewEvent(forkEvent),
		Error:                  events.NewEvent(events.ErrorCaller),
	}
}

func payloadIDEvent(handler interface{}, params ...interface{}) {
	handler.(func(payload.ID))(params[0].(payload.ID))
}

func cachedPayloadEvent(handler interface{}, params ...interface{}) {
	handler.(func(*CachedPayloadEvent))(cachedPayloadRetain(params[0].(*CachedPayloadEvent)))
}

func cachedPayloadErrorEvent(handler interface{}, params ...interface{}) {
	handler.(func(*CachedPayloadEvent, error))(
		cachedPayloadRetain(params[0].(*CachedPayloadEvent)),
		params[1].(error),
	)
}

func transactionBookedEvent(handler interface{}, params ...interface{}) {
	handler.(func(*CachedTxnBookEvent))(cachedTxnBookRetain(params[0].(*CachedTxnBookEvent)))
}

func forkEvent(handler interface{}, params ...interface{}) {
	handler.(func(*ForkEvent))(forkRetain(params[0].(*ForkEvent)))
}

func cachedTransactionEvent(handler interface{}, params ...interface{}) {
	handler.(func(*CachedTxnEvent))(cachedTxnRetain(params[0].(*CachedTxnEvent)))
}

func cachedTransactionErrorEvent(handler interface{}, params ...interface{}) {
	handler.(func(*CachedTxnEvent, error))(
		cachedTxnRetain(params[0].(*CachedTxnEvent)),
		params[1].(error),
	)
}

func cachedTransactionAttachmentEvent(handler interface{}, params ...interface{}) {
	handler.(func(*CachedAttachmentsEvent))(cachedAttachmentsRetain(params[0].(*CachedAttachmentsEvent)))
}

func cachedPayloadRetain(cachedPayload *CachedPayloadEvent) *CachedPayloadEvent {
	return &CachedPayloadEvent{
		Payload:         cachedPayload.Payload.Retain(),
		PayloadMetadata: cachedPayload.PayloadMetadata.Retain(),
	}
}

func cachedTxnRetain(cachedTxn *CachedTxnEvent) *CachedTxnEvent {
	return &CachedTxnEvent{
		Txn:         cachedTxn.Txn.Retain(),
		TxnMetadata: cachedTxn.TxnMetadata.Retain(),
	}
}

func cachedTxnBookRetain(cachedTxn *CachedTxnBookEvent) *CachedTxnBookEvent {
	return &CachedTxnBookEvent{
		Txn:         cachedTxn.Txn.Retain(),
		TxnMetadata: cachedTxn.TxnMetadata.Retain(),
		Pending:     cachedTxn.Pending,
	}
}

func forkRetain(forkEvent *ForkEvent) *ForkEvent {
	return &ForkEvent{
		Txn:         forkEvent.Txn.Retain(),
		TxnMetadata: forkEvent.TxnMetadata.Retain(),
		Branch:      forkEvent.Branch.Retain(),
		OutputIDs:   forkEvent.OutputIDs,
	}
}

func cachedAttachmentsRetain(cachedTxn *CachedAttachmentsEvent) *CachedAttachmentsEvent {
	return &CachedAttachmentsEvent{
		Txn:         cachedTxn.Txn.Retain(),
		TxnMetadata: cachedTxn.TxnMetadata.Retain(),
		Attachments: cachedTxn.Attachments.Retain(),
	}
}
