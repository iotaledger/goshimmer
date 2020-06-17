package tangle

import (
	"reflect"
	"testing"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/branchmanager"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
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

func (e *eventTangle) PayloadAttached(payload *payload.CachedPayload, payloadMetadata *CachedPayloadMetadata) {
	defer payload.Release()
	defer payloadMetadata.Release()
	e.Called(payload.Unwrap(), payloadMetadata.Unwrap())
}

func (e *eventTangle) PayloadSolid(payload *payload.CachedPayload, payloadMetadata *CachedPayloadMetadata) {
	defer payload.Release()
	defer payloadMetadata.Release()
	e.Called(payload.Unwrap(), payloadMetadata.Unwrap())
}

func (e *eventTangle) PayloadLiked(payload *payload.CachedPayload, payloadMetadata *CachedPayloadMetadata) {
	defer payload.Release()
	defer payloadMetadata.Release()
	e.Called(payload.Unwrap(), payloadMetadata.Unwrap())
}

func (e *eventTangle) PayloadConfirmed(payload *payload.CachedPayload, payloadMetadata *CachedPayloadMetadata) {
	defer payload.Release()
	defer payloadMetadata.Release()
	e.Called(payload.Unwrap(), payloadMetadata.Unwrap())
}

func (e *eventTangle) PayloadRejected(payload *payload.CachedPayload, payloadMetadata *CachedPayloadMetadata) {
	defer payload.Release()
	defer payloadMetadata.Release()
	e.Called(payload.Unwrap(), payloadMetadata.Unwrap())
}

func (e *eventTangle) PayloadDisliked(payload *payload.CachedPayload, payloadMetadata *CachedPayloadMetadata) {
	defer payload.Release()
	defer payloadMetadata.Release()
	e.Called(payload.Unwrap(), payloadMetadata.Unwrap())
}

func (e *eventTangle) MissingPayloadReceived(payload *payload.CachedPayload, payloadMetadata *CachedPayloadMetadata) {
	defer payload.Release()
	defer payloadMetadata.Release()
	e.Called(payload.Unwrap(), payloadMetadata.Unwrap())
}

func (e *eventTangle) PayloadMissing(id payload.ID) {
	e.Called(id)
}

func (e *eventTangle) PayloadInvalid(payload *payload.CachedPayload, payloadMetadata *CachedPayloadMetadata, err error) {
	defer payload.Release()
	defer payloadMetadata.Release()
	e.Called(payload.Unwrap(), payloadMetadata.Unwrap(), err)
}

func (e *eventTangle) TransactionReceived(transaction *transaction.CachedTransaction, transactionMetadata *CachedTransactionMetadata, attachment *CachedAttachment) {
	defer transaction.Release()
	defer transactionMetadata.Release()
	defer attachment.Release()
	e.Called(transaction.Unwrap(), transactionMetadata.Unwrap(), attachment.Unwrap())
}

func (e *eventTangle) TransactionInvalid(transaction *transaction.CachedTransaction, transactionMetadata *CachedTransactionMetadata, err error) {
	defer transaction.Release()
	defer transactionMetadata.Release()
	e.Called(transaction.Unwrap(), transactionMetadata.Unwrap(), err)
}

func (e *eventTangle) TransactionSolid(transaction *transaction.CachedTransaction, transactionMetadata *CachedTransactionMetadata) {
	defer transaction.Release()
	defer transactionMetadata.Release()
	e.Called(transaction.Unwrap(), transactionMetadata.Unwrap())
}

func (e *eventTangle) TransactionBooked(transaction *transaction.CachedTransaction, transactionMetadata *CachedTransactionMetadata, decisionPending bool) {
	defer transaction.Release()
	defer transactionMetadata.Release()
	e.Called(transaction.Unwrap(), transactionMetadata.Unwrap(), decisionPending)
}

func (e *eventTangle) TransactionPreferred(transaction *transaction.CachedTransaction, transactionMetadata *CachedTransactionMetadata) {
	defer transaction.Release()
	defer transactionMetadata.Release()
	e.Called(transaction.Unwrap(), transactionMetadata.Unwrap())
}

func (e *eventTangle) TransactionUnpreferred(transaction *transaction.CachedTransaction, transactionMetadata *CachedTransactionMetadata) {
	defer transaction.Release()
	defer transactionMetadata.Release()
	e.Called(transaction.Unwrap(), transactionMetadata.Unwrap())
}

func (e *eventTangle) TransactionLiked(transaction *transaction.CachedTransaction, transactionMetadata *CachedTransactionMetadata) {
	defer transaction.Release()
	defer transactionMetadata.Release()
	e.Called(transaction.Unwrap(), transactionMetadata.Unwrap())
}

func (e *eventTangle) TransactionDisliked(transaction *transaction.CachedTransaction, transactionMetadata *CachedTransactionMetadata) {
	defer transaction.Release()
	defer transactionMetadata.Release()
	e.Called(transaction.Unwrap(), transactionMetadata.Unwrap())
}

func (e *eventTangle) TransactionFinalized(transaction *transaction.CachedTransaction, transactionMetadata *CachedTransactionMetadata) {
	defer transaction.Release()
	defer transactionMetadata.Release()
	e.Called(transaction.Unwrap(), transactionMetadata.Unwrap())
}

func (e *eventTangle) TransactionConfirmed(transaction *transaction.CachedTransaction, transactionMetadata *CachedTransactionMetadata) {
	defer transaction.Release()
	defer transactionMetadata.Release()
	e.Called(transaction.Unwrap(), transactionMetadata.Unwrap())
}

func (e *eventTangle) TransactionRejected(transaction *transaction.CachedTransaction, transactionMetadata *CachedTransactionMetadata) {
	defer transaction.Release()
	defer transactionMetadata.Release()
	e.Called(transaction.Unwrap(), transactionMetadata.Unwrap())
}

func (e *eventTangle) Fork(transaction *transaction.CachedTransaction, transactionMetadata *CachedTransactionMetadata, branch *branchmanager.CachedBranch, outputIDs []transaction.OutputID) {
	defer transaction.Release()
	defer transactionMetadata.Release()
	defer branch.Release()
	e.Called(transaction.Unwrap(), transactionMetadata.Unwrap(), branch.Unwrap(), outputIDs)
}

// TODO: Error is never tested
func (e *eventTangle) Error(err error) {
	e.Called(err)
}
