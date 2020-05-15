package payload

import (
	"sync"

	"github.com/iotaledger/hive.go/stringify"

	drngPayload "github.com/iotaledger/goshimmer/packages/binary/drng/payload"
	"github.com/iotaledger/goshimmer/packages/binary/drng/payload/header"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
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
	if payload, err := marshalUtil.Parse(func(data []byte) (interface{}, int, error) { return FromBytes(data) }); err != nil {
		return &Payload{}, err
	} else {
		return payload.(*Payload), nil
	}
}

// FromBytes parses the marshaled version of a Payload into an object.
// It either returns a new Payload or fills an optionally provided Payload with the parsed information.
func FromBytes(bytes []byte, optionalTargetObject ...*Payload) (result *Payload, consumedBytes int, err error) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &Payload{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to OutputFromBytes")
	}

	// initialize helper
	marshalUtil := marshalutil.New(bytes)

	// read information that are required to identify the payload from the outside
	if _, err = marshalUtil.ReadUint32(); err != nil {
		return
	}
	if _, err = marshalUtil.ReadUint32(); err != nil {
		return
	}

	// parse header
	if result.Header, err = header.Parse(marshalUtil); err != nil {
		return
	}

	// parse round
	if result.Round, err = marshalUtil.ReadUint64(); err != nil {
		return
	}

	// parse prevSignature
	if result.PrevSignature, err = marshalUtil.ReadBytes(SignatureSize); err != nil {
		return
	}

	// parse current signature
	if result.Signature, err = marshalUtil.ReadBytes(SignatureSize); err != nil {
		return
	}

	// parse distributed public key
	if result.Dpk, err = marshalUtil.ReadBytes(PublicKeySize); err != nil {
		return
	}

	// return the number of bytes we processed
	consumedBytes = marshalUtil.ReadOffset()

	// store bytes, so we don't have to marshal manually
	result.bytes = bytes[:consumedBytes]

	return
}

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
	marshalUtil.WriteUint32(drngPayload.Type)
	marshalUtil.WriteUint32(uint32(payloadLength))
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

func (payload *Payload) Type() payload.Type {
	return drngPayload.Type
}

func (payload *Payload) Marshal() (bytes []byte, err error) {
	return payload.Bytes(), nil
}

func (payload *Payload) Unmarshal(data []byte) (err error) {
	_, _, err = FromBytes(data, payload)

	return
}

// // endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
