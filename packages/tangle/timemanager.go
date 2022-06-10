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
	lastConfirmedKey = "LastAcceptedMessage"
)

// region TimeManager //////////////////////////////////////////////////////////////////////////////////////////////////

// TimeManager is a Tangle component that keeps track of the TangleTime. The TangleTime can be seen as a clock for the
// entire network as it tracks the time of the last confirmed message. Comparing the issuing time of the last confirmed
// message to the node's current wall clock time then yields a reasonable assessment of how much in sync the node is.
type TimeManager struct {
	Events *TimeManagerEvents

	tangle      *Tangle
	startSynced bool

	lastAcceptedMutex   sync.RWMutex
	lastAcceptedMessage LastMessage
	lastSyncedMutex     sync.RWMutex
	lastSynced          bool
	bootstrapped        bool

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
	t.lastAcceptedMessage = LastMessage{
		MessageID:   EmptyMessageID,
		MessageTime: time.Unix(DefaultGenesisTime, 0),
		UpdateTime:  time.Unix(DefaultGenesisTime, 0),
	}

	marshaledLastConfirmedMessage, err := tangle.Options.Store.Get(kvstore.Key(lastConfirmedKey))
	if err != nil && !errors.Is(err, kvstore.ErrKeyNotFound) {
		panic(err)
	}
	// load from storage if key was found
	if marshaledLastConfirmedMessage != nil {
		if t.lastAcceptedMessage, _, err = lastMessageFromBytes(marshaledLastConfirmedMessage); err != nil {
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
	t.lastAcceptedMutex.RLock()
	defer t.lastAcceptedMutex.RUnlock()

	if err := t.tangle.Options.Store.Set(kvstore.Key(lastConfirmedKey), t.lastAcceptedMessage.Bytes()); err != nil {
		t.tangle.Events.Error.Trigger(errors.Errorf("failed to persists LastAcceptedMessage (%v): %w", err, cerrors.ErrFatal))
		return
	}

	// cancel the internal context
	t.cancel()
}

// LastAcceptedMessage returns the last confirmed message.
func (t *TimeManager) LastAcceptedMessage() LastMessage {
	t.lastAcceptedMutex.RLock()
	defer t.lastAcceptedMutex.RUnlock()

	return t.lastAcceptedMessage
}

// LastConfirmedMessage returns the last confirmed message.
func (t *TimeManager) LastConfirmedMessage() LastMessage {
	t.lastAcceptedMutex.RLock()
	defer t.lastAcceptedMutex.RUnlock()

	return t.lastAcceptedMessage
}

// AT returns the Acceptance Time, i.e., the issuing time of the last accepted message.
func (t *TimeManager) AT() time.Time {
	t.lastAcceptedMutex.RLock()
	defer t.lastAcceptedMutex.RUnlock()

	return t.lastAcceptedMessage.MessageTime
}

// CT returns the confirmed time, i.e. the issuing time of the last confirmed message.
// For now, it's just a stub, it actually returns AT.
func (t *TimeManager) CT() time.Time {
	return t.AT()
}

// RAT return relative acceptance time, i.e., AT + time since last update of AT.
func (t *TimeManager) RAT() time.Time {
	now := time.Now()
	lastConfirmedTime := t.lastAcceptedTime()
	ctt := t.AT()
	return ctt.Add(now.Sub(lastConfirmedTime))
}

// RCT return relative acceptance time, i.e., CT + time since last update of CT.
// For now, it's just a stub, it actually returns RAT.
func (t *TimeManager) RCT() time.Time {
	return t.RAT()
}

// Bootstrapped returns whether the node has bootstrapped based on the difference between CT and the current wall time which can
// be configured via SyncTimeWindow.
// When the node becomes bootstrapped and this method returns true, it can't return false after that.
func (t *TimeManager) Bootstrapped() bool {
	t.lastSyncedMutex.RLock()
	defer t.lastSyncedMutex.RUnlock()
	return t.bootstrapped
}

// Synced returns whether the node is in sync based on the difference between CT and the current wall time which can
// be configured via SyncTimeWindow.
func (t *TimeManager) Synced() bool {
	t.lastSyncedMutex.RLock()
	defer t.lastSyncedMutex.RUnlock()
	return t.lastSynced
}

func (t *TimeManager) synced() bool {
	if t.startSynced && t.CT().Unix() == DefaultGenesisTime {
		return true
	}

	return clock.Since(t.CT()) < t.tangle.Options.SyncTimeWindow
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
	t.lastAcceptedMutex.Lock()
	defer t.lastAcceptedMutex.Unlock()

	if t.lastAcceptedMessage.MessageTime.After(message.IssuingTime()) {
		return
	}
	t.lastAcceptedMessage = LastMessage{
		MessageID:   message.ID(),
		MessageTime: message.IssuingTime(),
		UpdateTime:  time.Now(),
	}
}

func (t *TimeManager) lastAcceptedTime() time.Time {
	t.lastAcceptedMutex.RLock()
	defer t.lastAcceptedMutex.RUnlock()
	return t.lastAcceptedMessage.UpdateTime
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

// region LastAcceptedMessage /////////////////////////////////////////////////////////////////////////////////////////

// LastMessage is a wrapper type for the last confirmed message, consisting of MessageID, MessageTime and UpdateTime.
type LastMessage struct {
	MessageID MessageID `serix:"0"`
	// MessageTime field is the time of the last confirmed message.
	MessageTime time.Time `serix:"1"`
	// UpdateTime field is the time when the last confirmed message was updated.
	UpdateTime time.Time `serix:"2"`
}

// lastMessageFromBytes unmarshals a LastMessage object from a sequence of bytes.
func lastMessageFromBytes(data []byte) (lcm LastMessage, consumedBytes int, err error) {
	consumedBytes, err = serix.DefaultAPI.Decode(context.Background(), data, &lcm, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse Background: %w", err)
		return
	}
	return
}

// Bytes returns a marshaled version of the LastMessage.
func (l LastMessage) Bytes() (marshaledLastConfirmedMessage []byte) {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), l, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// String returns a human-readable version of the LastMessage.
func (l LastMessage) String() string {
	return stringify.Struct("LastMessage",
		stringify.StructField("MessageID", l.MessageID),
		stringify.StructField("MessageTime", l.MessageTime),
		stringify.StructField("UpdateTime", l.UpdateTime),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
