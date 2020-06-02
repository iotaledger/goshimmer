package tangle

import (
	"github.com/iotaledger/hive.go/events"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/branchmanager"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
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
	PayloadUnsolidifiable  *events.Event

	// TransactionReceived gets triggered whenever a transaction was received for the first time (not solid yet).
	TransactionReceived *events.Event

	// TransactionBooked gets triggered whenever a transactions becomes solid and gets booked into a particular branch.
	TransactionBooked *events.Event

	TransactionPreferred *events.Event

	TransactionUnpreferred *events.Event

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
		PayloadUnsolidifiable:  events.NewEvent(payloadIDEvent),
		TransactionReceived:    events.NewEvent(cachedTransactionAttachmentEvent),
		TransactionBooked:      events.NewEvent(transactionBookedEvent),
		TransactionPreferred:   events.NewEvent(cachedTransactionEvent),
		TransactionUnpreferred: events.NewEvent(cachedTransactionEvent),
		TransactionFinalized:   events.NewEvent(cachedTransactionEvent),
		Fork:                   events.NewEvent(forkEvent),
		Error:                  events.NewEvent(events.ErrorCaller),
	}
}

func payloadIDEvent(handler interface{}, params ...interface{}) {
	handler.(func(payload.ID))(params[0].(payload.ID))
}

func cachedPayloadEvent(handler interface{}, params ...interface{}) {
	handler.(func(*payload.CachedPayload, *CachedPayloadMetadata))(
		params[0].(*payload.CachedPayload).Retain(),
		params[1].(*CachedPayloadMetadata).Retain(),
	)
}

func transactionBookedEvent(handler interface{}, params ...interface{}) {
	handler.(func(*transaction.CachedTransaction, *CachedTransactionMetadata, bool))(
		params[0].(*transaction.CachedTransaction).Retain(),
		params[1].(*CachedTransactionMetadata).Retain(),
		params[2].(bool),
	)
}

func forkEvent(handler interface{}, params ...interface{}) {
	handler.(func(*transaction.CachedTransaction, *CachedTransactionMetadata, *branchmanager.CachedBranch, []transaction.OutputID))(
		params[0].(*transaction.CachedTransaction).Retain(),
		params[1].(*CachedTransactionMetadata).Retain(),
		params[2].(*branchmanager.CachedBranch).Retain(),
		params[3].([]transaction.OutputID),
	)
}

func cachedTransactionEvent(handler interface{}, params ...interface{}) {
	handler.(func(*transaction.CachedTransaction, *CachedTransactionMetadata))(
		params[0].(*transaction.CachedTransaction).Retain(),
		params[1].(*CachedTransactionMetadata).Retain(),
	)
}

func cachedTransactionAttachmentEvent(handler interface{}, params ...interface{}) {
	handler.(func(*transaction.CachedTransaction, *CachedTransactionMetadata, *CachedAttachment))(
		params[0].(*transaction.CachedTransaction).Retain(),
		params[1].(*CachedTransactionMetadata).Retain(),
		params[2].(*CachedAttachment).Retain(),
	)
}
