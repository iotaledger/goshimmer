package remotelogmetrics

import (
	"sync"

	"github.com/iotaledger/hive.go/events"
)

var (
	once                sync.Once
	collectionLogEvents *CollectionLogEvents
)

func newEvents() *CollectionLogEvents {
	return &CollectionLogEvents{
		TangleTimeSyncChanged: events.NewEvent(boolCaller),
		SyncBeaconSyncChanged: events.NewEvent(boolCaller),
	}
}

// Events returns the events defined in the package.
func Events() *CollectionLogEvents {
	once.Do(func() {
		collectionLogEvents = newEvents()
	})
	return collectionLogEvents
}
