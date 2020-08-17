package tangle

import (
	"reflect"
	"testing"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/hive.go/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// eventTangle is a wrapper around Tangle used to test the triggered events.
type eventTangle struct {
	mock.Mock
	*Tangle

	attached []struct {
		*events.Event
		*events.Closure
	}
}

func newEventTangle(t *testing.T, tangle *Tangle) *eventTangle {
	e := &eventTangle{Tangle: tangle}
	e.Test(t)

	// attach all events
	e.attach(tangle.Events.PayloadAttached, e.PayloadAttached)
	e.attach(tangle.Events.PayloadSolid, e.PayloadSolid)
	e.attach(tangle.Events.PayloadLiked, e.PayloadLiked)
	e.attach(tangle.Events.PayloadConfirmed, e.PayloadConfirmed)
	e.attach(tangle.Events.PayloadRejected, e.PayloadRejected)
	e.attach(tangle.Events.PayloadDisliked, e.PayloadDisliked)
	e.attach(tangle.Events.MissingPayloadReceived, e.MissingPayloadReceived)
	e.attach(tangle.Events.PayloadMissing, e.PayloadMissing)
	e.attach(tangle.Events.PayloadInvalid, e.PayloadInvalid)
	e.attach(tangle.Events.TransactionReceived, e.TransactionReceived)
	e.attach(tangle.Events.TransactionInvalid, e.TransactionInvalid)
	e.attach(tangle.Events.TransactionSolid, e.TransactionSolid)
	e.attach(tangle.Events.TransactionBooked, e.TransactionBooked)
	e.attach(tangle.Events.TransactionPreferred, e.TransactionPreferred)
	e.attach(tangle.Events.TransactionUnpreferred, e.TransactionUnpreferred)
	e.attach(tangle.Events.TransactionLiked, e.TransactionLiked)
	e.attach(tangle.Events.TransactionDisliked, e.TransactionDisliked)
	e.attach(tangle.Events.TransactionFinalized, e.TransactionFinalized)
	e.attach(tangle.Events.TransactionConfirmed, e.TransactionConfirmed)
	e.attach(tangle.Events.TransactionRejected, e.TransactionRejected)
	e.attach(tangle.Events.Fork, e.Fork)
	e.attach(tangle.Events.Error, e.Error)

	// assure that all available events are mocked
	numEvents := reflect.ValueOf(tangle.Events).Elem().NumField()
	assert.Equalf(t, len(e.attached), numEvents, "not all events in Tangle.Events have been attached")

	return e
}

// DetachAll detaches all attached event mocks.
func (e *eventTangle) DetachAll() {
	for _, a := range e.attached {
		a.Event.Detach(a.Closure)
	}
}

func (e *eventTangle) attach(event *events.Event, f interface{}) {
	closure := events.NewClosure(f)
	event.Attach(closure)
	e.attached = append(e.attached, struct {
		*events.Event
		*events.Closure
	}{event, closure})
}

// Expect starts a description of an expectation of the specified event being triggered.
func (e *eventTangle) Expect(eventName string, arguments ...interface{}) {
	e.On(eventName, arguments...).Once()
}

func (e *eventTangle) PayloadAttached(cachedPayload *CachedPayloadEvent) {
	defer cachedPayload.Payload.Release()
	defer cachedPayload.PayloadMetadata.Release()
	e.Called(&CachedPayloadEvent{
		Payload:         cachedPayload.Payload,
		PayloadMetadata: cachedPayload.PayloadMetadata})
}

func (e *eventTangle) PayloadSolid(cachedPayload *CachedPayloadEvent) {
	defer cachedPayload.Payload.Release()
	defer cachedPayload.PayloadMetadata.Release()
	e.Called(&CachedPayloadEvent{
		Payload:         cachedPayload.Payload,
		PayloadMetadata: cachedPayload.PayloadMetadata})
}

func (e *eventTangle) PayloadLiked(cachedPayload *CachedPayloadEvent) {
	defer cachedPayload.Payload.Release()
	defer cachedPayload.PayloadMetadata.Release()
	e.Called(&CachedPayloadEvent{
		Payload:         cachedPayload.Payload,
		PayloadMetadata: cachedPayload.PayloadMetadata})
}

func (e *eventTangle) PayloadConfirmed(cachedPayload *CachedPayloadEvent) {
	defer cachedPayload.Payload.Release()
	defer cachedPayload.PayloadMetadata.Release()
	e.Called(&CachedPayloadEvent{
		Payload:         cachedPayload.Payload,
		PayloadMetadata: cachedPayload.PayloadMetadata})
}

func (e *eventTangle) PayloadRejected(cachedPayload *CachedPayloadEvent) {
	defer cachedPayload.Payload.Release()
	defer cachedPayload.PayloadMetadata.Release()
	e.Called(&CachedPayloadEvent{
		Payload:         cachedPayload.Payload,
		PayloadMetadata: cachedPayload.PayloadMetadata})
}

func (e *eventTangle) PayloadDisliked(cachedPayload *CachedPayloadEvent) {
	defer cachedPayload.Payload.Release()
	defer cachedPayload.PayloadMetadata.Release()
	e.Called(&CachedPayloadEvent{
		Payload:         cachedPayload.Payload,
		PayloadMetadata: cachedPayload.PayloadMetadata})
}

func (e *eventTangle) MissingPayloadReceived(cachedPayload *CachedPayloadEvent) {
	defer cachedPayload.Payload.Release()
	defer cachedPayload.PayloadMetadata.Release()
	e.Called(&CachedPayloadEvent{
		Payload:         cachedPayload.Payload,
		PayloadMetadata: cachedPayload.PayloadMetadata})
}

func (e *eventTangle) PayloadMissing(id payload.ID) {
	e.Called(id)
}

func (e *eventTangle) PayloadInvalid(cachedPayload *CachedPayloadEvent, err error) {
	defer cachedPayload.Payload.Release()
	defer cachedPayload.PayloadMetadata.Release()
	e.Called(&CachedPayloadEvent{
		Payload:         cachedPayload.Payload,
		PayloadMetadata: cachedPayload.PayloadMetadata}, err)
}

func (e *eventTangle) TransactionReceived(cachedAttachment *CachedAttachmentsEvent) {
	defer cachedAttachment.Txn.Release()
	defer cachedAttachment.TxnMetadata.Release()
	defer cachedAttachment.Attachments.Release()
	e.Called(&CachedAttachmentsEvent{
		Txn:         cachedAttachment.Txn,
		TxnMetadata: cachedAttachment.TxnMetadata,
		Attachments: cachedAttachment.Attachments})
}

func (e *eventTangle) TransactionInvalid(cachedTxn *CachedTxnEvent, err error) {
	defer cachedTxn.Txn.Release()
	defer cachedTxn.TxnMetadata.Release()
	e.Called(&CachedTxnEvent{
		Txn:         cachedTxn.Txn,
		TxnMetadata: cachedTxn.TxnMetadata}, err)
}

func (e *eventTangle) TransactionSolid(cachedTxn *CachedTxnEvent) {
	defer cachedTxn.Txn.Release()
	defer cachedTxn.TxnMetadata.Release()
	e.Called(&CachedTxnEvent{
		Txn:         cachedTxn.Txn,
		TxnMetadata: cachedTxn.TxnMetadata})
}

func (e *eventTangle) TransactionBooked(txnBooked *CachedTxnBookEvent) {
	defer txnBooked.Txn.Release()
	defer txnBooked.TxnMetadata.Release()
	e.Called(&CachedTxnBookEvent{
		Txn:         txnBooked.Txn,
		TxnMetadata: txnBooked.TxnMetadata,
		Pending:     txnBooked.Pending})
}

func (e *eventTangle) TransactionPreferred(cachedTxn *CachedTxnEvent) {
	defer cachedTxn.Txn.Release()
	defer cachedTxn.TxnMetadata.Release()
	e.Called(&CachedTxnEvent{
		Txn:         cachedTxn.Txn,
		TxnMetadata: cachedTxn.TxnMetadata})
}

func (e *eventTangle) TransactionUnpreferred(cachedTxn *CachedTxnEvent) {
	defer cachedTxn.Txn.Release()
	defer cachedTxn.TxnMetadata.Release()
	e.Called(&CachedTxnEvent{
		Txn:         cachedTxn.Txn,
		TxnMetadata: cachedTxn.TxnMetadata})
}

func (e *eventTangle) TransactionLiked(cachedTxn *CachedTxnEvent) {
	defer cachedTxn.Txn.Release()
	defer cachedTxn.TxnMetadata.Release()
	e.Called(&CachedTxnEvent{
		Txn:         cachedTxn.Txn,
		TxnMetadata: cachedTxn.TxnMetadata})
}

func (e *eventTangle) TransactionDisliked(cachedTxn *CachedTxnEvent) {
	defer cachedTxn.Txn.Release()
	defer cachedTxn.TxnMetadata.Release()
	e.Called(&CachedTxnEvent{
		Txn:         cachedTxn.Txn,
		TxnMetadata: cachedTxn.TxnMetadata})
}

func (e *eventTangle) TransactionFinalized(cachedTxn *CachedTxnEvent) {
	defer cachedTxn.Txn.Release()
	defer cachedTxn.TxnMetadata.Release()
	e.Called(&CachedTxnEvent{
		Txn:         cachedTxn.Txn,
		TxnMetadata: cachedTxn.TxnMetadata})
}

func (e *eventTangle) TransactionConfirmed(cachedTxn *CachedTxnEvent) {
	defer cachedTxn.Txn.Release()
	defer cachedTxn.TxnMetadata.Release()
	e.Called(&CachedTxnEvent{
		Txn:         cachedTxn.Txn,
		TxnMetadata: cachedTxn.TxnMetadata})
}

func (e *eventTangle) TransactionRejected(cachedTxn *CachedTxnEvent) {
	defer cachedTxn.Txn.Release()
	defer cachedTxn.TxnMetadata.Release()
	e.Called(&CachedTxnEvent{
		Txn:         cachedTxn.Txn,
		TxnMetadata: cachedTxn.TxnMetadata})
}

func (e *eventTangle) Fork(forkEvent *ForkEvent) {
	defer forkEvent.Txn.Release()
	defer forkEvent.TxnMetadata.Release()
	defer forkEvent.Branch.Release()
	e.Called(&ForkEvent{
		Txn:         forkEvent.Txn,
		TxnMetadata: forkEvent.TxnMetadata,
		Branch:      forkEvent.Branch,
		OutputIDs:   forkEvent.OutputIDs})
}

// TODO: Error is never tested
func (e *eventTangle) Error(err error) {
	e.Called(err)
}
