package networkdelay

import (
	"context"
	"fmt"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/serix"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/goshimmer/packages/tangle/payload"
)

func init() {
	err := serix.DefaultAPI.RegisterTypeSettings(new(Payload), serix.TypeSettings{}.WithObjectCode(new(Payload).Type()))
	if err != nil {
		panic(fmt.Errorf("error registering Transaction type settings: %w", err))
	}
	err = serix.DefaultAPI.RegisterInterfaceObjects((*payload.Payload)(nil), new(Payload))
	if err != nil {
		panic(fmt.Errorf("error registering Transaction as Payload interface: %w", err))
	}
}

const (
	// PayloadName defines the name of the networkdelay payload.
	PayloadName = "networkdelay"
	payloadType = 189
)

// ID represents a 32 byte ID of a network delay payload.
type ID [32]byte

// String returns a human-friendly representation of the ID.
func (id ID) String() string {
	return base58.Encode(id[:])
}

// Payload represents the network delay payload type.
type Payload struct {
	payloadInner `serix:"0"`
}

type payloadInner struct {
	ID       ID    `serix:"0"`
	SentTime int64 `serix:"1"` // [ns]

	bytes      []byte
	bytesMutex sync.RWMutex
}

// NewPayload creates a new  network delay payload.
func NewPayload(id ID, sentTime int64) *Payload {
	return &Payload{
		payloadInner{
			ID:       id,
			SentTime: sentTime,
		},
	}
}

// FromBytes parses the marshaled version of a Payload into a Go object.
// It either returns a new Payload or fills an optionally provided Payload with the parsed information.
func FromBytes(bytes []byte) (result *Payload, consumedBytes int, err error) {
	//TODO: remove eventually
	marshalUtil := marshalutil.New(bytes)
	result, err = Parse(marshalUtil)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// FromBytes parses the marshaled version of a Payload into a Go object.
// It either returns a new Payload or fills an optionally provided Payload with the parsed information.
func FromBytesNew(bytes []byte) (payload *Payload, consumedBytes int, err error) {
	payload = new(Payload)

	consumedBytes, err = serix.DefaultAPI.Decode(context.Background(), bytes, payload, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse NetworkDelayPayload: %w", err)
		return
	}

	return
}

// Parse unmarshals a Payload using the given marshalUtil (for easier marshaling/unmarshaling).
func Parse(marshalUtil *marshalutil.MarshalUtil) (result *Payload, err error) {
	// read information that are required to identify the payload from the outside
	//if _, err = marshalUtil.ReadUint32(); err != nil {
	//	err = fmt.Errorf("failed to parse payload size of networkdelay payload: %w", err)
	//	return
	//}
	if _, err = marshalUtil.ReadUint32(); err != nil {
		err = fmt.Errorf("failed to parse payload type of networkdelay payload: %w", err)
		return
	}

	// parse id
	result = &Payload{}
	id, err := marshalUtil.ReadBytes(32)
	if err != nil {
		err = fmt.Errorf("failed to parse id of networkdelay payload: %w", err)
		return
	}
	copy(result.payloadInner.ID[:], id)

	// parse sent time
	if result.payloadInner.SentTime, err = marshalUtil.ReadInt64(); err != nil {
		err = fmt.Errorf("failed to parse sent time of networkdelay payload: %w", err)
		return
	}

	// store bytes, so we don't have to marshal manually
	consumedBytes := marshalUtil.ReadOffset()
	copy(result.payloadInner.bytes, marshalUtil.Bytes()[:consumedBytes])

	return
}

// Bytes returns a marshaled version of this Payload.
func (p *Payload) Bytes() []byte {
	p.bytesMutex.Lock()
	defer p.bytesMutex.Unlock()
	if objBytes := p.payloadInner.bytes; objBytes != nil {
		return objBytes
	}

	objBytes, err := serix.DefaultAPI.Encode(context.Background(), p, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	p.payloadInner.bytes = objBytes
	return objBytes
}

// Bytes returns a marshaled version of this Payload.
func (p *Payload) BytesOld() (bytes []byte) {
	// remove eventuallyy
	// acquire lock for reading bytes
	p.bytesMutex.RLock()

	// return if bytes have been determined already
	if bytes = p.payloadInner.bytes; bytes != nil {
		p.bytesMutex.RUnlock()
		return
	}

	// switch to write lock
	p.bytesMutex.RUnlock()
	p.bytesMutex.Lock()
	defer p.bytesMutex.Unlock()

	// return if bytes have been determined in the mean time
	if bytes = p.payloadInner.bytes; bytes != nil {
		return
	}

	payloadLength := len(p.payloadInner.ID) + marshalutil.Int64Size
	// initialize helper
	marshalUtil := marshalutil.New(marshalutil.Uint32Size + marshalutil.Uint32Size + payloadLength)

	// marshal the payload specific information
	//marshalUtil.WriteUint32(payload.TypeLength + uint32(payloadLength))
	marshalUtil.WriteBytes(Type.Bytes())
	marshalUtil.WriteBytes(p.payloadInner.ID[:])
	marshalUtil.WriteInt64(p.payloadInner.SentTime)

	bytes = marshalUtil.Bytes()

	return bytes
}

// String returns a human-friendly representation of the Payload.
func (p *Payload) String() string {
	return stringify.Struct("NetworkDelayPayload",
		stringify.StructField("id", p.payloadInner.ID),
		stringify.StructField("sentTime", uint64(p.payloadInner.SentTime)),
	)
}

// region Payload implementation ///////////////////////////////////////////////////////////////////////////////////////

// Type represents the identifier which addresses the network delay Payload type.
var Type = payload.NewType(payloadType, PayloadName, func(data []byte) (payload payload.Payload, err error) {
	var consumedBytes int
	payload, consumedBytes, err = FromBytes(data)
	if err != nil {
		return nil, err
	}
	if consumedBytes != len(data) {
		return nil, errors.New("not all payload bytes were consumed")
	}
	return
})

// Type returns the type of the Payload.
func (p *Payload) Type() payload.Type {
	return Type
}

// // endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
