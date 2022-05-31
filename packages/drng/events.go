package drng

import (
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/generics/event"
)

// Events holds the different events triggered by a DRNG instance.
type Events struct {
	// Collective Beacon is triggered each time we receive a new CollectiveBeacon message.
	CollectiveBeacon *event.Event[*CollectiveBeaconEvent]

	// Randomness is triggered each time we receive a new and valid CollectiveBeacon message.
	Randomness *event.Event[*RandomnessEvent]
}

// newEvents returns a new Events object.
func newEvents() *Events {
	return &Events{
		CollectiveBeacon: event.New[*CollectiveBeaconEvent](),
		Randomness:       event.New[*RandomnessEvent](),
	}
}

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

// Randomness holds the new state when randonmess is generated.
type RandomnessEvent struct {
	State *State
}
