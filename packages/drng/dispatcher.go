package drng

import (
	"errors"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/marshalutil"
)

// Dispatch parses a DRNG message and process it based on its subtype
func (drng *DRNG) Dispatch(issuer ed25519.PublicKey, timestamp time.Time, payload *Payload) error {
	switch payload.PayloadType {
	case TypeCollectiveBeacon:
		// parse as CollectiveBeaconType
		marshalUtil := marshalutil.New(payload.Bytes())
		parsedPayload, err := ParseCollectiveBeaconPayload(marshalUtil)
		if err != nil {
			return err
		}
		// trigger CollectiveBeacon Event
		cbEvent := &CollectiveBeaconEvent{
			IssuerPublicKey: issuer,
			Timestamp:       timestamp,
			InstanceID:      parsedPayload.Header.InstanceID,
			Round:           parsedPayload.Round,
			PrevSignature:   parsedPayload.PrevSignature,
			Signature:       parsedPayload.Signature,
			Dpk:             parsedPayload.Dpk,
		}
		drng.Events.CollectiveBeacon.Trigger(cbEvent)

		// process collectiveBeacon
		if err := ProcessBeacon(drng.State, cbEvent); err != nil {
			return err
		}

		// trigger RandomnessEvent
		drng.Events.Randomness.Trigger(drng.State.Randomness())

		return nil

	default:
		return errors.New("subtype not implemented")
	}
}
