package tangle

import (
	"fmt"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/mr-tron/base58"
)

const (
	// MaxMessageSize defines the maximum size of a message.
	MaxMessageSize = 64 * 1024

	// MaxPayloadSize defines the maximum size of a payload.
	// parent1ID + parent2ID + issuerPublicKey + issuingTime + sequenceNumber + nonce + signature
	MaxPayloadSize = MaxMessageSize - 64 - 64 - 32 - 8 - 8 - 8 - 64
)

var (
	// ErrMaxPayloadSizeExceeded is returned if the maximum payload size is exceeded.
	ErrMaxPayloadSizeExceeded = fmt.Errorf("maximum payload size of %d bytes exceeded", MaxPayloadSize)
)

// PayloadID represents the id of a data payload.
type PayloadID [PayloadIDLength]byte

// Bytes returns the id as a byte slice backed by the original array,
// therefore it should not be modified.
func (id PayloadID) Bytes() []byte {
	return id[:]
}

func (id PayloadID) String() string {
	return base58.Encode(id[:])
}

// PayloadIDLength is the length of a data payload id.
const PayloadIDLength = 64

// Payload represents some kind of payload of data which only gains meaning by having
// corresponding node logic processing payloads of a given type.
type Payload interface {
	// Type returns the type of the payload.
	Type() Type
	// Bytes returns the payload bytes.
	Bytes() []byte
	// Unmarshal unmarshals the payload from the given bytes.
	Unmarshal(bytes []byte) error
	// String returns a human-friendly representation of the payload.
	String() string
}

// PayloadFromBytes unmarshals bytes into a payload.
func PayloadFromBytes(bytes []byte) (result Payload, consumedBytes int, err error) {
	// initialize helper
	marshalUtil := marshalutil.New(bytes)

	// calculate result
	payloadType, err := marshalUtil.ReadUint32()
	if err != nil {
		return
	}

	payloadSize, err := marshalUtil.ReadUint32()
	if err != nil {
		return
	}

	if payloadSize > MaxPayloadSize {
		err = fmt.Errorf("%w: %d", ErrMaxPayloadSizeExceeded, payloadSize)
		return
	}

	marshalUtil.ReadSeek(marshalUtil.ReadOffset() - marshalutil.UINT32_SIZE*2)
	payloadBytes, err := marshalUtil.ReadBytes(int(payloadSize) + 8)
	if err != nil {
		return
	}

	readOffset := marshalUtil.ReadOffset()
	result, err = GetUnmarshaler(payloadType)(payloadBytes)
	if err != nil {
		// fallback to the generic unmarshaler if registered type fails to unmarshal
		marshalUtil.ReadSeek(readOffset)
		result, err = GenericPayloadUnmarshalerFactory(payloadType)(payloadBytes)
		if err != nil {
			return
		}
	}

	// return the number of bytes we processed
	consumedBytes = marshalUtil.ReadOffset()
	return
}

// ParsePayload parses a payload by using the given marshal util.
func ParsePayload(marshalUtil *marshalutil.MarshalUtil) (Payload, error) {
	payload, err := marshalUtil.Parse(func(data []byte) (interface{}, int, error) { return PayloadFromBytes(data) })
	if err != nil {
		return nil, err
	}
	return payload.(Payload), nil
}
