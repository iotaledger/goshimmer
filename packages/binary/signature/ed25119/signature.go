package ed25119

import (
	"errors"
)

type Signature [SignatureSize]byte

func (signature *Signature) UnmarshalBinary(bytes []byte) (err error) {
	if len(bytes) < SignatureSize {
		return errors.New("not enough bytes")
	}

	copy(signature[:], bytes[:])

	return
}

const SignatureSize = 64
