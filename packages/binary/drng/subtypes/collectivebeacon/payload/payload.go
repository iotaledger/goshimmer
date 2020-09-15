package payload

import (
	"fmt"
	"sync"

	"github.com/iotaledger/hive.go/stringify"

	drngPayload "github.com/iotaledger/goshimmer/packages/binary/drng/payload"
	"github.com/iotaledger/goshimmer/packages/binary/drng/payload/header"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/hive.go/marshalutil"
)

// Payload is a collective beacon payload.
type Payload struct {
	header.Header

	// Round of the current beacon
	Round uint64
	// Collective signature of the previous beacon
	PrevSignature []byte
	// Collective signature of the current beacon
	Signature []byte
	// The distributed public key
	Dpk []byte

	bytes      []byte
	bytesMutex sync.RWMutex
}

// New creates a new collective beacon payload.
func New(instanceID uint32, round uint64, prevSignature, signature, dpk []byte) *Payload {
	return &Payload{
		Header:        header.New(header.TypeCollectiveBeacon, instanceID),
		Round:         round,
		PrevSignature: prevSignature,
		Signature:     signature,
		Dpk:           dpk,
	}
}

// Parse is a wrapper for simplified unmarshaling in a byte stream using the marshalUtil package.
func Parse(marshalUtil *marshalutil.MarshalUtil) (*Payload, error) {
	unmarshalledPayload, err := marshalUtil.Parse(func(data []byte) (interface{}, int, error) { return FromBytes(data) })
	if err != nil {
		err = fmt.Errorf("failed to parse collective beacon payload: %w", err)
		return nil, err
	}
	_payload := unmarshalledPayload.(*Payload)

	return _payload, nil
}

// FromBytes parses the marshaled version of a Payload into an object.
// It either returns a new Payload or fills an optionally provided Payload with the parsed information.
func FromBytes(bytes []byte) (result *Payload, consumedBytes int, err error) {
	// initialize helper
	marshalUtil := marshalutil.New(bytes)

	// read information that are required to identify the payload from the outside
	if _, err = marshalUtil.ReadUint32(); err != nil {
		err = fmt.Errorf("failed to parse payload size of collective beacon payload: %w", err)
		return
	}
	if _, err = marshalUtil.ReadUint32(); err != nil {
		err = fmt.Errorf("failed to parse payload type of collective beacon payload: %w", err)
		return
	}

	// parse header
	result = &Payload{}
	if result.Header, err = header.Parse(marshalUtil); err != nil {
		err = fmt.Errorf("failed to parse header of collective beacon payload: %w", err)
		return
	}

	// parse round
	if result.Round, err = marshalUtil.ReadUint64(); err != nil {
		err = fmt.Errorf("failed to parse round of collective beacon payload: %w", err)
		return
	}

	// parse prevSignature
	if result.PrevSignature, err = marshalUtil.ReadBytes(SignatureSize); err != nil {
		err = fmt.Errorf("failed to parse prevSignature of collective beacon payload: %w", err)
		return
	}

	// parse current signature
	if result.Signature, err = marshalUtil.ReadBytes(SignatureSize); err != nil {
		err = fmt.Errorf("failed to parse current signature of collective beacon payload: %w", err)
		return
	}

	// parse distributed public key
	if result.Dpk, err = marshalUtil.ReadBytes(PublicKeySize); err != nil {
		err = fmt.Errorf("failed to parse distributed public key of collective beacon payload: %w", err)
		return
	}

	// return the number of bytes we processed
	consumedBytes = marshalUtil.ReadOffset()

	// store bytes, so we don't have to marshal manually
	result.bytes = bytes[:consumedBytes]

	return
}

// Bytes returns the collective beacon payload bytes.
func (payload *Payload) Bytes() (bytes []byte) {
	// acquire lock for reading bytes
	payload.bytesMutex.RLock()

	// return if bytes have been determined already
	if bytes = payload.bytes; bytes != nil {
		payload.bytesMutex.RUnlock()
		return
	}

	// switch to write lock
	payload.bytesMutex.RUnlock()
	payload.bytesMutex.Lock()
	defer payload.bytesMutex.Unlock()

	// return if bytes have been determined in the mean time
	if bytes = payload.bytes; bytes != nil {
		return
	}

	// marshal fields
	payloadLength := header.Length + marshalutil.UINT64_SIZE + SignatureSize*2 + PublicKeySize
	marshalUtil := marshalutil.New(marshalutil.UINT32_SIZE + marshalutil.UINT32_SIZE + payloadLength)
	marshalUtil.WriteUint32(uint32(payloadLength))
	marshalUtil.WriteUint32(drngPayload.Type)
	marshalUtil.WriteBytes(payload.Header.Bytes())
	marshalUtil.WriteUint64(payload.Round)
	marshalUtil.WriteBytes(payload.PrevSignature)
	marshalUtil.WriteBytes(payload.Signature)
	marshalUtil.WriteBytes(payload.Dpk)

	bytes = marshalUtil.Bytes()

	// store result
	payload.bytes = bytes

	return
}

func (payload *Payload) String() string {
	return stringify.Struct("Payload",
		stringify.StructField("type", uint64(payload.Header.PayloadType)),
		stringify.StructField("instance", uint64(payload.Header.InstanceID)),
		stringify.StructField("round", payload.Round),
		stringify.StructField("prevSignature", payload.PrevSignature),
		stringify.StructField("signature", payload.Signature),
		stringify.StructField("distributedPK", payload.Dpk),
	)
}

// region Payload implementation ///////////////////////////////////////////////////////////////////////////////////////

// Type returns the collective beacon payload type.
func (payload *Payload) Type() tangle.PayloadType {
	return drngPayload.Type
}

// Marshal marshals the collective beacon payload into bytes.
func (payload *Payload) Marshal() (bytes []byte, err error) {
	return payload.Bytes(), nil
}

// // endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
