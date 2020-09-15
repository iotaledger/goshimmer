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

func (e *eventTangle) PayloadAttached(cachedPayloadEvent *CachedPayloadEvent) {
	defer cachedPayloadEvent.Payload.Release()
	defer cachedPayloadEvent.PayloadMetadata.Release()
	e.Called(&CachedPayloadEvent{
		Payload:         cachedPayloadEvent.Payload,
		PayloadMetadata: cachedPayloadEvent.PayloadMetadata})
}

func (e *eventTangle) PayloadSolid(cachedPayloadEvent *CachedPayloadEvent) {
	defer cachedPayloadEvent.Payload.Release()
	defer cachedPayloadEvent.PayloadMetadata.Release()
	e.Called(&CachedPayloadEvent{
		Payload:         cachedPayloadEvent.Payload,
		PayloadMetadata: cachedPayloadEvent.PayloadMetadata})
}

func (e *eventTangle) PayloadLiked(cachedPayloadEvent *CachedPayloadEvent) {
	defer cachedPayloadEvent.Payload.Release()
	defer cachedPayloadEvent.PayloadMetadata.Release()
	e.Called(&CachedPayloadEvent{
		Payload:         cachedPayloadEvent.Payload,
		PayloadMetadata: cachedPayloadEvent.PayloadMetadata})
}

func (e *eventTangle) PayloadConfirmed(cachedPayloadEvent *CachedPayloadEvent) {
	defer cachedPayloadEvent.Payload.Release()
	defer cachedPayloadEvent.PayloadMetadata.Release()
	e.Called(&CachedPayloadEvent{
		Payload:         cachedPayloadEvent.Payload,
		PayloadMetadata: cachedPayloadEvent.PayloadMetadata})
}

func (e *eventTangle) PayloadRejected(cachedPayloadEvent *CachedPayloadEvent) {
	defer cachedPayloadEvent.Payload.Release()
	defer cachedPayloadEvent.PayloadMetadata.Release()
	e.Called(&CachedPayloadEvent{
		Payload:         cachedPayloadEvent.Payload,
		PayloadMetadata: cachedPayloadEvent.PayloadMetadata})
}

func (e *eventTangle) PayloadDisliked(cachedPayloadEvent *CachedPayloadEvent) {
	defer cachedPayloadEvent.Payload.Release()
	defer cachedPayloadEvent.PayloadMetadata.Release()
	e.Called(&CachedPayloadEvent{
		Payload:         cachedPayloadEvent.Payload,
		PayloadMetadata: cachedPayloadEvent.PayloadMetadata})
}

func (e *eventTangle) MissingPayloadReceived(cachedPayloadEvent *CachedPayloadEvent) {
	defer cachedPayloadEvent.Payload.Release()
	defer cachedPayloadEvent.PayloadMetadata.Release()
	e.Called(&CachedPayloadEvent{
		Payload:         cachedPayloadEvent.Payload,
		PayloadMetadata: cachedPayloadEvent.PayloadMetadata})
}

func (e *eventTangle) PayloadMissing(id payload.ID) {
	e.Called(id)
}

func (e *eventTangle) PayloadInvalid(cachedPayloadEvent *CachedPayloadEvent, err error) {
	defer cachedPayloadEvent.Payload.Release()
	defer cachedPayloadEvent.PayloadMetadata.Release()
	e.Called(&CachedPayloadEvent{
		Payload:         cachedPayloadEvent.Payload,
		PayloadMetadata: cachedPayloadEvent.PayloadMetadata}, err)
}

func (e *eventTangle) TransactionReceived(cachedAttachmentsEvent *CachedAttachmentsEvent) {
	defer cachedAttachmentsEvent.Transaction.Release()
	defer cachedAttachmentsEvent.TransactionMetadata.Release()
	defer cachedAttachmentsEvent.Attachments.Release()
	e.Called(&CachedAttachmentsEvent{
		Transaction:         cachedAttachmentsEvent.Transaction,
		TransactionMetadata: cachedAttachmentsEvent.TransactionMetadata,
		Attachments:         cachedAttachmentsEvent.Attachments})
}

func (e *eventTangle) TransactionInvalid(cachedTransactionEvent *CachedTransactionEvent, err error) {
	defer cachedTransactionEvent.Transaction.Release()
	defer cachedTransactionEvent.TransactionMetadata.Release()
	e.Called(&CachedTransactionEvent{
		Transaction:         cachedTransactionEvent.Transaction,
		TransactionMetadata: cachedTransactionEvent.TransactionMetadata}, err)
}

func (e *eventTangle) TransactionSolid(cachedTransactionEvent *CachedTransactionEvent) {
	defer cachedTransactionEvent.Transaction.Release()
	defer cachedTransactionEvent.TransactionMetadata.Release()
	e.Called(&CachedTransactionEvent{
		Transaction:         cachedTransactionEvent.Transaction,
		TransactionMetadata: cachedTransactionEvent.TransactionMetadata})
}

func (e *eventTangle) TransactionBooked(cachedTransactionBookEvent *CachedTransactionBookEvent) {
	defer cachedTransactionBookEvent.Transaction.Release()
	defer cachedTransactionBookEvent.TransactionMetadata.Release()
	e.Called(&CachedTransactionBookEvent{
		Transaction:         cachedTransactionBookEvent.Transaction,
		TransactionMetadata: cachedTransactionBookEvent.TransactionMetadata,
		Pending:             cachedTransactionBookEvent.Pending})
}

func (e *eventTangle) TransactionPreferred(cachedTransactionEvent *CachedTransactionEvent) {
	defer cachedTransactionEvent.Transaction.Release()
	defer cachedTransactionEvent.TransactionMetadata.Release()
	e.Called(&CachedTransactionEvent{
		Transaction:         cachedTransactionEvent.Transaction,
		TransactionMetadata: cachedTransactionEvent.TransactionMetadata})
}

func (e *eventTangle) TransactionUnpreferred(cachedTransactionEvent *CachedTransactionEvent) {
	defer cachedTransactionEvent.Transaction.Release()
	defer cachedTransactionEvent.TransactionMetadata.Release()
	e.Called(&CachedTransactionEvent{
		Transaction:         cachedTransactionEvent.Transaction,
		TransactionMetadata: cachedTransactionEvent.TransactionMetadata})
}

func (e *eventTangle) TransactionLiked(cachedTransactionEvent *CachedTransactionEvent) {
	defer cachedTransactionEvent.Transaction.Release()
	defer cachedTransactionEvent.TransactionMetadata.Release()
	e.Called(&CachedTransactionEvent{
		Transaction:         cachedTransactionEvent.Transaction,
		TransactionMetadata: cachedTransactionEvent.TransactionMetadata})
}

func (e *eventTangle) TransactionDisliked(cachedTransactionEvent *CachedTransactionEvent) {
	defer cachedTransactionEvent.Transaction.Release()
	defer cachedTransactionEvent.TransactionMetadata.Release()
	e.Called(&CachedTransactionEvent{
		Transaction:         cachedTransactionEvent.Transaction,
		TransactionMetadata: cachedTransactionEvent.TransactionMetadata})
}

func (e *eventTangle) TransactionFinalized(cachedTransactionEvent *CachedTransactionEvent) {
	defer cachedTransactionEvent.Transaction.Release()
	defer cachedTransactionEvent.TransactionMetadata.Release()
	e.Called(&CachedTransactionEvent{
		Transaction:         cachedTransactionEvent.Transaction,
		TransactionMetadata: cachedTransactionEvent.TransactionMetadata})
}

func (e *eventTangle) TransactionConfirmed(cachedTransactionEvent *CachedTransactionEvent) {
	defer cachedTransactionEvent.Transaction.Release()
	defer cachedTransactionEvent.TransactionMetadata.Release()
	e.Called(&CachedTransactionEvent{
		Transaction:         cachedTransactionEvent.Transaction,
		TransactionMetadata: cachedTransactionEvent.TransactionMetadata})
}

func (e *eventTangle) TransactionRejected(cachedTransactionEvent *CachedTransactionEvent) {
	defer cachedTransactionEvent.Transaction.Release()
	defer cachedTransactionEvent.TransactionMetadata.Release()
	e.Called(&CachedTransactionEvent{
		Transaction:         cachedTransactionEvent.Transaction,
		TransactionMetadata: cachedTransactionEvent.TransactionMetadata})
}

func (e *eventTangle) Fork(forkEvent *ForkEvent) {
	defer forkEvent.Transaction.Release()
	defer forkEvent.TransactionMetadata.Release()
	defer forkEvent.Branch.Release()
	e.Called(&ForkEvent{
		Transaction:         forkEvent.Transaction,
		TransactionMetadata: forkEvent.TransactionMetadata,
		Branch:              forkEvent.Branch,
		InputIDs:            forkEvent.InputIDs})
}

// TODO: Error is never tested
func (e *eventTangle) Error(err error) {
	e.Called(err)
}
