package proofs

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
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

	attached []struct {
		*event.Event[*commitment.Commitment]
		*event.Closure[*commitment.Commitment]
	}
}

// NewEventMock creates a new EventMock.
func NewEventMock(t *testing.T, notarizationManager *notarization.Manager) *EventMock {
	e := &EventMock{
		test: t,
	}

	// attach all events
	notarizationManager.Events.EpochCommitted.Hook(event.NewClosure(e.EpochCommittable))
	// notarizationManager.Events.ConsensusWeightsUpdated.Hook(event.NewClosure(e.ManaVectorUpdate))

	return e
}

// DetachAll detaches all event handlers.
func (e *EventMock) DetachAll() {
	for _, a := range e.attached {
		a.Event.Detach(a.Closure)
	}
}

// Expect is a proxy for Mock.On() but keeping track of num of calls.
func (e *EventMock) Expect(eventName string, arguments ...interface{}) {
	event.Loop.WaitUntilAllTasksProcessed()
	e.On(eventName, arguments...)
	atomic.AddUint64(&e.expectedEvents, 1)
}

// AssertExpectations asserts expectations.
func (e *EventMock) AssertExpectations(t mock.TestingT) bool {
	var calledEvents, expectedEvents uint64
	event.Loop.WaitUntilAllTasksProcessed()

	assert.Eventuallyf(t, func() bool {
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

// EpochCommittable is the mocked EpochCommittable event.
func (e *EventMock) EpochCommittable(commitment *commitment.Commitment) {
	e.Called(commitment.Index())
	atomic.AddUint64(&e.calledEvents, 1)
}

/*
// ManaVectorUpdate is the mocked ManaVectorUpdate event.
func (e *EventMock) ManaVectorUpdate(event *mana.ManaVectorUpdateEvent) {
	e.Called(event.EI)
	atomic.AddUint64(&e.calledEvents, 1)
}
*/
