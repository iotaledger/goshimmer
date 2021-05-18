package tangle

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/markers"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
)

const (
	lastConfirmedKey = "LastConfirmedMessage"
)

// region TimeManager //////////////////////////////////////////////////////////////////////////////////////////////////

// TimeManager is a Tangle component that keeps track of the TangleTime. The TangleTime can be seen as a clock for the
// entire network as it tracks the time of the last confirmed message. Comparing the issuing time of the last confirmed
// message to the node's current wall clock time then yields a reasonable assessment of how much in sync the node is.
type TimeManager struct {
	Events *TimeManagerEvents

	tangle      *Tangle
	startSynced bool

	lastConfirmedMutex   sync.RWMutex
	lastConfirmedMessage LastConfirmedMessage
	lastSynced           bool
}

// NewTimeManager is the constructor for TimeManager.
func NewTimeManager(tangle *Tangle) *TimeManager {
	t := &TimeManager{
		Events: &TimeManagerEvents{
			SyncChanged: events.NewEvent(SyncChangedCaller),
		},
		tangle:      tangle,
		startSynced: tangle.Options.StartSynced,
	}

	// initialize with Genesis
	t.lastConfirmedMessage = LastConfirmedMessage{
		MessageID: EmptyMessageID,
		Time:      time.Unix(DefaultGenesisTime, 0),
	}

	marshaledLastConfirmedMessage, err := tangle.Options.Store.Get(kvstore.Key(lastConfirmedKey))
	if err != nil && !errors.Is(err, kvstore.ErrKeyNotFound) {
		panic(err)
	}
	// load from storage if key was found
	if marshaledLastConfirmedMessage != nil {
		if t.lastConfirmedMessage, _, err = lastConfirmedMessageFromBytes(marshaledLastConfirmedMessage); err != nil {
			panic(err)
		}
	}

	// initialize the synced status
	t.lastSynced = t.synced()

	return t
}

// Setup sets up the behavior of the component by making it attach to the relevant events of other components.
func (t *TimeManager) Setup() {
	t.tangle.ApprovalWeightManager.Events.MarkerConfirmation.Attach(events.NewClosure(t.updateTime))
}

// Shutdown shuts down the TimeManager and persists its state.
func (t *TimeManager) Shutdown() {
	t.lastConfirmedMutex.RLock()
	defer t.lastConfirmedMutex.RUnlock()

	if err := t.tangle.Options.Store.Set(kvstore.Key(lastConfirmedKey), t.lastConfirmedMessage.Bytes()); err != nil {
		t.tangle.Events.Error.Trigger(errors.Errorf("failed to persists LastConfirmedMessage (%v): %w", err, cerrors.ErrFatal))
		return
	}
}

// LastConfirmedMessage returns the last confirmed message.
func (t *TimeManager) LastConfirmedMessage() LastConfirmedMessage {
	t.lastConfirmedMutex.RLock()
	defer t.lastConfirmedMutex.RUnlock()

	return t.lastConfirmedMessage
}

// Time returns the TangleTime, i.e., the issuing time of the last confirmed message.
func (t *TimeManager) Time() time.Time {
	t.lastConfirmedMutex.RLock()
	defer t.lastConfirmedMutex.RUnlock()

	return t.lastConfirmedMessage.Time
}

// Synced returns whether the node is in sync based on the difference between TangleTime and current wall time which can
// be configured via SyncTimeWindow.
func (t *TimeManager) Synced() bool {
	t.lastConfirmedMutex.RLock()
	defer t.lastConfirmedMutex.RUnlock()

	return t.lastSynced
}

func (t *TimeManager) synced() bool {
	if t.startSynced && t.lastConfirmedMessage.Time.Unix() == DefaultGenesisTime {
		return true
	}

	return clock.Since(t.lastConfirmedMessage.Time) < t.tangle.Options.SyncTimeWindow
}

// updateTime updates the last confirmed message.
func (t *TimeManager) updateTime(marker markers.Marker, newLevel int, transition events.ThresholdEventTransition) {
	if transition != events.ThresholdLevelIncreased {
		return
	}
	// get message ID of marker
	messageID := t.tangle.Booker.MarkersManager.MessageID(&marker)

	t.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		t.lastConfirmedMutex.Lock()
		defer t.lastConfirmedMutex.Unlock()

		if t.lastConfirmedMessage.Time.Before(message.IssuingTime()) {
			t.lastConfirmedMessage = LastConfirmedMessage{
				MessageID: messageID,
				Time:      message.IssuingTime(),
			}

			if newSynced := t.synced(); t.lastSynced != newSynced {
				t.lastSynced = newSynced
				// trigger the event inside the lock to assure that the status is still correct
				t.Events.SyncChanged.Trigger(&SyncChangedEvent{Synced: newSynced})
			}
		}
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region LastConfirmedMessage /////////////////////////////////////////////////////////////////////////////////////////

// LastConfirmedMessage is a wrapper type for the last confirmed message, consisting of MessageID and time.
type LastConfirmedMessage struct {
	MessageID MessageID
	Time      time.Time
}

// lastConfirmedMessageFromBytes unmarshals a LastConfirmedMessage object from a sequence of bytes.
func lastConfirmedMessageFromBytes(bytes []byte) (lcm LastConfirmedMessage, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if lcm, err = lastConfirmedMessageFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse LastConfirmedMessage from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// lastConfirmedMessageFromMarshalUtil unmarshals a LastConfirmedMessage object using a MarshalUtil (for easier unmarshaling).
func lastConfirmedMessageFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (lcm LastConfirmedMessage, err error) {
	lcm = LastConfirmedMessage{}
	if lcm.MessageID, err = MessageIDFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse MessageID from MarshalUtil: %w", err)
		return
	}

	if lcm.Time, err = marshalUtil.ReadTime(); err != nil {
		err = errors.Errorf("failed to parse time (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// Bytes returns a marshaled version of the LastConfirmedMessage.
func (l LastConfirmedMessage) Bytes() (marshaledLastConfirmedMessage []byte) {
	return marshalutil.New(MessageIDLength + marshalutil.TimeSize).
		Write(l.MessageID).
		WriteTime(l.Time).
		Bytes()
}

// String returns a human readable version of the LastConfirmedMessage.
func (l LastConfirmedMessage) String() string {
	return stringify.Struct("LastConfirmedMessage",
		stringify.StructField("MessageID", l.MessageID),
		stringify.StructField("Time", l.Time),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TimeManagerEvents ////////////////////////////////////////////////////////////////////////////////////////////

// TimeManagerEvents represents events happening in the TimeManager.
type TimeManagerEvents struct {
	// Fired when the nodes sync status changes.
	SyncChanged *events.Event
}

// SyncChangedEvent represents a syn changed event.
type SyncChangedEvent struct {
	Synced bool
}

// SyncChangedCaller is the caller function for sync changed event.
func SyncChangedCaller(handler interface{}, params ...interface{}) {
	handler.(func(ev *SyncChangedEvent))(params[0].(*SyncChangedEvent))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
