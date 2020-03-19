package drng

import (
	"crypto/sha512"
	"errors"

	"github.com/drand/drand/beacon"
	"github.com/drand/drand/key"
	"github.com/iotaledger/goshimmer/packages/binary/drng/payload/collectiveBeacon"
)

// VerifyCollectiveBeacon checks the current signature against the distributed public key
func VerifyCollectiveBeacon(data *collectiveBeacon.Payload) error {
	if data == nil {
		return errors.New("nil data")
	}

	dpk := key.KeyGroup.Point()
	if err := dpk.UnmarshalBinary(data.DistributedPK()); err != nil {
		return err
	}

	msg := beacon.Message(data.PrevSignature(), data.Round())

	if err := key.Scheme.VerifyRecovered(dpk, msg, data.Signature()); err != nil {
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

	//return hash, nil
	return hash.Sum(nil), nil
}
