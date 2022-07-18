package notarization

import (
	"github.com/iotaledger/goshimmer/packages/epoch"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/types/confirmation"
	"github.com/stretchr/testify/mock"

	"github.com/iotaledger/goshimmer/packages/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
)

const (
	testingAcceptanceThreshold = 0.4
)

var (
	// TestBranchConfirmationStateTranslation translates a branch's AW into a confirmation state.
	TestBranchConfirmationStateTranslation acceptance.BranchThresholdTranslation = func(branchID utxo.TransactionID, aw float64) confirmation.State {
		if aw >= testingAcceptanceThreshold {
			return confirmation.Accepted
		}

		return confirmation.Pending
	}

	// TestMessageConfirmationStateTranslation translates a message's AW into a confirmation state.
	TestMessageConfirmationStateTranslation acceptance.MessageThresholdTranslation = func(aw float64) confirmation.State {
		if aw >= testingAcceptanceThreshold {
			return confirmation.Accepted
		}

		return confirmation.Pending
	}
)

// EventMock acts as a container for event mocks.
type EventMock struct {
	mock.Mock
	expectedEvents uint64
	calledEvents   uint64
	test           *testing.T

	attached []struct {
		*event.Event[*EpochCommittableEvent]
		*event.Closure[*EpochCommittableEvent]
	}
}

// NewEventMock creates a new EventMock.
func NewEventMock(t *testing.T, notarizationManager *Manager) *EventMock {
	e := &EventMock{
		test: t,
	}

	// attach all events
	notarizationManager.Events.EpochCommittable.Hook(event.NewClosure(e.EpochCommittable))
	notarizationManager.Events.ManaVectorUpdate.Hook(event.NewClosure(e.ManaVectorUpdate))

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

// EpochCommittable is the mocked EpochCommittable event.
func (e *EventMock) EpochCommittable(event *EpochCommittableEvent) {
	e.Called(event.EI)
	atomic.AddUint64(&e.calledEvents, 1)
}

// ManaVectorUpdate is the mocked ManaVectorUpdate event.
func (e *EventMock) ManaVectorUpdate(event *ManaVectorUpdateEvent) {
	e.Called(event.EI, event.EpochDiffCreated, event.EpochDiffSpent)
	atomic.AddUint64(&e.calledEvents, 1)
}

func epochRetrieverMock() epoch.Index {
	return epoch.IndexFromTime(time.Now())
}
