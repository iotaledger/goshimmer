package remotelogmetrics

import (
	"time"

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
	// Time defines the time when the conflict has been finalized.
	Time time.Time `json:"datetime" bson:"datetime"`
}

type TransactionMetricsLogger struct {
	Type               string    `json:"type" bson:"type"`
	MessageID          string    `json:"messageID" bson:"messageID"`
	TransactionID      string    `json:"transactionID" bson:"transactionID"`
	IssuedTimestamp    time.Time `json:"issuedTimestamp" bson:"issuedTimestamp"`
	SolidTimestamp     time.Time `json:"solidTimestamp" bson:"solidTimestamp"`
	ScheduledTimestamp time.Time `json:"scheduledTimestamp" bson:"scheduledTimestamp"`
	BookedTimestamp    time.Time `json:"bookedTimestamp" bson:"bookedTimestamp"`
	ConfirmedTimestamp time.Time `json:"confirmedTimestamp" bson:"confirmedTimestamp"`
}
