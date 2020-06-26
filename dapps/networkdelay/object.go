package networkdelay

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/mr-tron/base58"
)

const (
	// ObjectName defines the name of the networkdelay object.
	ObjectName = "networkdelay"
)

// ID represents a 32 byte ID of a network delay object.
type ID [32]byte

// String returns a human-friendly representation of the ID.
func (id ID) String() string {
	return base58.Encode(id[:])
}

// Object represents the network delay object type.
type Object struct {
	id       ID
	sentTime int64

	bytes      []byte
	bytesMutex sync.RWMutex
}

// NewObject creates a new  network delay object.
func NewObject(id ID, sentTime int64) *Object {
	return &Object{
		id:       id,
		sentTime: sentTime,
	}
}

// FromBytes parses the marshaled version of an Object into a Go object.
// It either returns a new Object or fills an optionally provided Object with the parsed information.
func FromBytes(bytes []byte, optionalTargetObject ...*Object) (result *Object, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = Parse(marshalUtil, optionalTargetObject...)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// Parse unmarshals an Object using the given marshalUtil (for easier marshaling/unmarshaling).
func Parse(marshalUtil *marshalutil.MarshalUtil, optionalTarget ...*Object) (result *Object, err error) {
	// determine the target that will hold the unmarshaled information
	switch len(optionalTarget) {
	case 0:
		result = &Object{}
	case 1:
		result = optionalTarget[0]
	default:
		panic("too many arguments in call to FromBytes")
	}

	// read information that are required to identify the object from the outside
	if _, err = marshalUtil.ReadUint32(); err != nil {
		return
	}
	if _, err = marshalUtil.ReadUint32(); err != nil {
		return
	}

	// parse id
	id, err := marshalUtil.ReadBytes(32)
	if err != nil {
		return
	}
	copy(result.id[:], id)

	// parse sent time
	if result.sentTime, err = marshalUtil.ReadInt64(); err != nil {
		return
	}

	// store bytes, so we don't have to marshal manually
	consumedBytes := marshalUtil.ReadOffset()
	copy(result.bytes, marshalUtil.Bytes()[:consumedBytes])

	return
}

// Bytes returns a marshaled version of this Object.
func (o *Object) Bytes() (bytes []byte) {
	// acquire lock for reading bytes
	o.bytesMutex.RLock()

	// return if bytes have been determined already
	if bytes = o.bytes; bytes != nil {
		o.bytesMutex.RUnlock()
		return
	}

	// switch to write lock
	o.bytesMutex.RUnlock()
	o.bytesMutex.Lock()
	defer o.bytesMutex.Unlock()

	// return if bytes have been determined in the mean time
	if bytes = o.bytes; bytes != nil {
		return
	}

	objectLength := len(o.id) + marshalutil.INT64_SIZE
	// initialize helper
	marshalUtil := marshalutil.New(marshalutil.UINT32_SIZE + marshalutil.UINT32_SIZE + objectLength)

	// marshal the payload specific information
	marshalUtil.WriteUint32(Type)
	marshalUtil.WriteUint32(uint32(objectLength))
	marshalUtil.WriteBytes(o.id[:])
	marshalUtil.WriteInt64(o.sentTime)

	bytes = marshalUtil.Bytes()

	return
}

// String returns a human-friendly representation of the Object.
func (o *Object) String() string {
	return stringify.Struct("NetworkDelayObject",
		stringify.StructField("id", o.id),
		stringify.StructField("sentTime", uint64(o.sentTime)),
	)
}

// region Payload implementation ///////////////////////////////////////////////////////////////////////////////////////

// Type represents the identifier which addresses the network delay Object type.
const Type = payload.Type(189)

// Type returns the type of the Object.
func (o *Object) Type() payload.Type {
	return Type
}

// Unmarshal unmarshals the payload from the given bytes.
func (o *Object) Unmarshal(data []byte) (err error) {
	_, _, err = FromBytes(data, o)

	return
}

func init() {
	payload.RegisterType(Type, ObjectName, func(data []byte) (payload payload.Payload, err error) {
		payload = &Object{}
		err = payload.Unmarshal(data)

		return
	})
}

// // endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
