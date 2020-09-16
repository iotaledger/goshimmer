package tangle

import (
	"fmt"
	"sync"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/mr-tron/base58"
)

const (
	// MaxPayloadSize defines the maximum size of a payload.
	// parent1ID + parent2ID + issuerPublicKey + issuingTime + sequenceNumber + nonce + signature
	MaxPayloadSize = MaxMessageSize - 64 - 64 - 32 - 8 - 8 - 8 - 64

	// PayloadIDLength is the length of a data payload id.
	PayloadIDLength = 64

	// ObjectName defines the name of the data object.
	ObjectName = "data"
)

var (
	// ErrMaxPayloadSizeExceeded is returned if the maximum payload size is exceeded.
	ErrMaxPayloadSizeExceeded = fmt.Errorf("maximum payload size of %d bytes exceeded", MaxPayloadSize)

	typeRegister       = make(map[PayloadType]Definition)
	typeRegisterMutex  sync.RWMutex
	genericUnmarshaler Unmarshaler
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

// PayloadType represents the type id of a payload.
type PayloadType = uint32

// Unmarshaler takes some data and unmarshals it into a payload.
type Unmarshaler func(data []byte) (Payload, error)

// Definition defines the properties of a payload type.
type Definition struct {
	Name string
	Unmarshaler
}

func init() {
	// register the generic unmarshaler
	SetGenericUnmarshaler(GenericPayloadUnmarshaler)
}

// RegisterPayloadType registers a payload type with the given unmarshaler.
func RegisterPayloadType(payloadType PayloadType, payloadName string, unmarshaler Unmarshaler) {
	typeRegisterMutex.Lock()
	typeRegister[payloadType] = Definition{
		Name:        payloadName,
		Unmarshaler: unmarshaler,
	}
	typeRegisterMutex.Unlock()
}

// GetUnmarshaler returns the unmarshaler for the given type if known or
// the generic unmarshaler if the given payload type has no associated unmarshaler.
func GetUnmarshaler(payloadType PayloadType) Unmarshaler {
	typeRegisterMutex.RLock()
	defer typeRegisterMutex.RUnlock()

	if definition, exists := typeRegister[payloadType]; exists {
		return definition.Unmarshaler
	}

	return genericUnmarshaler
}

// SetGenericUnmarshaler sets the generic unmarshaler.
func SetGenericUnmarshaler(unmarshaler Unmarshaler) {
	genericUnmarshaler = unmarshaler
}

// Name returns the name of a given payload type.
func Name(payloadType PayloadType) string {
	typeRegisterMutex.RLock()
	defer typeRegisterMutex.RUnlock()
	if definition, exists := typeRegister[payloadType]; exists {
		return definition.Name
	}
	return ObjectName
}

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

// PayloadFromMarshalUtil parses a payload by using the given marshal util.
func PayloadFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (Payload, error) {
	payload, err := marshalUtil.Parse(func(data []byte) (interface{}, int, error) { return PayloadFromBytes(data) })
	if err != nil {
		err = fmt.Errorf("failed to parse payload: %w", err)
		return nil, err
	}
	return payload.(Payload), nil
}

// DataType is the message type of a data payload.
var DataType = PayloadType(0)

func init() {
	// register the generic data payload type
	RegisterPayloadType(DataType, ObjectName, GenericPayloadUnmarshaler)
}

// DataPayload represents a payload which just contains a blob of data.
type DataPayload struct {
	payloadType PayloadType
	data        []byte
}

// NewDataPayload creates new data payload.
func NewDataPayload(data []byte) *DataPayload {
	return &DataPayload{
		payloadType: DataType,
		data:        data,
	}
}

// DataPayloadFromBytes creates a new data payload from the given bytes.
func DataPayloadFromBytes(bytes []byte) (result *DataPayload, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = DataPayloadFromMarshalUtil(marshalUtil)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// DataPayloadFromMarshalUtil parses a new data payload out of the given marshal util.
func DataPayloadFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (result *DataPayload, err error) {
	// parse information
	result = &DataPayload{}
	payloadBytes, err := marshalUtil.ReadUint32()
	if err != nil {
		err = fmt.Errorf("failed to parse data payload size: %w", err)
		return
	}
	result.payloadType, err = marshalUtil.ReadUint32()
	if err != nil {
		err = fmt.Errorf("failed to parse data payload type: %w", err)
		return
	}
	result.data, err = marshalUtil.ReadBytes(int(payloadBytes))
	if err != nil {
		err = fmt.Errorf("failed to parse data payload contest: %w", err)
		return
	}

	return
}

// Type returns the payload type.
func (d *DataPayload) Type() PayloadType {
	return d.payloadType
}

// Data returns the data of the data payload.
func (d *DataPayload) Data() []byte {
	return d.data
}

// Bytes marshals the data payload into a sequence of bytes.
func (d *DataPayload) Bytes() []byte {
	// initialize helper
	marshalUtil := marshalutil.New()

	// marshal the payload specific information
	marshalUtil.WriteUint32(uint32(len(d.data)))
	marshalUtil.WriteUint32(d.Type())
	marshalUtil.WriteBytes(d.data[:])

	// return result
	return marshalUtil.Bytes()
}

func (d *DataPayload) String() string {
	return stringify.Struct("Data",
		stringify.StructField("type", int(d.Type())),
		stringify.StructField("data", string(d.Data())),
	)
}

// GenericPayloadUnmarshaler is an unmarshaler for the generic data payload type.
func GenericPayloadUnmarshaler(data []byte) (payload Payload, err error) {
	payload, _, err = DataPayloadFromBytes(data)

	return
}
