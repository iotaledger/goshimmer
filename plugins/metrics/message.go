package metrics

import (
	"time"

	"github.com/iotaledger/hive.go/syncutils"
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

// MessageType defines the component for the different MPS metrics.
type MessageType byte

const (
	// DataMessage denotes data message type.
	DataMessage MessageType = iota
	// Transaction denotes transaction message.
	Transaction
)

// String returns the stringified component type.
func (c MessageType) String() string {
	switch c {
	case DataMessage:
		return "DataMessage"
	case Transaction:
		return "Transaction"
	default:
		return "Unknown"
	}
}

// ComponentType defines the component for the different MPS metrics.
type ComponentType byte

const (
	// Store denotes messages stored by the message store.
	Store ComponentType = iota
	// Solidifier denotes messages solidified by the solidifier.
	Solidifier
	// Scheduler denotes messages scheduled by the scheduler.
	Scheduler
	// SchedulerDropped denotes messages dropped by the scheduler.
	SchedulerDropped
	// SchedulerSkipped denotes confirmed messages skipped by the scheduler.
	SchedulerSkipped
	// Booker denotes messages booked by the booker.
	Booker
)

// String returns the stringified component type.
func (c ComponentType) String() string {
	switch c {
	case Store:
		return "Store"
	case Solidifier:
		return "Solidifier"
	case Scheduler:
		return "Scheduler"
	case SchedulerDropped:
		return "SchedulerDropped"
	case SchedulerSkipped:
		return "SchedulerSkipped"
	case Booker:
		return "Booker"
	default:
		return "Unknown"
	}
}

// initial values at start of the node.
var (
	// number of solid messages in the database at startup.
	initialMessageCountPerComponentDB = make(map[ComponentType]uint64)

	// initial number of missing messages in missingMessageStorage (at startup).
	initialMissingMessageCountDB uint64

	// helper variable that is only calculated at init phase. unit is milliseconds!
	initialSumTimeSinceReceived = make(map[ComponentType]time.Duration)

	// sum of time messages spend in the queue (since start of the node).
	initialSumSchedulerBookedTime time.Duration
)

// the same metrics as above, but since the start of a node.
var (
	// Number of messages per component (store, scheduler, booker) type since start of the node.
	// One for dashboard (reset every time is read), other for grafana with cumulative value.
	messageCountPerComponentDashboard = make(map[ComponentType]uint64)
	messageCountPerComponentGrafana   = make(map[ComponentType]uint64)

	// protect map from concurrent read/write.
	messageCountPerComponentMutex syncutils.RWMutex

	// current number of missing messages in missingMessageStorage.
	missingMessageCountDB atomic.Uint64

	// sum of times since received (since start of the node).
	sumTimesSinceReceived = make(map[ComponentType]time.Duration)

	// sum of times since issued (since start of the node).
	sumTimesSinceIssued = make(map[ComponentType]time.Duration)
	sumTimeMutex        syncutils.RWMutex

	// sum of time messages spend in the queue (since start of the node).
	sumSchedulerBookedTime time.Duration
	schedulerTimeMutex     syncutils.RWMutex
)

// other metrics stored since the start of a node.
var (
	// current number of finalized messages.
	finalizedMessageCount      = make(map[MessageType]uint64)
	finalizedMessageCountMutex syncutils.RWMutex

	// total time it took all messages to finalize after being received. unit is milliseconds!
	messageFinalizationReceivedTotalTime = make(map[MessageType]uint64)

	// total time it took all messages to finalize after being issued. unit is milliseconds!
	messageFinalizationIssuedTotalTime = make(map[MessageType]uint64)
	messageFinalizationTotalTimeMutex  syncutils.RWMutex

	// current number of message tips.
	messageTips atomic.Uint64

	// total number of parents of all messages per parent type.
	parentsCountPerType      = make(map[tangle.ParentsType]uint64)
	parentsCountPerTypeMutex syncutils.RWMutex

	// Number of messages per payload type since start of the node.
	messageCountPerPayload      = make(map[payload.Type]uint64)
	messageCountPerPayloadMutex syncutils.RWMutex

	// number of messages being requested by the message layer.
	requestQueueSize atomic.Int64

	// number of messages being requested by the message layer.
	solidificationRequests atomic.Uint64

	// counter for the received MPS (for dashboard).
	mpsReceivedSinceLastMeasurement atomic.Uint64
)

////// Exported functions to obtain metrics from outside //////

// MessageCountSinceStartPerPayload returns a map of message payload types and their count since the start of the node.
func MessageCountSinceStartPerPayload() map[payload.Type]uint64 {
	messageCountPerPayloadMutex.RLock()
	defer messageCountPerPayloadMutex.RUnlock()

	// copy the original map
	clone := make(map[payload.Type]uint64)
	for key, element := range messageCountPerPayload {
		clone[key] = element
	}

	return clone
}

// MessageCountSinceStartPerComponentGrafana returns a map of message count per component types and their count since the start of the node.
func MessageCountSinceStartPerComponentGrafana() map[ComponentType]uint64 {
	messageCountPerComponentMutex.RLock()
	defer messageCountPerComponentMutex.RUnlock()

	// copy the original map
	clone := make(map[ComponentType]uint64)
	for key, element := range messageCountPerComponentGrafana {
		clone[key] = element
	}

	return clone
}

// InitialMessageCountPerComponentGrafana returns a map of message count per component types and their count at the start of the node.
func InitialMessageCountPerComponentGrafana() map[ComponentType]uint64 {
	messageCountPerComponentMutex.RLock()
	defer messageCountPerComponentMutex.RUnlock()

	// copy the original map
	clone := make(map[ComponentType]uint64)
	for key, element := range initialMessageCountPerComponentDB {
		clone[key] = element
	}

	return clone
}

// MessageCountSinceStartPerComponentDashboard returns a map of message count per component types and their count since last time the value was read.
func MessageCountSinceStartPerComponentDashboard() map[ComponentType]uint64 {
	messageCountPerComponentMutex.RLock()
	defer messageCountPerComponentMutex.RUnlock()

	// copy the original map
	clone := make(map[ComponentType]uint64)
	for key, element := range messageCountPerComponentDashboard {
		clone[key] = element
	}

	return clone
}

// MessageTips returns the actual number of tips in the message tangle.
func MessageTips() uint64 {
	return messageTips.Load()
}

// SolidificationRequests returns the number of solidification requests since start of node.
func SolidificationRequests() uint64 {
	return solidificationRequests.Load()
}

// MessageRequestQueueSize returns the number of message requests the node currently has registered.
func MessageRequestQueueSize() int64 {
	return requestQueueSize.Load()
}

// SumTimeSinceReceived returns the cumulative time it took for all message to be processed by each component since being received since startup [milliseconds].
func SumTimeSinceReceived() map[ComponentType]int64 {
	sumTimeMutex.RLock()
	defer sumTimeMutex.RUnlock()

	// copy the original map
	clone := make(map[ComponentType]int64)
	for key, element := range sumTimesSinceReceived {
		clone[key] = element.Milliseconds()
	}

	return clone
}

// InitialSumTimeSinceReceived returns the cumulative time it took for all message to be processed by each component since being received at startup [milliseconds].
func InitialSumTimeSinceReceived() map[ComponentType]int64 {
	sumTimeMutex.RLock()
	defer sumTimeMutex.RUnlock()

	// copy the original map
	clone := make(map[ComponentType]int64)
	for key, element := range initialSumTimeSinceReceived {
		clone[key] = element.Milliseconds()
	}

	return clone
}

// SumTimeSinceIssued returns the cumulative time it took for all message to be processed by each component since being issued [milliseconds].
func SumTimeSinceIssued() map[ComponentType]int64 {
	sumTimeMutex.RLock()
	defer sumTimeMutex.RUnlock()

	// copy the original map
	clone := make(map[ComponentType]int64)
	for key, element := range sumTimesSinceIssued {
		clone[key] = element.Milliseconds()
	}

	return clone
}

// SchedulerTime returns the cumulative time it took for all message to become scheduled before startup [milliseconds].
func SchedulerTime() (result int64) {
	schedulerTimeMutex.RLock()
	defer schedulerTimeMutex.RUnlock()
	result = sumSchedulerBookedTime.Milliseconds()
	return
}

// InitialSchedulerTime returns the cumulative time it took for all message to become scheduled at startup [milliseconds].
func InitialSchedulerTime() (result int64) {
	schedulerTimeMutex.RLock()
	defer schedulerTimeMutex.RUnlock()
	result = initialSumSchedulerBookedTime.Milliseconds()
	return
}

// InitialMessageMissingCountDB returns the number of messages in missingMessageStore at startup.
func InitialMessageMissingCountDB() uint64 {
	return initialMissingMessageCountDB
}

// MessageMissingCountDB returns the number of messages in missingMessageStore.
func MessageMissingCountDB() uint64 {
	return missingMessageCountDB.Load()
}

// MessageFinalizationTotalTimeSinceReceivedPerType returns total time message received it took for all messages to finalize per message type.
func MessageFinalizationTotalTimeSinceReceivedPerType() map[MessageType]uint64 {
	messageFinalizationTotalTimeMutex.RLock()
	defer messageFinalizationTotalTimeMutex.RUnlock()

	// copy the original map
	clone := make(map[MessageType]uint64)
	for key, element := range messageFinalizationReceivedTotalTime {
		clone[key] = element
	}

	return clone
}

// MessageFinalizationTotalTimeSinceIssuedPerType returns total time since message issuance it took for all messages to finalize per message type.
func MessageFinalizationTotalTimeSinceIssuedPerType() map[MessageType]uint64 {
	messageFinalizationTotalTimeMutex.RLock()
	defer messageFinalizationTotalTimeMutex.RUnlock()

	// copy the original map
	clone := make(map[MessageType]uint64)
	for key, element := range messageFinalizationIssuedTotalTime {
		clone[key] = element
	}

	return clone
}

// FinalizedMessageCountPerType returns the number of messages finalized per message type.
func FinalizedMessageCountPerType() map[MessageType]uint64 {
	finalizedMessageCountMutex.RLock()
	defer finalizedMessageCountMutex.RUnlock()

	// copy the original map
	clone := make(map[MessageType]uint64)
	for key, element := range finalizedMessageCount {
		clone[key] = element
	}

	return clone
}

// ParentCountPerType returns a map of parent counts per parent type.
func ParentCountPerType() map[tangle.ParentsType]uint64 {
	parentsCountPerTypeMutex.RLock()
	defer parentsCountPerTypeMutex.RUnlock()

	// copy the original map
	clone := make(map[tangle.ParentsType]uint64)
	for key, element := range parentsCountPerType {
		clone[key] = element
	}

	return clone
}

////// Handling data updates and measuring.
func increasePerPayloadCounter(p payload.Type) {
	messageCountPerPayloadMutex.Lock()
	defer messageCountPerPayloadMutex.Unlock()

	// increase cumulative metrics
	messageCountPerPayload[p]++
}

func increasePerComponentCounter(c ComponentType) {
	messageCountPerComponentMutex.Lock()
	defer messageCountPerComponentMutex.Unlock()

	// increase cumulative metrics
	messageCountPerComponentDashboard[c]++
	messageCountPerComponentGrafana[c]++
}

func increasePerParentType(c tangle.ParentsType) {
	parentsCountPerTypeMutex.Lock()
	defer parentsCountPerTypeMutex.Unlock()

	// increase cumulative metrics
	parentsCountPerType[c]++
}

// measures the Component Counter value per second.
func measurePerComponentCounter() {
	// sample the current counter value into a measured MPS value
	componentCounters := MessageCountSinceStartPerComponentDashboard()

	// reset the counter
	messageCountPerComponentMutex.Lock()
	for key := range messageCountPerComponentDashboard {
		messageCountPerComponentDashboard[key] = 0
	}
	messageCountPerComponentMutex.Unlock()

	// trigger events for outside listeners
	Events.ComponentCounterUpdated.Trigger(componentCounters)
}

func measureMessageTips() {
	messageTips.Store(uint64(deps.Tangle.TipManager.TipCount()))
}

// increases the received MPS counter
func increaseReceivedMPSCounter() {
	mpsReceivedSinceLastMeasurement.Inc()
}

// measures the received MPS value
func measureReceivedMPS() {
	// sample the current counter value into a measured MPS value
	sampledMPS := mpsReceivedSinceLastMeasurement.Load()

	// reset the counter
	mpsReceivedSinceLastMeasurement.Store(0)

	// trigger events for outside listeners
	Events.ReceivedMPSUpdated.Trigger(sampledMPS)
}

func measureRequestQueueSize() {
	size := int64(deps.Tangle.Requester.RequestQueueSize())
	requestQueueSize.Store(size)
}

func measureInitialDBStats() {
	dbStatsResult := deps.Tangle.Storage.DBStats()

	initialMessageCountPerComponentDB[Store] = uint64(dbStatsResult.StoredCount)
	initialMessageCountPerComponentDB[Solidifier] = uint64(dbStatsResult.SolidCount)
	initialMessageCountPerComponentDB[Booker] = uint64(dbStatsResult.BookedCount)
	initialMessageCountPerComponentDB[Scheduler] = uint64(dbStatsResult.ScheduledCount)

	initialSumTimeSinceReceived[Solidifier] = dbStatsResult.SumSolidificationReceivedTime
	initialSumTimeSinceReceived[Booker] = dbStatsResult.SumBookedReceivedTime
	initialSumTimeSinceReceived[Scheduler] = dbStatsResult.SumSchedulerReceivedTime

	initialSumSchedulerBookedTime = dbStatsResult.SumSchedulerBookedTime

	initialMissingMessageCountDB = uint64(dbStatsResult.MissingMessageCount)
}
