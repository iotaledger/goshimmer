package proofs

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
)

const (
	testingAcceptanceThreshold = 0.4
)

// EventMock acts as a container for event mocks.
type EventMock struct {
	mock.Mock
	expectedEvents uint64
	calledEvents   uint64
	test           *testing.T

	attached []func()
}

// NewEventMock creates a new EventMock.
func NewEventMock(t *testing.T, notarization notarization.Notarization) *EventMock {
	e := &EventMock{
		test:     t,
		attached: make([]func(), 0),
	}

	// attach all events
	e.attached = append(e.attached, notarization.Events().SlotCommitted.Hook(e.SlotCommittable).Unhook)

	return e
}

// DetachAll detaches all event handlers.
func (e *EventMock) DetachAll() {
	for _, a := range e.attached {
		a()
	}
}

// Expect is a proxy for Mock.On() but keeping track of num of calls.
func (e *EventMock) Expect(eventName string, arguments ...interface{}) {
	e.On(eventName, arguments...)
	atomic.AddUint64(&e.expectedEvents, 1)
}

// AssertExpectations asserts expectations.
func (e *EventMock) AssertExpectations(t mock.TestingT) bool {
	var calledEvents, expectedEvents uint64

	require.Eventuallyf(t, func() bool {
		calledEvents = atomic.LoadUint64(&e.calledEvents)
		expectedEvents = atomic.LoadUint64(&e.expectedEvents)
		return calledEvents == expectedEvents
	}, 5*time.Second, 1*time.Millisecond, "number of called (%d) events is not equal to number of expected events (%d)", calledEvents, expectedEvents)

	defer func() {
		e.Calls = make([]mock.Call, 0)
		e.ExpectedCalls = make([]*mock.Call, 0)
		e.expectedEvents = 0
		e.calledEvents = 0
	}()

	return e.Mock.AssertExpectations(t)
}

// SlotCommittable is the mocked SlotCommittable event.
func (e *EventMock) SlotCommittable(details *notarization.SlotCommittedDetails) {
	e.Called(details.Commitment.Index())
	atomic.AddUint64(&e.calledEvents, 1)
}

/*
// ManaVectorUpdate is the mocked ManaVectorUpdate event.
func (e *EventMock) ManaVectorUpdate(event *mana.ManaVectorUpdateEvent) {
	e.Called(event.SlotIndex)
	atomic.AddUint64(&e.calledEvents, 1)
}
*/
