package tangle

import (
	"errors"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/epochs"
	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/xerrors"
)

const (
	lastConfirmedKey = "lastConfirmedMessage"
)

// region TimeManager //////////////////////////////////////////////////////////////////////////////////////////////////

// TimeManager is a Tangle component that keeps track of the TangleTime. The TangleTime can be seen as a clock for the
// entire network as it tracks the time of the last confirmed message. Comparing the issuing time of the last confirmed
// message to the node's current wall clock time then yields a reasonable assessment of how much in sync the node is.
type TimeManager struct {
	tangle *Tangle

	lastConfirmedMessage lastConfirmedMessage
	lastConfirmedMutex   sync.RWMutex
}

// NewTimeManager is the constructor for TimeManager.
func NewTimeManager(tangle *Tangle) (timeManager *TimeManager) {
	timeManager = &TimeManager{
		tangle: tangle,
	}

	marshaledLastConfirmedMessage, err := tangle.Options.Store.Get(kvstore.Key(lastConfirmedKey))
	if err != nil && !errors.Is(err, kvstore.ErrKeyNotFound) {
		panic(err)
	}
	// Load from storage if key was found.
	if marshaledLastConfirmedMessage != nil {
		if timeManager.lastConfirmedMessage, _, err = lastConfirmedMessageFromBytes(marshaledLastConfirmedMessage); err != nil {
			panic(err)
		}
		return
	}

	// Initialize with Genesis if not found in storage.
	timeManager.lastConfirmedMessage = lastConfirmedMessage{
		MessageID: EmptyMessageID,
		Time:      time.Unix(epochs.DefaultGenesisTime, 0),
	}

	return
}

// Setup sets up the behavior of the component by making it attach to the relevant events of other components.
func (t *TimeManager) Setup() {
	// TODO: maybe it is better to attach to an event for each message being confirmed.
	//  However, we're confirming a marker and walking in its past cone, hence it should have the most recent timestamp.
	t.tangle.ApprovalWeightManager.Events.MarkerConfirmation.Attach(events.NewClosure(t.updateTime))
}

// Shutdown shuts down the TimeManager and persists its state.
func (t *TimeManager) Shutdown() {
	t.lastConfirmedMutex.RLock()
	defer t.lastConfirmedMutex.RUnlock()

	if err := t.tangle.Options.Store.Set(kvstore.Key(lastConfirmedKey), t.lastConfirmedMessage.Bytes()); err != nil {
		t.tangle.Events.Error.Trigger(xerrors.Errorf("failed to persists lastConfirmedMessage (%v): %w", err, cerrors.ErrFatal))
		return
	}
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

	return clock.SyncedTime().Sub(t.lastConfirmedMessage.Time) < t.tangle.Options.SyncTimeWindow
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
			t.lastConfirmedMessage = lastConfirmedMessage{
				MessageID: messageID,
				Time:      message.IssuingTime(),
			}
		}
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region lastConfirmedMessage /////////////////////////////////////////////////////////////////////////////////////////

// lastConfirmedMessage is a wrapper type for the last confirmed message, consisting of MessageID and time.
type lastConfirmedMessage struct {
	MessageID MessageID
	Time      time.Time
}

// lastConfirmedMessageFromBytes unmarshals a lastConfirmedMessage object from a sequence of bytes.
func lastConfirmedMessageFromBytes(bytes []byte) (lcm lastConfirmedMessage, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if lcm, err = lastConfirmedMessageFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse lastConfirmedMessage from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// lastConfirmedMessageFromMarshalUtil unmarshals a lastConfirmedMessage object using a MarshalUtil (for easier unmarshaling).
func lastConfirmedMessageFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (lcm lastConfirmedMessage, err error) {
	lcm = lastConfirmedMessage{}
	if lcm.MessageID, err = MessageIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse MessageID from MarshalUtil: %w", err)
		return
	}

	if lcm.Time, err = marshalUtil.ReadTime(); err != nil {
		err = xerrors.Errorf("failed to parse time (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// Bytes returns a marshaled version of the lastConfirmedMessage.
func (l lastConfirmedMessage) Bytes() (marshaledLastConfirmedMessage []byte) {
	return marshalutil.New(MessageIDLength + marshalutil.TimeSize).
		Write(l.MessageID).
		WriteTime(l.Time).
		Bytes()
}

// String returns a human readable version of the lastConfirmedMessage.
func (l lastConfirmedMessage) String() string {
	return stringify.Struct("lastConfirmedMessage",
		stringify.StructField("MessageID", l.MessageID),
		stringify.StructField("Time", l.Time),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
