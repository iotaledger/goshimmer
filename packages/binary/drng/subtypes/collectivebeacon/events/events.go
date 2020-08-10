package events

import (
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
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
