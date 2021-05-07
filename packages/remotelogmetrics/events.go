package remotelogmetrics

import (
	"time"

	"github.com/iotaledger/hive.go/events"
)

// CollectionLogEvents defines the events for the remotelogmetrics package.
type CollectionLogEvents struct {
	// SyncBeaconSyncChanged defines the local sync status change event based on sync beacon.
	SyncBeaconSyncChanged *events.Event
	// TangleTimeSyncChanged defines the local sync status change event based on tangle time.
	TangleTimeSyncChanged *events.Event
}

func boolCaller(handler interface{}, params ...interface{}) {
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
	// SyncType contains the type of sync that changed - tangletime or syncbeacon
	SyncType string `json:"syncType" bson:"syncType"`
	// LastConfirmedMessageTime contains time of the last confirmed message
	LastConfirmedMessageTime time.Time `json:"lastConfirmedMessageTime" bson:"lastConfirmedMessageTime"`
}
