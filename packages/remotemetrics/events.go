package remotemetrics

import (
	"time"

	"github.com/iotaledger/hive.go/generics/event"
)

// CollectionLogEvents defines the events for the remotelogmetrics package.
type CollectionLogEvents struct {
	// TangleTimeSyncChanged defines the local sync status change event based on tangle time.
	TangleTimeSyncChanged *event.Event[*TangleTimeSyncChangedEvent]
	SchedulerQuery        *event.Event[*SchedulerQueryEvent]
}

func newCollectionLogEvents() *CollectionLogEvents {
	return &CollectionLogEvents{
		TangleTimeSyncChanged: event.New[*TangleTimeSyncChangedEvent](),
		SchedulerQuery:        event.New[*SchedulerQueryEvent](),
	}
}

// TangleTimeSyncChangedEvent is triggered by a node when its sync status changes. It is also structure that is sent to remote logger.
type TangleTimeSyncChangedEvent struct {
	// Type defines the type of the block.
	Type string `json:"type" bson:"type"`
	// NodeID defines the ID of the node.
	NodeID string `json:"nodeid" bson:"nodeid"`
	// MetricsLevel defines the amount of metrics that are sent by the node.
	MetricsLevel uint8 `json:"metricsLevel" bson:"metricsLevel"`
	// Time defines the time when the sync status changed.
	Time time.Time `json:"datetime" bson:"datetime"`
	// CurrentStatus contains current sync status
	CurrentStatus bool `json:"currentStatus" bson:"currentStatus"`
	// PreviousStatus contains previous sync status
	PreviousStatus bool `json:"previousStatus" bson:"previousStatus"`
	// ATT contains time of the last accepted block
	ATT time.Time `json:"acceptanceTangleTime" bson:"acceptanceTangleTime"`
	// RATT contains relative time of the last accepted block
	RATT time.Time `json:"relativeAcceptanceTangleTime" bson:"relativeAcceptanceTangleTime"`
	// CTT contains time of the last confirmed block
	CTT time.Time `json:"confirmedTangleTime" bson:"confirmedTangleTime"`
	// RCTT contains relative time of the last confirmed block
	RCTT time.Time `json:"relativeConfirmedTangleTime" bson:"relativeConfirmedTangleTime"`
}

// SchedulerQueryEvent is used to trigger scheduler metric collection for remote metric monitoring.
type SchedulerQueryEvent struct {
	Time time.Time
}

// BlockFinalizedMetrics defines the transaction metrics record that is sent to remote logger.
type BlockFinalizedMetrics struct {
	Type                 string    `json:"type" bson:"type"`
	NodeID               string    `json:"nodeID" bson:"nodeID"`
	IssuerID             string    `json:"issuerID" bson:"issuerID"`
	MetricsLevel         uint8     `json:"metricsLevel" bson:"metricsLevel"`
	BlockID              string    `json:"blockID" bson:"blockID"`
	TransactionID        string    `json:"transactionID,omitempty" bson:"transactionID"`
	IssuedTimestamp      time.Time `json:"issuedTimestamp" bson:"issuedTimestamp"`
	SolidTimestamp       time.Time `json:"solidTimestamp,omitempty" bson:"solidTimestamp"`
	ScheduledTimestamp   time.Time `json:"scheduledTimestamp" bson:"scheduledTimestamp"`
	BookedTimestamp      time.Time `json:"bookedTimestamp" bson:"bookedTimestamp"`
	ConfirmedTimestamp   time.Time `json:"confirmedTimestamp" bson:"confirmedTimestamp"`
	DeltaSolid           int64     `json:"deltaSolid,omitempty" bson:"deltaSolid"`
	DeltaScheduled       int64     `json:"deltaArrival" bson:"deltaArrival"`
	DeltaBooked          int64     `json:"deltaBooked" bson:"deltaBooked"`
	DeltaConfirmed       int64     `json:"deltaConfirmed" bson:"deltaConfirmed"`
	StrongEdgeCount      int       `json:"strongEdgeCount" bson:"strongEdgeCount"`
	WeakEdgeCount        int       `json:"weakEdgeCount,omitempty" bson:"weakEdgeCount"`
	ShallowLikeEdgeCount int       `json:"shallowLikeEdgeCount,omitempty" bson:"likeEdgeCount"`
}

// BlockScheduledMetrics defines the scheduling block confirmation metrics record that is sent to remote logger.
type BlockScheduledMetrics struct {
	Type          string `json:"type" bson:"type"`
	NodeID        string `json:"nodeID" bson:"nodeID"`
	IssuerID      string `json:"issuerID" bson:"issuerID"`
	MetricsLevel  uint8  `json:"metricsLevel" bson:"metricsLevel"`
	BlockID       string `json:"blockID" bson:"blockID"`
	TransactionID string `json:"transactionID,omitempty" bson:"transactionID"`
	// Time where the block was created by the issuing node
	IssuedTimestamp time.Time `json:"issuedTimestamp" bson:"issuedTimestamp"`
	// Time where the block was first seen by the node
	ReceivedTimestamp          time.Time `json:"receivedTimestamp" bson:"receivedTimestamp"`
	SolidTimestamp             time.Time `json:"solidTimestamp,omitempty" bson:"solidTimestamp"`
	ScheduledTimestamp         time.Time `json:"scheduledTimestamp,omitempty" bson:"scheduledTimestamp"`
	BookedTimestamp            time.Time `json:"bookedTimestamp" bson:"bookedTimestamp"`
	QueuedTimestamp            time.Time `json:"queuedTimestamp" bson:"queuedTimestamp"`
	DroppedTimestamp           time.Time `json:"droppedTimestamp,omitempty" bson:"DroppedTimestamp"`
	ConfirmationStateTimestamp time.Time `json:"confirmationStateTimestamp,omitempty" bson:"ConfirmationStateTimestamp"`
	ConfirmationState          uint8     `json:"confirmationState" bson:"ConfirmationState"`
	DeltaConfirmationStateTime int64     `json:"deltaConfirmationStateTime" bson:"deltaConfirmationStateTime"`
	DeltaSolid                 int64     `json:"deltaSolid,omitempty" bson:"deltaSolid"`
	// ScheduledTimestamp - IssuedTimestamp in nanoseconds
	DeltaScheduledIssued int64 `json:"deltaScheduledIssued" bson:"deltaScheduledIssued"`
	DeltaBooked          int64 `json:"deltaBooked" bson:"deltaBooked"`
	// ScheduledTimestamp - ReceivedTimestamp in nanoseconds
	DeltaScheduledReceived int64 `json:"deltaScheduledReceived" bson:"deltaScheduledReceived"`
	// ReceivedTimestamp - IssuedTimestamp in nanoseconds
	DeltaReceivedIssued int64 `json:"DeltaReceivedIssued" bson:"DeltaReceivedIssued"`
	// ScheduledTimestamp - QueuedTimestamp in nanoseconds
	SchedulingTime  int64 `json:"schedulingTime" bson:"schedulingTime"`
	AccessMana      int64 `json:"accessMana" bson:"accessMana"`
	StrongEdgeCount int   `json:"strongEdgeCount" bson:"strongEdgeCount"`
	WeakEdgeCount   int   `json:"weakEdgeCount,omitempty" bson:"weakEdgeCount"`
	LikeEdgeCount   int   `json:"likeEdgeCount,omitempty" bson:"likeEdgeCount"`
}

// MissingBlockMetrics defines block solidification record that is sent to the remote logger.
type MissingBlockMetrics struct {
	Type         string `json:"type" bson:"type"`
	NodeID       string `json:"nodeID" bson:"nodeID"`
	MetricsLevel uint8  `json:"metricsLevel" bson:"metricsLevel"`
	BlockID      string `json:"blockID" bson:"blockID"`
	IssuerID     string `json:"issuerID"  bson:"issuerID"`
}

// BranchConfirmationMetrics defines the branch confirmation metrics record that is sent to remote logger.
type BranchConfirmationMetrics struct {
	Type               string    `json:"type" bson:"type"`
	NodeID             string    `json:"nodeID" bson:"nodeID"`
	IssuerID           string    `json:"issuerID" bson:"issuerID"`
	MetricsLevel       uint8     `json:"metricsLevel" bson:"metricsLevel"`
	BlockID            string    `json:"blockID" bson:"blockID"`
	BranchID           string    `json:"transactionID" bson:"transactionID"`
	CreatedTimestamp   time.Time `json:"createdTimestamp" bson:"createdTimestamp"`
	ConfirmedTimestamp time.Time `json:"confirmedTimestamp" bson:"confirmedTimestamp"`
	DeltaConfirmed     int64     `json:"deltaConfirmed" bson:"deltaConfirmed"`
}

// SchedulerMetrics defines the schedule metrics sent to the remote logger.
type SchedulerMetrics struct {
	Type                         string             `json:"type" bson:"type"`
	NodeID                       string             `json:"nodeID" bson:"nodeID"`
	Synced                       bool               `json:"synced" bson:"synced"`
	MetricsLevel                 uint8              `json:"metricsLevel" bson:"metricsLevel"`
	QueueLengthPerNode           map[string]uint32  `json:"queueLengthPerNode" bson:"queueLengthPerNode"`
	AManaNormalizedLengthPerNode map[string]float64 `json:"aManaNormalizedQueueLengthPerNode" bson:"aManaNormalizedQueueLengthPerNode"`
	BufferSize                   uint32             `json:"bufferSize" bson:"bufferSize"`
	BufferLength                 uint32             `json:"bufferLength" bson:"bufferLength"`
	ReadyBlocksInBuffer          uint32             `json:"readyBlocksInBuffer" bson:"readyBlocksInBuffer"`
	Timestamp                    time.Time          `json:"timestamp" bson:"timestamp"`
}

// BranchCountUpdate defines the branch confirmation metrics record that is sent to remote logger.
type BranchCountUpdate struct {
	Type                           string `json:"type" bson:"type"`
	NodeID                         string `json:"nodeID" bson:"nodeID"`
	MetricsLevel                   uint8  `json:"metricsLevel" bson:"metricsLevel"`
	TotalBranchCount               uint64 `json:"totalBranchCount" bson:"totalBranchCount"`
	FinalizedBranchCount           uint64 `json:"finalizedBranchCount" bson:"finalizedBranchCount"`
	ConfirmedBranchCount           uint64 `json:"confirmedBranchCount" bson:"confirmedBranchCount"`
	InitialTotalBranchCount        uint64 `json:"initialTotalBranchCount" bson:"initialTotalBranchCount"`
	TotalBranchCountSinceStart     uint64 `json:"totalBranchCountSinceStart" bson:"totalBranchCountSinceStart"`
	InitialConfirmedBranchCount    uint64 `json:"initialConfirmedBranchCount" bson:"initialConfirmedBranchCount"`
	ConfirmedBranchCountSinceStart uint64 `json:"confirmedBranchCountSinceStart" bson:"confirmedBranchCountSinceStart"`
	InitialFinalizedBranchCount    uint64 `json:"initialFinalizedBranchCount" bson:"initialFinalizedBranchCount"`
	FinalizedBranchCountSinceStart uint64 `json:"finalizedBranchCountSinceStart" bson:"finalizedBranchCountSinceStart"`
}
