package drng

import (
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/crypto/ed25519"
)

// Dispatch parses a DRNG message and processes it based on its subtype
func (d *DRNG) Dispatch(issuer ed25519.PublicKey, timestamp time.Time, payload *CollectiveBeaconPayload) error {
	switch payload.PayloadType {
	case TypeCollectiveBeacon:
		// trigger CollectiveBeacon Event
		cbEvent := &CollectiveBeaconEvent{
			IssuerPublicKey: issuer,
			Timestamp:       timestamp,
			InstanceID:      payload.Header.InstanceID,
			Round:           payload.Round,
			PrevSignature:   payload.PrevSignature,
			Signature:       payload.Signature,
			Dpk:             payload.Dpk,
		}
		d.Events.CollectiveBeacon.Trigger(cbEvent)

		// process collectiveBeacon
		if _, ok := d.State[cbEvent.InstanceID]; !ok {
			return ErrInstanceIDMismatch
		}
		if err := ProcessBeacon(d.State[cbEvent.InstanceID], cbEvent); err != nil {
			return err
		}

		// update the dpk (if not set) from the valid beacon
		if len(d.State[cbEvent.InstanceID].committee.DistributedPK) == 0 {
			d.State[cbEvent.InstanceID].UpdateDPK(cbEvent.Dpk)
		}

		// trigger RandomnessEvent
		d.Events.Randomness.Trigger(d.State[cbEvent.InstanceID])

		return nil

	default:
		return errors.New("subtype not implemented")
	}
}
