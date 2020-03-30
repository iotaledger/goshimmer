package events

import (
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/events"
)

type CollectiveBeacon = *events.Event

func NewCollectiveBeaconEvent() *events.Event {
	return events.NewEvent(collectiveBeaconReceived)
}

type CollectiveBeaconEvent struct {
	IssuerPublicKey ed25519.PublicKey // public key of the issuer
	Timestamp       time.Time         // timestamp when the beacon was issued
	InstanceID      uint32            // instanceID of the beacon
	Round           uint64            // round of the current beacon
	PrevSignature   []byte            // collective signature of the previous beacon
	Signature       []byte            // collective signature of the current beacon
	Dpk             []byte            // distributed public key
}

func collectiveBeaconReceived(handler interface{}, params ...interface{}) {
	handler.(func(*CollectiveBeaconEvent))(params[0].(*CollectiveBeaconEvent))
}
