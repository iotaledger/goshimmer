package ed25119

import (
	"errors"
	"fmt"
)

type Signature [SignatureSize]byte

func SignatureFromBytes(bytes []byte) (result Signature, err error, consumedBytes int) {
	if len(bytes) < SignatureSize {
		err = fmt.Errorf("bytes too short")

		return
	}

	copy(result[:SignatureSize], bytes)

	consumedBytes = SignatureSize

	return
}

func (signature *Signature) UnmarshalBinary(bytes []byte) (err error) {
	if len(bytes) < SignatureSize {
		return errors.New("not enough bytes")
	}

	copy(signature[:], bytes[:])

	return
}

const SignatureSize = 64
