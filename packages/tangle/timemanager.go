package tangle

import (
	"context"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/generics/event"
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
	bootstrapped         bool

	ctx    context.Context
	cancel context.CancelFunc
}

// NewTimeManager is the constructor for TimeManager.
func NewTimeManager(tangle *Tangle) *TimeManager {
	t := &TimeManager{
		Events:      newTimeManagerEvents(),
		tangle:      tangle,
		startSynced: tangle.Options.StartSynced,
	}
	t.ctx, t.cancel = context.WithCancel(context.Background())

	// initialize with Genesis
	t.lastConfirmedMessage = LastConfirmedMessage{
		MessageID:     EmptyMessageID,
		MessageTime:   time.Unix(DefaultGenesisTime, 0),
		ConfirmedTime: time.Unix(DefaultGenesisTime, 0),
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
	t.bootstrapped = t.lastSynced

	return t
}

// Start starts the TimeManager.
func (t *TimeManager) Start() {
	go t.mainLoop()
}

// Setup sets up the behavior of the component by making it attach to the relevant events of other components.
func (t *TimeManager) Setup() {
	t.tangle.ConfirmationOracle.Events().MessageConfirmed.Attach(event.NewClosure(func(event *MessageConfirmedEvent) {
		t.updateTime(event.Message)
		t.updateSyncedState()
	}))
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

// CTT returns the confirmed tangle Time, i.e., the issuing time of the last confirmed message.
func (t *TimeManager) CTT() time.Time {
	t.lastConfirmedMutex.RLock()
	defer t.lastConfirmedMutex.RUnlock()

	return t.lastConfirmedMessage.MessageTime
}

// FTT returns the finalized tangle time. For now, it's just a stub, it actually returns CTT.
func (t *TimeManager) FTT() time.Time {
	return t.CTT()
}

// Bootstrapped returns whether the node has bootstrapped based on the difference between FTT and the current wall time which can
// be configured via SyncTimeWindow.
// When the node becomes bootstrapped and this method returns true, it can't return false after that.
func (t *TimeManager) Bootstrapped() bool {
	t.lastSyncedMutex.RLock()
	defer t.lastSyncedMutex.RUnlock()
	return t.bootstrapped
}

// Synced returns whether the node is in sync based on the difference between FTT and the current wall time which can
// be configured via SyncTimeWindow.
func (t *TimeManager) Synced() bool {
	t.lastSyncedMutex.RLock()
	defer t.lastSyncedMutex.RUnlock()
	return t.lastSynced
}

func (t *TimeManager) synced() bool {
	if t.startSynced && t.FTT().Unix() == DefaultGenesisTime {
		return true
	}

	return clock.Since(t.FTT()) < t.tangle.Options.SyncTimeWindow
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
		if newSynced {
			t.bootstrapped = true
		}
	}
}

// updateTime updates the last confirmed message.
func (t *TimeManager) updateTime(message *Message) {
	t.lastConfirmedMutex.Lock()
	defer t.lastConfirmedMutex.Unlock()

	if t.lastConfirmedMessage.MessageTime.After(message.IssuingTime()) {
		return
	}
	t.lastConfirmedMessage = LastConfirmedMessage{
		MessageID:     message.ID(),
		MessageTime:   message.IssuingTime(),
		ConfirmedTime: time.Now(),
	}
}

// RCTT return relative confirmed tangle time.
func (t *TimeManager) RCTT() time.Time {
	now := time.Now()
	lastConfirmedTime := t.lastConfirmedTime()
	ctt := t.CTT()
	return ctt.Add(now.Sub(lastConfirmedTime))
}

// RFTT return relative finalized tangle time. For now, it's the same as RCTT.
func (t *TimeManager) RFTT() time.Time {
	return t.RCTT()
}

func (t *TimeManager) lastConfirmedTime() time.Time {
	t.lastConfirmedMutex.RLock()
	defer t.lastConfirmedMutex.RUnlock()
	return t.lastConfirmedMessage.ConfirmedTime
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

// LastConfirmedMessage is a wrapper type for the last confirmed message, consisting of MessageID, MessageTime and ConfirmedTime.
type LastConfirmedMessage struct {
	MessageID MessageID `serix:"0"`
	// MessageTime field is the time of the last confirmed message.
	MessageTime time.Time `serix:"1"`
	// ConfirmedTime field is the time when the last confirmed message was updated.
	ConfirmedTime time.Time `serix:"2"`
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
		stringify.StructField("MessageTime", l.MessageTime),
		stringify.StructField("ConfirmedTime", l.ConfirmedTime),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
