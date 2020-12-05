package drng

import (
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/events"
)

// CollectiveBeaconEvent holds data about a collective beacon event.
type CollectiveBeaconEvent struct {
	// Public key of the issuer.
	IssuerPublicKey ed25519.PublicKey
	// Timestamp when the beacon was issued.
	Timestamp time.Time
	// InstanceID of the beacon.
	InstanceID uint32
	// Round of the current beacon.
	Round uint64
	// Collective signature of the previous beacon.
	PrevSignature []byte
	// Collective signature of the current beacon.
	Signature []byte
	// The distributed public key.
	Dpk []byte
}

// CollectiveBeaconReceived returns the data of a collective beacon event.
func CollectiveBeaconReceived(handler interface{}, params ...interface{}) {
	handler.(func(*CollectiveBeaconEvent))(params[0].(*CollectiveBeaconEvent))
}

// Event holds the different events triggered by a DRNG instance.
type Event struct {
	// Collective Beacon is triggered each time we receive a new CollectiveBeacon message.
	CollectiveBeacon *events.Event
	// Randomness is triggered each time we receive a new and valid CollectiveBeacon message.
	Randomness *events.Event
}

func newEvent() *Event {
	return &Event{
		CollectiveBeacon: events.NewEvent(CollectiveBeaconReceived),
		Randomness:       events.NewEvent(randomnessReceived),
	}
}

func randomnessReceived(handler interface{}, params ...interface{}) {
	handler.(func(*State))(params[0].(*State))
}
