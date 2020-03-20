package collectiveBeacon

import (
	"crypto/sha512"
	"errors"

	"github.com/drand/drand/beacon"
	"github.com/drand/drand/key"
	"github.com/iotaledger/goshimmer/packages/binary/drng/state"
	"github.com/iotaledger/goshimmer/packages/binary/drng/subtypes/collectiveBeacon/events"
	"github.com/iotaledger/goshimmer/packages/binary/drng/subtypes/collectiveBeacon/payload"
	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
	"github.com/iotaledger/goshimmer/packages/binary/signature/ed25119"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction"
)

// ProcessTransaction performs the following tasks:
// 1 - parse as CollectiveBeaconType
// 2 - trigger CollectiveBeaconEvent
// 3 - verify that we have a valid random
// 4 - update drng state
func ProcessTransaction(drng *state.State, event events.CollectiveBeacon, tx *transaction.Transaction) error {
	// 1 - parse as CollectiveBeaconType
	marshalUtil := marshalutil.New(tx.GetPayload().Bytes())
	parsedPayload, err := payload.Parse(marshalUtil)
	if err != nil {
		return err
	}

	// 2 - trigger CollectiveBeaconEvent
	cbEvent := &events.CollectiveBeaconEvent{
		IssuerPublicKey: tx.IssuerPublicKey(),
		Timestamp:       tx.IssuingTime(),
		InstanceID:      parsedPayload.Instance(),
		Round:           parsedPayload.Round(),
		PrevSignature:   parsedPayload.PrevSignature(),
		Signature:       parsedPayload.Signature(),
		Dpk:             parsedPayload.DistributedPK(),
	}
	event.Trigger(cbEvent)

	// 3 - verify that we have a valid random
	if err := VerifyCollectiveBeacon(drng, cbEvent); err != nil {
		//TODO: handle error
		return err
	}

	// 4 - update drng state
	randomness, err := GetRandomness(cbEvent.Signature)
	if err != nil {
		//TODO: handle error
		return err
	}
	newRandomness := &state.Randomness{
		Round:      cbEvent.Round,
		Randomness: randomness,
		Timestamp:  cbEvent.Timestamp,
	}
	drng.SetRandomness(newRandomness)
	return nil
}

// VerifyCollectiveBeacon verifies against a given state that
// the given CollectiveBeaconEvent contains a valid beacon
func VerifyCollectiveBeacon(state *state.State, data *events.CollectiveBeaconEvent) error {

	if err := verifyIssuer(state, data.IssuerPublicKey); err != nil {
		return err
	}

	if err := verifySignature(data); err != nil {
		return err
	}

	return nil
}

// verifyIssuer checks the given issuer is a member of the committee
func verifyIssuer(state *state.State, issuer ed25119.PublicKey) error {
	for _, member := range state.Committee().Identities {
		if member == issuer {
			return nil
		}
	}
	return errors.New("Invalid Issuer")
}

// verifySignature checks the current signature against the distributed public key
func verifySignature(data *events.CollectiveBeaconEvent) error {
	if data == nil {
		return errors.New("nil data")
	}

	dpk := key.KeyGroup.Point()
	if err := dpk.UnmarshalBinary(data.Dpk); err != nil {
		return err
	}

	msg := beacon.Message(data.PrevSignature, data.Round)

	if err := key.Scheme.VerifyRecovered(dpk, msg, data.Signature); err != nil {
		return err
	}

	return nil
}

// GetRandomness returns the randomness from a given signature
func GetRandomness(signature []byte) ([]byte, error) {
	hash := sha512.New()
	if _, err := hash.Write(signature); err != nil {
		return nil, err
	}

	return hash.Sum(nil), nil
}
