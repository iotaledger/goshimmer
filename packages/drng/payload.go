package drng

import (
	"fmt"
	"sync"

	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
)

const (
	// ObjectName defines the name of the dRNG object.
	ObjectName = "dRNG"
)

// Payload defines a DRNG payload.
type Payload struct {
	Header
	Data []byte

	bytes      []byte
	bytesMutex sync.RWMutex
}

// NewPayload creates a new DRNG payload.
func NewPayload(header Header, data []byte) *Payload {
	return &Payload{
		Header: header,
		Data:   data,
	}
}

// PayloadFromMarshalUtil is a wrapper for simplified unmarshaling in a byte stream using the marshalUtil package.
func PayloadFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (*Payload, error) {
	payload, err := marshalUtil.Parse(func(data []byte) (interface{}, int, error) { return FromBytes(data) })
	if err != nil {
		err = fmt.Errorf("failed to parse drng payload: %w", err)
		return &Payload{}, err
	}
	return payload.(*Payload), nil
}

// FromBytes parses the marshaled version of a Payload into an object.
// It either returns a new Payload or fills an optionally provided Payload with the parsed information.
func FromBytes(bytes []byte) (result *Payload, consumedBytes int, err error) {
	// initialize helper
	marshalUtil := marshalutil.New(bytes)

	// read information that are required to identify the payload from the outside
	result = &Payload{}
	len, err := marshalUtil.ReadUint32()
	if err != nil {
		err = fmt.Errorf("failed to parse payload size of drng payload: %w", err)
		return
	}

	if _, err = marshalUtil.ReadUint32(); err != nil {
		err = fmt.Errorf("failed to parse payload type of drng payload: %w", err)
		return
	}

	// parse header
	if result.Header, err = HeaderFromMarshalUtil(marshalUtil); err != nil {
		err = fmt.Errorf("failed to parse header of drng payload: %w", err)
		return
	}

	// parse data
	if result.Data, err = marshalUtil.ReadBytes(int(len - payload.TypeLength - HeaderLength)); err != nil {
		err = fmt.Errorf("failed to parse data of drng payload: %w", err)
		return
	}

	// return the number of bytes we processed
	consumedBytes = marshalUtil.ReadOffset()

	// store bytes, so we don't have to marshal manually
	result.bytes = bytes[:consumedBytes]

	return
}

// Bytes returns the drng payload bytes.
func (p *Payload) Bytes() (bytes []byte) {
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
	// initialize helper
	marshalUtil := marshalutil.New()

	// marshal the payload specific information
	marshalUtil.WriteUint32(payload.TypeLength + uint32(len(p.Data)+HeaderLength))
	marshalUtil.WriteBytes(PayloadType.Bytes())
	marshalUtil.WriteBytes(p.Header.Bytes())
	marshalUtil.WriteBytes(p.Data[:])

	bytes = marshalUtil.Bytes()

	return
}

func (p *Payload) String() string {
	return stringify.Struct("Payload",
		stringify.StructField("type", uint64(p.Header.PayloadType)),
		stringify.StructField("instance", uint64(p.Header.InstanceID)),
		stringify.StructField("data", p.Data),
	)
}

// region Payload implementation ///////////////////////////////////////////////////////////////////////////////////////

// PayloadType defines the type of the drng payload.
var PayloadType = payload.NewType(111, ObjectName, func(data []byte) (payload payload.Payload, err error) {
	payload, _, err = FromBytes(data)

	return
})

// Type returns the type of the drng payload.
func (p *Payload) Type() payload.Type {
	return PayloadType
}

// Marshal marshals the drng payload into bytes.
func (p *Payload) Marshal() (bytes []byte, err error) {
	return p.Bytes(), nil
}

// // endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
