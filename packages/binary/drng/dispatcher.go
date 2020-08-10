package drng

import (
	"errors"
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/drng/payload"
	"github.com/iotaledger/goshimmer/packages/binary/drng/payload/header"
	"github.com/iotaledger/goshimmer/packages/binary/drng/subtypes/collectivebeacon"
	"github.com/iotaledger/goshimmer/packages/binary/drng/subtypes/collectivebeacon/events"
	cb "github.com/iotaledger/goshimmer/packages/binary/drng/subtypes/collectivebeacon/payload"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/marshalutil"
)

// Dispatch parses a DRNG message and process it based on its subtype
func (drng *DRNG) Dispatch(issuer ed25519.PublicKey, timestamp time.Time, payload *payload.Payload) error {
	switch payload.Header.PayloadType {
	case header.TypeCollectiveBeacon:
		// parse as CollectiveBeaconType
		marshalUtil := marshalutil.New(payload.Bytes())
		parsedPayload, err := cb.Parse(marshalUtil)
		if err != nil {
			return err
		}
		// trigger CollectiveBeaconEvent
		cbEvent := &events.CollectiveBeaconEvent{
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
		if err := collectivebeacon.ProcessBeacon(drng.State, cbEvent); err != nil {
			return err
		}

		// trigger RandomnessEvent
		drng.Events.Randomness.Trigger(drng.State.Randomness())

		return nil

	default:
		return errors.New("subtype not implemented")
	}
}
