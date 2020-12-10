package networkdelay

import (
	"fmt"
	"sync"

	"github.com/iotaledger/goshimmer/packages/tangle/payload"
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
func FromBytes(bytes []byte) (result *Object, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = Parse(marshalUtil)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// Parse unmarshals an Object using the given marshalUtil (for easier marshaling/unmarshaling).
func Parse(marshalUtil *marshalutil.MarshalUtil) (result *Object, err error) {
	// read information that are required to identify the object from the outside
	if _, err = marshalUtil.ReadUint32(); err != nil {
		err = fmt.Errorf("failed to parse payload size of networkdelay object: %w", err)
		return
	}
	if _, err = marshalUtil.ReadUint32(); err != nil {
		err = fmt.Errorf("failed to parse payload type of networkdelay object: %w", err)
		return
	}

	// parse id
	result = &Object{}
	id, err := marshalUtil.ReadBytes(32)
	if err != nil {
		err = fmt.Errorf("failed to parse id of networkdelay object: %w", err)
		return
	}
	copy(result.id[:], id)

	// parse sent time
	if result.sentTime, err = marshalUtil.ReadInt64(); err != nil {
		err = fmt.Errorf("failed to parse sent time of networkdelay object: %w", err)
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

	objectLength := len(o.id) + marshalutil.Int64Size
	// initialize helper
	marshalUtil := marshalutil.New(marshalutil.Uint32Size + marshalutil.Uint32Size + objectLength)

	// marshal the payload specific information
	marshalUtil.WriteUint32(payload.TypeLength + uint32(objectLength))
	marshalUtil.WriteBytes(Type.Bytes())
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
var Type = payload.NewType(189, ObjectName, func(data []byte) (payload payload.Payload, err error) {
	payload, _, err = FromBytes(data)

	return
})

// Type returns the type of the Object.
func (o *Object) Type() payload.Type {
	return Type
}

// // endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
