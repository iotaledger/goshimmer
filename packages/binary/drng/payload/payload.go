package payload

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/binary/drng/payload/header"
	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction/payload"
	"github.com/iotaledger/hive.go/stringify"
)

type Payload struct {
	header header.Header
	data   []byte

	bytes      []byte
	bytesMutex sync.RWMutex
}

func New(header header.Header, data []byte) *Payload {
	return &Payload{
		header: header,
		data:   data,
	}
}

func (p *Payload) SubType() header.Type {
	return p.header.PayloadType()
}

func (payload *Payload) Instance() uint32 {
	return payload.header.Instance()
}

func (payload *Payload) Data() []byte {
	return payload.data
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

	len, err := marshalUtil.ReadUint32()
	if err != nil {
		return
	}

	// parse header
	if result.header, err = header.Parse(marshalUtil); err != nil {
		return
	}

	// parse data
	if result.data, err = marshalUtil.ReadBytes(int(len - header.Length)); err != nil {
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
	// initialize helper
	marshalUtil := marshalutil.New()

	// marshal the payload specific information
	marshalUtil.WriteUint32(Type)
	marshalUtil.WriteUint32(uint32(len(payload.data) + header.Length))
	marshalUtil.WriteBytes(payload.header.Bytes())
	marshalUtil.WriteBytes(payload.data[:])

	bytes = marshalUtil.Bytes()

	return
}

func (payload *Payload) String() string {
	return stringify.Struct("Payload",
		stringify.StructField("type", uint64(payload.SubType())),
		stringify.StructField("instance", uint64(payload.Instance())),
		stringify.StructField("data", payload.Data()),
	)
}

// region Payload implementation ///////////////////////////////////////////////////////////////////////////////////////

var Type = payload.Type(111)

func (payload *Payload) GetType() payload.Type {
	return Type
}

func (payload *Payload) MarshalBinary() (bytes []byte, err error) {
	return payload.Bytes(), nil
}

func (payload *Payload) UnmarshalBinary(data []byte) (err error) {
	_, err, _ = FromBytes(data, payload)

	return
}

func init() {
	payload.RegisterType(Type, func(data []byte) (payload payload.Payload, err error) {
		payload = &Payload{}
		err = payload.UnmarshalBinary(data)

		return
	})
}

// define contract (ensure that the struct fulfills the corresponding interface)
var _ payload.Payload = &Payload{}

// // endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
