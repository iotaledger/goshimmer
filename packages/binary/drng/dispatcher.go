package drng

import (
	"errors"
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/drng/payload"
	"github.com/iotaledger/goshimmer/packages/binary/drng/payload/header"
	"github.com/iotaledger/goshimmer/packages/binary/drng/subtypes/collectiveBeacon"
	"github.com/iotaledger/goshimmer/packages/binary/drng/subtypes/collectiveBeacon/events"
	cb "github.com/iotaledger/goshimmer/packages/binary/drng/subtypes/collectiveBeacon/payload"
	"github.com/iotaledger/goshimmer/packages/binary/signature/ed25119"
	"github.com/iotaledger/hive.go/marshalutil"
)

func (drng *Instance) Dispatch(issuer ed25119.PublicKey, timestamp time.Time, payload *payload.Payload) error {
	switch payload.SubType() {
	case header.CollectiveBeaconType():
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
			InstanceID:      parsedPayload.Instance(),
			Round:           parsedPayload.Round(),
			PrevSignature:   parsedPayload.PrevSignature(),
			Signature:       parsedPayload.Signature(),
			Dpk:             parsedPayload.DistributedPK(),
		}
		drng.Events.CollectiveBeacon.Trigger(cbEvent)

		// process collectiveBeacon
		if err := collectiveBeacon.ProcessBeacon(drng.State, cbEvent); err != nil {
			return err
		}

		// trigger RandomnessEvent
		drng.Events.Randomness.Trigger(drng.State.Randomness())

		return nil

	default:
		//do other stuff
		return errors.New("subtype not implemented")
	}
}
