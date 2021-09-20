package drng

import (
	"bytes"
	"crypto/sha512"
	"fmt"

	"github.com/cockroachdb/errors"

	"github.com/drand/drand/chain"
	"github.com/drand/drand/key"
	"github.com/iotaledger/hive.go/crypto/ed25519"
)

var (
	// ErrDistributedPubKeyMismatch is returned if the distributed public key does not match.
	ErrDistributedPubKeyMismatch = errors.New("Distributed Public Key does not match")
	// ErrInvalidRound is returned if the round is invalid.
	ErrInvalidRound = errors.New("Invalid Round")
	// ErrInstanceIDMismatch is returned if the instanceID does not match.
	ErrInstanceIDMismatch = errors.New("InstanceID does not match")
	// ErrInvalidIssuer is returned if the issuer is invalid.
	ErrInvalidIssuer = errors.New("Invalid Issuer")
	// ErrNilState is returned on nil state.
	ErrNilState = errors.New("Nil state")
	// ErrNilData is returned on nil data.
	ErrNilData = errors.New("Nil data")
)

// ProcessBeacon performs the following tasks:
// - verify that we have a valid random
// - update drng state
func ProcessBeacon(state *State, cb *CollectiveBeaconEvent) error {
	// verify that we have a valid random
	if err := VerifyCollectiveBeacon(state, cb); err != nil {
		// TODO: handle error
		return err
	}

	// update drng state
	randomness, err := ExtractRandomness(cb.Signature)
	if err != nil {
		// TODO: handle error
		return err
	}
	newRandomness := &Randomness{
		Round:      cb.Round,
		Randomness: randomness,
		Timestamp:  cb.Timestamp,
	}

	state.UpdateRandomness(newRandomness)

	return nil
}

// VerifyCollectiveBeacon verifies against a given state that
// the given CollectiveBeaconEvent contains a valid beacon.
func VerifyCollectiveBeacon(state *State, cb *CollectiveBeaconEvent) error {
	if state == nil {
		return ErrNilState
	}

	if cb == nil {
		return ErrNilData
	}

	if err := verifyIssuer(state, cb.IssuerPublicKey); err != nil {
		return err
	}

	wantedCommitteePubKey := state.Committee().DistributedPK
	if len(state.Committee().DistributedPK) != 0 && !bytes.Equal(cb.Dpk, wantedCommitteePubKey) {
		return fmt.Errorf("%w: distributed public key for committee %d is invalid, wanted %s, got %s", ErrDistributedPubKeyMismatch, state.Committee().InstanceID, wantedCommitteePubKey, cb.Dpk)
	}

	if cb.Round <= state.Randomness().Round {
		return fmt.Errorf("%w: collective beacon event round is %d, but current state is %d", ErrInvalidRound, cb.Round, state.Randomness().Round)
	}

	if cb.InstanceID != state.Committee().InstanceID {
		return fmt.Errorf("%w: wanted %d instance ID but got %d from collective beacon event", ErrInstanceIDMismatch, state.Committee().InstanceID, cb.InstanceID)
	}

	if err := verifySignature(cb); err != nil {
		return err
	}

	return nil
}

// verifyIssuer checks the given issuer is a member of the committee.
func verifyIssuer(state *State, issuer ed25519.PublicKey) error {
	for _, member := range state.Committee().Identities {
		if member == issuer {
			return nil
		}
	}
	return fmt.Errorf("%w: issuer %s not found in committee %d", ErrInvalidIssuer, issuer.String(), state.Committee().InstanceID)
}

// verifySignature checks the current signature against the distributed public key.
func verifySignature(cb *CollectiveBeaconEvent) error {
	dpk := key.KeyGroup.Point()
	if err := dpk.UnmarshalBinary(cb.Dpk); err != nil {
		return err
	}

	msg := chain.Message(cb.Round, cb.PrevSignature)

	if err := key.Scheme.VerifyRecovered(dpk, msg, cb.Signature); err != nil {
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
