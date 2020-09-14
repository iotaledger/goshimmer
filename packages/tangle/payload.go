package tangle

import (
	"fmt"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/mr-tron/base58"
)

const (
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
	// PayloadType returns the type of the payload.
	Type() PayloadType
	// Bytes returns the payload bytes.
	Bytes() []byte
	// String returns a human-friendly representation of the payload.
	String() string
}

// PayloadFromBytes unmarshals bytes into a payload.
func PayloadFromBytes(bytes []byte) (result Payload, consumedBytes int, err error) {
	// initialize helper
	marshalUtil := marshalutil.New(bytes)

	payloadSize, err := marshalUtil.ReadUint32()
	if err != nil {
		err = fmt.Errorf("failed to unmarshal payload size from bytes: %w", err)
		return
	}

	if payloadSize > MaxPayloadSize {
		err = fmt.Errorf("%w: %d", ErrMaxPayloadSizeExceeded, payloadSize)
		return
	}

	// calculate result
	payloadType, err := marshalUtil.ReadUint32()
	if err != nil {
		err = fmt.Errorf("failed to unmarshal payload type from bytes: %w", err)
		return
	}

	marshalUtil.ReadSeek(marshalUtil.ReadOffset() - marshalutil.UINT32_SIZE*2)
	payloadBytes, err := marshalUtil.ReadBytes(int(payloadSize) + 8)
	if err != nil {
		err = fmt.Errorf("failed to unmarshal payload bytes from bytes: %w", err)
		return
	}

	readOffset := marshalUtil.ReadOffset()
	result, err = GetUnmarshaler(payloadType)(payloadBytes)
	if err != nil {
		// fallback to the generic unmarshaler if registered type fails to unmarshal
		marshalUtil.ReadSeek(readOffset)
		result, err = GenericPayloadUnmarshaler(payloadBytes)
		if err != nil {
			err = fmt.Errorf("failed to unmarshal payload from bytes with generic unmarshaler: %w", err)
			return
		}
	}

	// return the number of bytes we processed
	consumedBytes = marshalUtil.ReadOffset()
	return
}

// PayloadParse parses a payload by using the given marshal util.
func PayloadParse(marshalUtil *marshalutil.MarshalUtil) (Payload, error) {
	payload, err := marshalUtil.Parse(func(data []byte) (interface{}, int, error) { return PayloadFromBytes(data) })
	if err != nil {
		err = fmt.Errorf("failed to parse payload: %w", err)
		return nil, err
	}
	return payload.(Payload), nil
}
