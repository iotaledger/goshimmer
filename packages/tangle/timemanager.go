package tangle

import (
	"context"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/serix"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/timeutil"

	"github.com/iotaledger/goshimmer/packages/clock"
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
	lastSyncedMutex      sync.RWMutex
	lastSynced           bool

	ctx    context.Context
	cancel context.CancelFunc
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
	t.ctx, t.cancel = context.WithCancel(context.Background())

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

// Start starts the TimeManager.
func (t *TimeManager) Start() {
	go t.mainLoop()
}

// Setup sets up the behavior of the component by making it attach to the relevant events of other components.
func (t *TimeManager) Setup() {
	t.tangle.ConfirmationOracle.Events().MessageConfirmed.Attach(events.NewClosure(t.updateTime))
	t.Start()
}

// Shutdown shuts down the TimeManager and persists its state.
func (t *TimeManager) Shutdown() {
	t.lastConfirmedMutex.RLock()
	defer t.lastConfirmedMutex.RUnlock()

	if err := t.tangle.Options.Store.Set(kvstore.Key(lastConfirmedKey), t.lastConfirmedMessage.Bytes()); err != nil {
		t.tangle.Events.Error.Trigger(errors.Errorf("failed to persists LastConfirmedMessage (%v): %w", err, cerrors.ErrFatal))
		return
	}

	// cancel the internal context
	t.cancel()
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
	t.lastSyncedMutex.RLock()
	defer t.lastSyncedMutex.RUnlock()
	return t.lastSynced
}

func (t *TimeManager) synced() bool {
	if t.startSynced && t.lastConfirmedMessage.Time.Unix() == DefaultGenesisTime {
		return true
	}

	return clock.Since(t.lastConfirmedMessage.Time) < t.tangle.Options.SyncTimeWindow
}

// checks whether the synced state needs to be updated and if so,
// triggers a corresponding event reflecting the new state.
func (t *TimeManager) updateSyncedState() {
	t.lastSyncedMutex.Lock()
	defer t.lastSyncedMutex.Unlock()
	if newSynced := t.synced(); t.lastSynced != newSynced {
		t.lastSynced = newSynced
		// trigger the event inside the lock to assure that the status is still correct
		t.Events.SyncChanged.Trigger(&SyncChangedEvent{Synced: newSynced})
	}
}

// updateTime updates the last confirmed message.
func (t *TimeManager) updateTime(messageID MessageID) {
	t.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		t.lastConfirmedMutex.Lock()
		defer t.lastConfirmedMutex.Unlock()

		if t.lastConfirmedMessage.Time.After(message.IssuingTime()) {
			return
		}

		t.lastConfirmedMessage = LastConfirmedMessage{
			MessageID: messageID,
			Time:      message.IssuingTime(),
		}

		t.updateSyncedState()
	})
}

// the main loop runs the updateSyncedState at least every synced time window interval to keep the synced state updated
// even if no updateTime is ever called.
func (t *TimeManager) mainLoop() {
	timeutil.NewTicker(t.updateSyncedState, func() time.Duration {
		if t.tangle.Options.SyncTimeWindow == 0 {
			return DefaultSyncTimeWindow
		}
		return t.tangle.Options.SyncTimeWindow
	}(), t.ctx).WaitForShutdown()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region LastConfirmedMessage /////////////////////////////////////////////////////////////////////////////////////////

// LastConfirmedMessage is a wrapper type for the last confirmed message, consisting of MessageID and time.
type LastConfirmedMessage struct {
	MessageID MessageID `serix:"0"`
	Time      time.Time `serix:"1"`
}

// lastConfirmedMessageFromBytes unmarshals a LastConfirmedMessage object from a sequence of bytes.
func lastConfirmedMessageFromBytes(data []byte) (lcm LastConfirmedMessage, consumedBytes int, err error) {
	consumedBytes, err = serix.DefaultAPI.Decode(context.Background(), data, &lcm, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse Background: %w", err)
		return
	}
	return
}

// Bytes returns a marshaled version of the LastConfirmedMessage.
func (l LastConfirmedMessage) Bytes() (marshaledLastConfirmedMessage []byte) {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), l, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
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

// SyncChangedEvent represents a sync changed event.
type SyncChangedEvent struct {
	Synced bool
}

// SyncChangedCaller is the caller function for sync changed event.
func SyncChangedCaller(handler interface{}, params ...interface{}) {
	handler.(func(ev *SyncChangedEvent))(params[0].(*SyncChangedEvent))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
