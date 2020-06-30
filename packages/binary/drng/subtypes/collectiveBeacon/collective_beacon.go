package collectiveBeacon

import (
	"bytes"
	"crypto/sha512"
	"errors"

	"github.com/drand/drand/beacon"
	"github.com/drand/drand/key"
	"github.com/iotaledger/goshimmer/packages/binary/drng/state"
	"github.com/iotaledger/goshimmer/packages/binary/drng/subtypes/collectiveBeacon/events"
	"github.com/iotaledger/hive.go/crypto/ed25519"
)

var (
	ErrDistributedPubKeyMismatch = errors.New("Distributed Public Key does not match")
	ErrInvalidRound              = errors.New("Invalid Round")
	ErrInstanceIdMismatch        = errors.New("InstanceID does not match")
	ErrInvalidIssuer             = errors.New("Invalid Issuer")
	ErrNilState                  = errors.New("Nil state")
	ErrNilData                   = errors.New("Nil data")
)

// ProcessBeacon performs the following tasks:
// - verify that we have a valid random
// - update drng state
func ProcessBeacon(drng *state.State, cb *events.CollectiveBeaconEvent) error {

	// verify that we have a valid random
	if err := VerifyCollectiveBeacon(drng, cb); err != nil {
		//TODO: handle error
		return err
	}

	// update drng state
	randomness, err := ExtractRandomness(cb.Signature)
	if err != nil {
		//TODO: handle error
		return err
	}
	newRandomness := &state.Randomness{
		Round:      cb.Round,
		Randomness: randomness,
		Timestamp:  cb.Timestamp,
	}

	drng.UpdateRandomness(newRandomness)

	return nil
}

// VerifyCollectiveBeacon verifies against a given state that
// the given CollectiveBeaconEvent contains a valid beacon.
func VerifyCollectiveBeacon(state *state.State, data *events.CollectiveBeaconEvent) error {
	if state == nil {
		return ErrNilState
	}

	if data == nil {
		return ErrNilData
	}

	if err := verifyIssuer(state, data.IssuerPublicKey); err != nil {
		return err
	}

	if !bytes.Equal(data.Dpk, state.Committee().DistributedPK) {
		return ErrDistributedPubKeyMismatch
	}

	if data.Round <= state.Randomness().Round {
		return ErrInvalidRound
	}

	if data.InstanceID != state.Committee().InstanceID {
		return ErrInstanceIdMismatch
	}

	if err := verifySignature(data); err != nil {
		return err
	}

	return nil
}

// verifyIssuer checks the given issuer is a member of the committee.
func verifyIssuer(state *state.State, issuer ed25519.PublicKey) error {
	for _, member := range state.Committee().Identities {
		if member == issuer {
			return nil
		}
	}
	return ErrInvalidIssuer
}

// verifySignature checks the current signature against the distributed public key.
func verifySignature(data *events.CollectiveBeaconEvent) error {
	dpk := key.KeyGroup.Point()
	if err := dpk.UnmarshalBinary(data.Dpk); err != nil {
		return err
	}

	msg := beacon.Message(data.Round, data.PrevSignature)

	if err := key.Scheme.VerifyRecovered(dpk, msg, data.Signature); err != nil {
		return err
	}

	return nil
}

// ExtractRandomness returns the randomness from a given signature.
func ExtractRandomness(signature []byte) ([]byte, error) {
	hash := sha512.New()
	if _, err := hash.Write(signature); err != nil {
		return nil, err
	}

	return hash.Sum(nil), nil
}
