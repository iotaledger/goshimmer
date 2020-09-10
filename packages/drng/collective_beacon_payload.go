package drng

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
)

// CollectiveBeaconPayload is a collective beacon payload.
type CollectiveBeaconPayload struct {
	Header

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

// NewCollectiveBeaconPayload creates a new collective beacon payload.
func NewCollectiveBeaconPayload(instanceID uint32, round uint64, prevSignature, signature, dpk []byte) *CollectiveBeaconPayload {
	return &CollectiveBeaconPayload{
		Header:        NewHeader(TypeCollectiveBeacon, instanceID),
		Round:         round,
		PrevSignature: prevSignature,
		Signature:     signature,
		Dpk:           dpk,
	}
}

// ParseCollectiveBeaconPayload is a wrapper for simplified unmarshaling in a byte stream using the marshalUtil package.
func ParseCollectiveBeaconPayload(marshalUtil *marshalutil.MarshalUtil) (*CollectiveBeaconPayload, error) {
	unmarshalledPayload, err := marshalUtil.Parse(func(data []byte) (interface{}, int, error) { return CollectiveBeaconPayloadFromBytes(data) })
	if err != nil {
		return nil, err
	}
	_payload := unmarshalledPayload.(*CollectiveBeaconPayload)

	return _payload, nil
}

// CollectiveBeaconPayloadFromBytes parses the marshaled version of a Payload into an object.
// It either returns a new Payload or fills an optionally provided Payload with the parsed information.
func CollectiveBeaconPayloadFromBytes(bytes []byte, optionalTargetObject ...*CollectiveBeaconPayload) (result *CollectiveBeaconPayload, consumedBytes int, err error) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &CollectiveBeaconPayload{}
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
	if result.Header, err = ParseHeader(marshalUtil); err != nil {
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

// Bytes returns the collective beacon payload bytes.
func (p *CollectiveBeaconPayload) Bytes() (bytes []byte) {
	// acquire lock for reading bytes
	p.bytesMutex.RLock()

	// return if bytes have been determined already
	if bytes = p.bytes; bytes != nil {
		p.bytesMutex.RUnlock()
		return
	}

	// switch to write lock
	p.bytesMutex.RUnlock()
	p.bytesMutex.Lock()
	defer p.bytesMutex.Unlock()

	// return if bytes have been determined in the mean time
	if bytes = p.bytes; bytes != nil {
		return
	}

	// marshal fields
	payloadLength := HeaderLength + marshalutil.UINT64_SIZE + SignatureSize*2 + PublicKeySize
	marshalUtil := marshalutil.New(marshalutil.UINT32_SIZE + marshalutil.UINT32_SIZE + payloadLength)
	marshalUtil.WriteUint32(PayloadType)
	marshalUtil.WriteUint32(uint32(payloadLength))
	marshalUtil.WriteBytes(p.Header.Bytes())
	marshalUtil.WriteUint64(p.Round)
	marshalUtil.WriteBytes(p.PrevSignature)
	marshalUtil.WriteBytes(p.Signature)
	marshalUtil.WriteBytes(p.Dpk)

	bytes = marshalUtil.Bytes()

	// store result
	p.bytes = bytes

	return
}

func (p *CollectiveBeaconPayload) String() string {
	return stringify.Struct("CollectiveBeaconPayload",
		stringify.StructField("type", uint64(p.Header.PayloadType)),
		stringify.StructField("instance", uint64(p.Header.InstanceID)),
		stringify.StructField("round", p.Round),
		stringify.StructField("prevSignature", p.PrevSignature),
		stringify.StructField("signature", p.Signature),
		stringify.StructField("distributedPK", p.Dpk),
	)
}

// region Payload implementation ///////////////////////////////////////////////////////////////////////////////////////

// Type returns the collective beacon payload type.
func (p *CollectiveBeaconPayload) Type() payload.Type {
	return PayloadType
}

// Marshal marshals the collective beacon payload into bytes.
func (p *CollectiveBeaconPayload) Marshal() (bytes []byte, err error) {
	return p.Bytes(), nil
}

// Unmarshal returns a collective beacon payload from the given bytes.
func (p *CollectiveBeaconPayload) Unmarshal(data []byte) (err error) {
	_, _, err = CollectiveBeaconPayloadFromBytes(data, p)

	return
}

// // endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
