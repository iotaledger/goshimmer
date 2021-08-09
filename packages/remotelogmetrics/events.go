package remotelogmetrics

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/consensus/fcob"

	"github.com/iotaledger/hive.go/events"
)

// CollectionLogEvents defines the events for the remotelogmetrics package.
type CollectionLogEvents struct {
	// TangleTimeSyncChanged defines the local sync status change event based on tangle time.
	TangleTimeSyncChanged *events.Event
}

// SyncStatusChangedEventCaller is called when a node changes its sync status.
func SyncStatusChangedEventCaller(handler interface{}, params ...interface{}) {
	handler.(func(SyncStatusChangedEvent))(params[0].(SyncStatusChangedEvent))
}

// SyncStatusChangedEvent is triggered by a node when its sync status changes. It is also structure that is sent to remote logger.
type SyncStatusChangedEvent struct {
	// Type defines the type of the message.
	Type string `json:"type" bson:"type"`
	// NodeID defines the ID of the node.
	NodeID string `json:"nodeid" bson:"nodeid"`
	// Time defines the time when the sync status changed.
	Time time.Time `json:"datetime" bson:"datetime"`
	// CurrentStatus contains current sync status
	CurrentStatus bool `json:"currentStatus" bson:"currentStatus"`
	// PreviousStatus contains previous sync status
	PreviousStatus bool `json:"previousStatus" bson:"previousStatus"`
	// LastConfirmedMessageTime contains time of the last confirmed message
	LastConfirmedMessageTime time.Time `json:"lastConfirmedMessageTime" bson:"lastConfirmedMessageTime"`
}

// FPCConflictRecord defines the FPC conflict record to sent be to remote logger.
type FPCConflictRecord struct {
	// Type defines the type of the message.
	Type string `json:"type" bson:"type"`
	// ConflictID defines the ID of the conflict.
	ConflictID string `json:"conflictid" bson:"conflictid"`
	// NodeID defines the ID of the node.
	NodeID string `json:"nodeid" bson:"nodeid"`
	// Rounds defines number of rounds performed to finalize the conflict.
	Rounds int `json:"rounds" bson:"rounds"`
	// Opinions contains the opinion of each round.
	Opinions []int32 `json:"opinions" bson:"opinions"`
	// Outcome defines final opinion of the conflict.
	Outcome int32 `json:"outcome,omitempty" bson:"outcome,omitempty"`
	// Time defines the time when the voting round has been finalized.
	Time time.Time `json:"datetime" bson:"datetime"`
	// ConflictCreationTime points to time when the context has been created
	ConflictCreationTime time.Time `json:"conflictStart" bson:"conflictStart"`
	Delta                int64     `json:"delta"`
}

// TransactionMetrics defines the transaction metrics record to sent be to remote logger.
type TransactionMetrics struct {
	Type               string    `json:"type" bson:"type"`
	NodeID             string    `json:"nodeID" bson:"nodeID"`
	MessageID          string    `json:"messageID" bson:"messageID"`
	TransactionID      string    `json:"transactionID" bson:"transactionID"`
	IssuedTimestamp    time.Time `json:"issuedTimestamp" bson:"issuedTimestamp"`
	SolidTimestamp     time.Time `json:"solidTimestamp" bson:"solidTimestamp"`
	ScheduledTimestamp time.Time `json:"scheduledTimestamp" bson:"scheduledTimestamp"`
	BookedTimestamp    time.Time `json:"bookedTimestamp" bson:"bookedTimestamp"`
	ConfirmedTimestamp time.Time `json:"confirmedTimestamp" bson:"confirmedTimestamp"`
	DeltaSolid         int64     `json:"deltaSolid"`
	DeltaScheduled     int64     `json:"deltaArrival"`
	DeltaBooked        int64     `json:"deltaBooked"`
	DeltaConfirmed     int64     `json:"deltaConfirmed"`
}

// TransactionOpinionMetrics defines the transaction opinion metrics record to sent to the remote logger.
type TransactionOpinionMetrics struct {
	Type             string                `json:"type" bson:"type"`
	NodeID           string                `json:"nodeID" bson:"nodeID"`
	MessageID        string                `json:"messageID" bson:"messageID"`
	TransactionID    string                `json:"transactionID" bson:"transactionID"`
	Fcob1Time        time.Time             `json:"fcob1Time" bson:"fcob1Time"`
	Fcob2Time        time.Time             `json:"fcob2Time" bson:"fcob2Time"`
	Liked            bool                  `json:"liked" bson:"liked"`
	LevelOfKnowledge fcob.LevelOfKnowledge `json:"levelOfKnowledge" bson:"levelOfKnowledge"`
	Timestamp        time.Time             `json:"timestamp" bson:"timestamp"`
}

// DRNGMetrics defines the DRNG metrics record to sent be to remote logger.
type DRNGMetrics struct {
	Type              string    `json:"type" bson:"type"`
	NodeID            string    `json:"nodeID" bson:"nodeID"`
	InstanceID        uint32    `json:"instanceID" bson:"instanceID"`
	Round             uint64    `json:"round" bson:"round"`
	IssuedTimestamp   time.Time `json:"issuedTimestamp" bson:"issuedTimestamp"`
	ReceivedTimestamp time.Time `json:"receivedTimestamp" bson:"receivedTimestamp"`
	DeltaReceived     int64     `json:"deltaReceived"  bson:"deltaReceived"`
}

// StatementLog defines the statement metrics record to sent be to remote logger.
type StatementLog struct {
	NodeID       string    `json:"nodeID"`
	MsgID        string    `json:"msgID"`
	IssuerID     string    `json:"issuerID"`
	IssuedTime   time.Time `json:"issuedTime"`
	ArrivalTime  time.Time `json:"arrivalTime"`
	SolidTime    time.Time `json:"solidTime"`
	DeltaArrival int64     `json:"deltaArrival"`
	DeltaSolid   int64     `json:"deltaSolid"`
	Clock        bool      `json:"clock"`
	Sync         bool      `json:"sync"`
	Type         string    `json:"type"`
}
