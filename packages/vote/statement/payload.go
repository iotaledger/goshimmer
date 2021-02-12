package statement

import (
	"fmt"
	"sync"

	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/xerrors"
)

const (
	// ObjectName defines the name of the Statement object.
	ObjectName = "Statement"
)

// StatementType represents the payload Type of a Statement.
var StatementType payload.Type

func init() {
	// Type defines the type of the statement payload.
	StatementType = payload.NewType(3, ObjectName, func(data []byte) (payload payload.Payload, err error) {
		payload, _, err = FromBytes(data)
		return
	})
}

// Statement defines a Statement payload.
type Statement struct {
	ConflictsCount  uint32
	Conflicts       Conflicts
	TimestampsCount uint32
	Timestamps      Timestamps

	bytes      []byte
	bytesMutex sync.RWMutex
}

// New creates a new Statement payload.
func New(conflicts Conflicts, timestamps Timestamps) *Statement {
	return &Statement{
		ConflictsCount:  uint32(len(conflicts)),
		Conflicts:       conflicts,
		TimestampsCount: uint32(len(timestamps)),
		Timestamps:      timestamps,
	}
}

// FromBytes unmarshals a Statement Payload from a sequence of bytes.
func FromBytes(bytes []byte) (statement *Statement, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if statement, err = Parse(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Statement Payload from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	// store bytes, so we don't have to marshal manually
	statement.bytes = bytes[:consumedBytes]

	return
}

// Parse unmarshals a statement using the given marshalUtil (for easier marshaling/unmarshaling).
func Parse(marshalUtil *marshalutil.MarshalUtil) (statement *Statement, err error) {
	readStartOffset := marshalUtil.ReadOffset()

	// read information that are required to identify the payload from the outside
	statement = &Statement{}
	payloadSize, err := marshalUtil.ReadUint32()
	if err != nil {
		err = xerrors.Errorf("failed to parse payload size of statement payload: %w", err)
		return
	}
	// a payloadSize of 0 indicates the payload is omitted and the payload is nil
	if payloadSize == 0 {
		return
	}

	payloadType, err := payload.TypeFromMarshalUtil(marshalUtil)
	if err != nil {
		err = xerrors.Errorf("failed to parse payload type of statement payload: %w", err)
		return
	}
	if payloadType != StatementType {
		err = xerrors.Errorf("payload type '%s' does not match expected '%s': %w", payloadType, StatementType, cerrors.ErrParseBytesFailed)
		return
	}

	// parse conflicts
	if statement.ConflictsCount, err = marshalUtil.ReadUint32(); err != nil {
		err = xerrors.Errorf("failed to parse conflicts len of statement payload: %w", err)
		return
	}

	parsedBytes := marshalUtil.ReadOffset() - 8 //skip the payload size and type
	if uint32(parsedBytes)+(statement.ConflictsCount*ConflictLength) > payloadSize {
		err = fmt.Errorf("failed to parse statement payload: number of conflicts overflowing: %w", err)
		return
	}

	if statement.Conflicts, err = ConflictsFromMarshalUtil(marshalUtil, statement.ConflictsCount); err != nil {
		err = xerrors.Errorf("failed to parse conflicts from statement payload: %w", err)
		return
	}

	// parse timestamps
	if statement.TimestampsCount, err = marshalUtil.ReadUint32(); err != nil {
		err = xerrors.Errorf("failed to parse timestamps len of statement payload: %w", err)
		return
	}

	parsedBytes = marshalUtil.ReadOffset() - 8 //skip the payload size and type
	if uint32(parsedBytes)+statement.TimestampsCount*TimestampLength > payloadSize {
		err = fmt.Errorf("failed to parse statement payload: number of timestamps overflowing: %w", err)
		return
	}

	if statement.Timestamps, err = TimestampsFromMarshalUtil(marshalUtil, statement.TimestampsCount); err != nil {
		err = xerrors.Errorf("failed to parse timestamps from statement payload: %w", err)
		return
	}

	// return the number of bytes we processed
	parsedBytes = marshalUtil.ReadOffset() - readStartOffset
	if parsedBytes != int(payloadSize)+4 { //skip the payload size
		err = xerrors.Errorf("parsed bytes (%d) did not match expected size (%d): %w", parsedBytes, payloadSize, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// Bytes returns the statement payload bytes.
func (s *Statement) Bytes() (bytes []byte) {
	// acquire lock for reading bytes
	s.bytesMutex.RLock()

	// return if bytes have been determined already
	if bytes = s.bytes; bytes != nil {
		s.bytesMutex.RUnlock()
		return
	}

	// switch to write lock
	s.bytesMutex.RUnlock()
	s.bytesMutex.Lock()
	defer s.bytesMutex.Unlock()

	// return if bytes have been determined in the mean time
	if bytes = s.bytes; bytes != nil {
		return
	}

	payloadBytes := marshalutil.New().
		WriteUint32(s.ConflictsCount).
		Write(s.Conflicts).
		WriteUint32(s.TimestampsCount).
		Write(s.Timestamps).
		Bytes()

	payloadBytesLength := len(payloadBytes)

	// add uint32 for length and type
	return marshalutil.New(2*marshalutil.Uint32Size + payloadBytesLength).
		WriteUint32(payload.TypeLength + uint32(payloadBytesLength)).
		Write(StatementType).
		WriteBytes(payloadBytes).
		Bytes()
}

func (s *Statement) String() string {
	return stringify.Struct("Payload",
		stringify.StructField("conflictsLen", s.ConflictsCount),
		stringify.StructField("conflicts", s.Conflicts),
		stringify.StructField("timestampsLen", s.TimestampsCount),
		stringify.StructField("timestamps", s.Timestamps),
	)
}

// region Payload implementation ///////////////////////////////////////////////////////////////////////////////////////

// Type returns the type of the statement payload.
func (*Statement) Type() payload.Type {
	return StatementType
}

// Marshal marshals the statement payload into bytes.
func (s *Statement) Marshal() (bytes []byte, err error) {
	return s.Bytes(), nil
}

// // endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
