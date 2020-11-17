package statement

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/hive.go/marshalutil"
	"golang.org/x/xerrors"
)

const (
	// TimestampLength defines the Timestamp length in bytes.
	TimestampLength = tangle.MessageIDLength + 1
)

// Timestamp holds the message ID and its timestamp opinion.
type Timestamp struct {
	ID      tangle.MessageID
	Opinion bool
}

// Timestamps is a slice of Timestamp.
type Timestamps []Timestamp

// Bytes returns the timestamp statment encoded as bytes.
func (t Timestamp) Bytes() (bytes []byte) {
	bytes = make([]byte, TimestampLength)

	// initialize helper
	marshalUtil := marshalutil.New(bytes)
	marshalUtil.WriteBytes(t.ID.Bytes())
	marshalUtil.WriteBool(t.Opinion)

	return bytes
}

// Bytes returns the timestamps statments encoded as bytes.
func (t Timestamps) Bytes() (bytes []byte) {
	bytes = make([]byte, TimestampLength*len(t))

	// initialize helper
	marshalUtil := marshalutil.New(bytes)

	for _, timestamp := range t {
		marshalUtil.WriteBytes(timestamp.ID.Bytes())
		marshalUtil.WriteBool(timestamp.Opinion)
	}

	return bytes
}

// TimestampFromBytes parses a timestamp statement from a byte slice.
func TimestampFromBytes(bytes []byte) (result Timestamp, consumedBytes int, err error) {
	// initialize helper
	marshalUtil := marshalutil.New(bytes)

	// read information that are required to identify the Conflict
	result = Timestamp{}
	bytesID, err := marshalUtil.ReadBytes(int(tangle.MessageIDLength))
	if err != nil {
		err = xerrors.Errorf("failed to parse ID from timestamp: %w", err)
		return
	}
	result.ID, _, err = tangle.MessageIDFromBytes(bytesID)
	if err != nil {
		err = xerrors.Errorf("failed to parse ID from bytes: %w", err)
		return
	}
	if result.Opinion, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse opinion from timestamp: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// TimestampsFromBytes parses a slice of timestamp statements from a byte slice.
func TimestampsFromBytes(bytes []byte, n int) (result Timestamps, consumedBytes int, err error) {
	if len(bytes)/TimestampLength < n {
		err = xerrors.Errorf("not enough bytes to parse %d timestamps", n)
		return
	}

	// initialize helper
	marshalUtil := marshalutil.New(bytes)

	result = Timestamps{}
	for i := 0; i < n; i++ {
		next, e := marshalUtil.ReadBytes(TimestampLength)
		if e != nil {
			err = xerrors.Errorf("failed to read bytes while parsing timestamp from timestamps: %w", e)
			return
		}
		timestamp, _, e := TimestampFromBytes(next)
		if e != nil {
			err = xerrors.Errorf("failed to parse timestamp from timestamps: %w", e)
			return
		}
		result = append(result, timestamp)
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func (t Timestamps) String() (result string) {
	for _, timestamp := range t {
		result += fmt.Sprintln(timestamp.ID, timestamp.Opinion)
	}
	return result
}
