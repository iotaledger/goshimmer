package payload

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"golang.org/x/xerrors"
)

// MaxSize defines the maximum allowed size of a marshaled Payload (in bytes).
const MaxSize = 65288

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
func FromBytes(payloadBytes []byte) (payload Payload, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(payloadBytes)
	if payload, err = FromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Payload from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// FromMarshalUtil unmarshals a Payload using a MarshalUtil (for easier unmarshaling).
func FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (payload Payload, err error) {
	payloadSize, err := marshalUtil.ReadUint32()
	if err != nil {
		err = xerrors.Errorf("failed to parse payload size (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if payloadSize == 0 {
		return
	}
	if payloadSize > MaxSize {
		err = xerrors.Errorf("maximum payload size of %d bytes exceeded: %w", MaxSize, cerrors.ErrParseBytesFailed)
		return
	}
	// a payloadSize of 0 indicates the payload is omitted and the payload is nil
	if payloadSize == 0 {
		return
	}

	payloadType, err := TypeFromMarshalUtil(marshalUtil)
	if err != nil {
		err = xerrors.Errorf("failed to unmarshal Type from MarshalUtil: %w", err)
		return
	}

	marshalUtil.ReadSeek(-marshalutil.UINT32_SIZE * 2)
	payloadBytes, err := marshalUtil.ReadBytes(int(payloadSize) + 8)
	if err != nil {
		err = xerrors.Errorf("failed to unmarshal payload bytes (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	readOffset := marshalUtil.ReadOffset()
	if payload, err = Unmarshaler(payloadType)(payloadBytes); err != nil {
		marshalUtil.ReadSeek(readOffset)

		if payload, err = GenericDataPayloadUnmarshaler(payloadBytes); err != nil {
			err = fmt.Errorf("failed to parse Payload with generic GenericDataPayloadUnmarshaler: %w", err)
			return
		}
	}

	return
}
