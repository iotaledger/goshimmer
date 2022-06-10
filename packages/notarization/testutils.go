package notarization

import (
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/iotaledger/hive.go/generics/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// EventMock acts as a container for event mocks.
type EventMock struct {
	mock.Mock
	expectedEvents uint64
	calledEvents   uint64
	test           *testing.T

	// TODO: what is this for, do we need this?
	attached []struct {
		*event.Event[*EpochCommittedEvent]
		*event.Closure[*EpochCommittedEvent]
	}
}

// NewEventMock creates a new EventMock.
func NewEventMock(t *testing.T, notarizationManager *Manager, ecFactory *EpochCommitmentFactory) *EventMock {
	e := &EventMock{
		test: t,
	}

	// attach all events
	notarizationManager.Events.EpochCommitted.Hook(event.NewClosure(e.EpochCommitted))
	ecFactory.Events.NewCommitmentTreesCreated.Hook(event.NewClosure(e.NewCommitmentTreesCreated))

	// assure that all available events are mocked
	numEvents := reflect.ValueOf(notarizationManager.Events).Elem().NumField()
	factoryNumEvents := reflect.ValueOf(ecFactory.Events).Elem().NumField()
	assert.Equalf(t, len(e.attached)+2, numEvents+factoryNumEvents, "not all events in notarizationManager.Events have been attached")

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
	e.On(eventName, arguments...)
	atomic.AddUint64(&e.expectedEvents, 1)
}

// AssertExpectations asserts expectations.
func (e *EventMock) AssertExpectations(t mock.TestingT) bool {
	calledEvents := atomic.LoadUint64(&e.calledEvents)
	expectedEvents := atomic.LoadUint64(&e.expectedEvents)
	if calledEvents != expectedEvents {
		t.Errorf("number of called (%d) events is not equal to number of expected events (%d)", calledEvents, expectedEvents)
		return false
	}

	defer func() {
		e.Calls = make([]mock.Call, 0)
		e.ExpectedCalls = make([]*mock.Call, 0)
		e.expectedEvents = 0
		e.calledEvents = 0
	}()

	return e.Mock.AssertExpectations(t)
}

// EpochCommitted is the mocked BranchWeightChanged function.
func (e *EventMock) EpochCommitted(event *EpochCommittedEvent) {
	e.Called(event.EI)

	atomic.AddUint64(&e.calledEvents, 1)
}

// NewCommitmentTreesCreated is the mocked BranchWeightChanged function.
func (e *EventMock) NewCommitmentTreesCreated(event *CommitmentTreesCreatedEvent) {
	e.Called(event.EI)

	atomic.AddUint64(&e.calledEvents, 1)
}
