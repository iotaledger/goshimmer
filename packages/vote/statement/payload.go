package statement

import (
	"fmt"
	"sync"

	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/xerrors"
)

const (
	// ObjectName defines the name of the Statement object.
	ObjectName = "Statement"
)

// Payload defines a Statement payload.
type Payload struct {
	ConflictsLen  uint32
	Conflicts     Conflicts
	TimestampsLen uint32
	Timestamps    Timestamps

	bytes      []byte
	bytesMutex sync.RWMutex
}

// NewPayload creates a new Statement payload.
func NewPayload(conflicts Conflicts, timestamps Timestamps) *Payload {
	return &Payload{
		ConflictsLen:  uint32(len(conflicts)),
		Conflicts:     conflicts,
		TimestampsLen: uint32(len(timestamps)),
		Timestamps:    timestamps,
	}
}

// PayloadFromMarshalUtil is a wrapper for simplified unmarshaling in a byte stream using the marshalUtil package.
func PayloadFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (*Payload, error) {
	payload, err := marshalUtil.Parse(func(data []byte) (interface{}, int, error) { return FromBytes(data) })
	if err != nil {
		err = xerrors.Errorf("failed to parse statement payload: %w", err)
		return &Payload{}, err
	}
	return payload.(*Payload), nil
}

// FromBytes parses the marshaled version of a Statement Payload into an object.
// It either returns a new Payload or fills an optionally provided Payload with the parsed information.
func FromBytes(bytes []byte) (result *Payload, consumedBytes int, err error) {
	// initialize helper
	marshalUtil := marshalutil.New(bytes)

	// read information that are required to identify the payload from the outside
	result = &Payload{}
	length, err := marshalUtil.ReadUint32()
	if err != nil {
		err = xerrors.Errorf("failed to parse payload size of statement payload: %w", err)
		return
	}

	if _, err = marshalUtil.ReadUint32(); err != nil {
		err = xerrors.Errorf("failed to parse payload type of statement payload: %w", err)
		return
	}

	// parse conflicts
	if result.ConflictsLen, err = marshalUtil.ReadUint32(); err != nil {
		err = xerrors.Errorf("failed to parse conflicts len of statement payload: %w", err)
		return
	}

	consumedBytes = marshalUtil.ReadOffset() - 8
	if uint32(consumedBytes)+(result.ConflictsLen*ConflictLength) > length {
		err = fmt.Errorf("failed to parse statement payload: number of conflicts overflowing: %w", err)
		return
	}

	conflictsBytes, e := marshalUtil.ReadBytes(int(result.ConflictsLen * ConflictLength))
	if e != nil {
		err = xerrors.Errorf("failed to read bytes while parsing conflicts of statement payload: %w", e)
		return
	}
	if result.Conflicts, _, err = ConflictsFromBytes(conflictsBytes, int(result.ConflictsLen)); err != nil {
		err = xerrors.Errorf("failed to parse conflicts of statement payload: %w", err)
		return
	}

	// parse timestamps
	if result.TimestampsLen, err = marshalUtil.ReadUint32(); err != nil {
		err = xerrors.Errorf("failed to parse timestamps len of statement payload: %w", err)
		return
	}

	consumedBytes = marshalUtil.ReadOffset() - 8
	if uint32(consumedBytes)+result.TimestampsLen*TimestampLength > length {
		err = fmt.Errorf("failed to parse statement payload: number of timestamps overflowing: %w", err)
		return
	}

	timestampsBytes, e := marshalUtil.ReadBytes(int(result.TimestampsLen * TimestampLength))
	if e != nil {
		err = xerrors.Errorf("failed to read bytes while parsing timestamps of statement payload: %w", e)
		return
	}
	if result.Timestamps, _, err = TimestampsFromBytes(timestampsBytes, int(result.TimestampsLen)); err != nil {
		err = xerrors.Errorf("failed to parse timestamps of statement payload: %w", err)
		return
	}

	// return the number of bytes we processed
	consumedBytes = marshalUtil.ReadOffset()

	// store bytes, so we don't have to marshal manually
	result.bytes = bytes[:consumedBytes]

	return
}

// Bytes returns the statement payload bytes.
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
	marshalUtil.WriteUint32(uint32(len(p.Conflicts)*ConflictLength + len(p.Timestamps)*TimestampLength + 8))
	marshalUtil.WriteBytes(Type.Bytes())
	marshalUtil.WriteUint32(p.ConflictsLen)
	marshalUtil.WriteBytes(p.Conflicts.Bytes())
	marshalUtil.WriteUint32(p.TimestampsLen)
	marshalUtil.WriteBytes(p.Timestamps.Bytes())

	bytes = marshalUtil.Bytes()

	return
}

func (p *Payload) String() string {
	return stringify.Struct("Payload",
		stringify.StructField("conflictsLen", p.ConflictsLen),
		stringify.StructField("conflicts", p.Conflicts),
		stringify.StructField("timestampsLen", p.TimestampsLen),
		stringify.StructField("timestamps", p.Timestamps),
	)
}

// region Payload implementation ///////////////////////////////////////////////////////////////////////////////////////

// Type defines the type of the statement payload.
var Type = payload.NewType(3, ObjectName, func(data []byte) (payload payload.Payload, err error) {
	payload, _, err = FromBytes(data)

	return
})

// Type returns the type of the statement payload.
func (p *Payload) Type() payload.Type {
	return Type
}

// Marshal marshals the statement payload into bytes.
func (p *Payload) Marshal() (bytes []byte, err error) {
	return p.Bytes(), nil
}

// // endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
