package payload

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/serix"
)

// MaxPayloadSize = MaxMessageSize -
//                    (version(1) + parentsBlocksCount(1) + 4 * (parentsType(1) + parentsCount(1) + 8 * reference(32)) +
//		      issuerPK(32) + issuanceTime(8) + seqNum(8) + payloadLength(4) + nonce(8) + signature(64)
//		      = MaxMessageSize - 1158 bytes = 64378
const MaxSize = 64378

// Payload represents the generic interface for an object that can be embedded in Messages of the Tangle.
type Payload interface {
	// Type returns the Type of the Payload.
	Type() Type

	// Bytes returns a marshaled version of the Payload.
	Bytes() []byte

	// String returns a human readable version of the Payload.
	String() string
}

// FromBytes unmarshals a Payload from a sequence of bytes.
func FromBytes(data []byte) (payloadDecoded Payload, consumedBytes int, err error) {
	payloadDecoded = Payload(nil)

	consumedBytes, err = serix.DefaultAPI.Decode(context.Background(), data, payloadDecoded, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse Chat Payload: %w", err)
		return
	}

	return
}
