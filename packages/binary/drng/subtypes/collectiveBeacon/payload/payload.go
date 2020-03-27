package payload

import (
	"sync"

	"github.com/iotaledger/hive.go/stringify"

	drngPayload "github.com/iotaledger/goshimmer/packages/binary/drng/payload"
	"github.com/iotaledger/goshimmer/packages/binary/drng/payload/header"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message/payload"
	"github.com/iotaledger/hive.go/marshalutil"
)

type Payload struct {
	//objectstorage.StorableObjectFlags

	header header.Header

	round         uint64 // round of the current beacon
	prevSignature []byte // collective signature of the previous beacon
	signature     []byte // collective signature of the current beacon
	dpk           []byte // distributed public key
	bytes         []byte
	bytesMutex    sync.RWMutex
}

func New(instanceID uint32, round uint64, prevSignature, signature, dpk []byte) *Payload {
	return &Payload{
		header:        header.New(header.CollectiveBeaconType(), instanceID),
		round:         round,
		prevSignature: prevSignature,
		signature:     signature,
		dpk:           dpk,
	}
}

func (p *Payload) SubType() header.Type {
	return p.header.PayloadType()
}

func (payload *Payload) Instance() uint32 {
	return payload.header.Instance()
}

func (payload *Payload) Round() uint64 {
	return payload.round
}

func (payload *Payload) PrevSignature() []byte {
	return payload.prevSignature
}

func (payload *Payload) Signature() []byte {
	return payload.signature
}

func (payload *Payload) DistributedPK() []byte {
	return payload.dpk
}

// Parse is a wrapper for simplified unmarshaling in a byte stream using the marshalUtil package.
func Parse(marshalUtil *marshalutil.MarshalUtil) (*Payload, error) {
	if payload, err := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return FromBytes(data) }); err != nil {
		return &Payload{}, err
	} else {
		return payload.(*Payload), nil
	}
}

// FromBytes parses the marshaled version of a Payload into an object.
// It either returns a new Payload or fills an optionally provided Payload with the parsed information.
func FromBytes(bytes []byte, optionalTargetObject ...*Payload) (result *Payload, err error, consumedBytes int) {
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
	if result.header, err = header.Parse(marshalUtil); err != nil {
		return
	}

	// parse round
	if result.round, err = marshalUtil.ReadUint64(); err != nil {
		return
	}

	// parse prevSignature
	if result.prevSignature, err = marshalUtil.ReadBytes(SignatureSize); err != nil {
		return
	}

	// parse current signature
	if result.signature, err = marshalUtil.ReadBytes(SignatureSize); err != nil {
		return
	}

	// parse distributed public key
	if result.dpk, err = marshalUtil.ReadBytes(PublicKeySize); err != nil {
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
		defer payload.bytesMutex.RUnlock()
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
	marshalUtil.WriteBytes(payload.header.Bytes())
	marshalUtil.WriteUint64(payload.Round())
	marshalUtil.WriteBytes(payload.PrevSignature())
	marshalUtil.WriteBytes(payload.Signature())
	marshalUtil.WriteBytes(payload.DistributedPK())

	bytes = marshalUtil.Bytes()

	// store result
	payload.bytes = bytes

	return
}

func (payload *Payload) String() string {
	return stringify.Struct("Payload",
		stringify.StructField("type", uint64(payload.SubType())),
		stringify.StructField("instance", uint64(payload.Instance())),
		stringify.StructField("round", payload.Round()),
		stringify.StructField("prevSignature", payload.PrevSignature()),
		stringify.StructField("signature", payload.Signature()),
		stringify.StructField("distributedPK", payload.DistributedPK()),
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
	_, err, _ = FromBytes(data, payload)

	return
}

// func init() {
// 	payload.RegisterType(drngPayload.Type, func(data []byte) (payload payload.Payload, err error) {
// 		payload = &Payload{}
// 		err = payload.UnmarshalBinary(data)

// 		return
// 	})
// }

// define contract (ensure that the struct fulfills the corresponding interface)
var _ payload.Payload = &Payload{}

// // endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
