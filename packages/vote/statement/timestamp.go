package statement

import (
	"fmt"
	"strconv"

	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/xerrors"
)

const (
	// TimestampLength defines the Timestamp length in bytes.
	TimestampLength = tangle.MessageIDLength + OpinionLength
)

// region Timestamp /////////////////////////////////////////////////////////////////////////////////////////////////////

// Timestamp holds the message ID and its timestamp opinion.
type Timestamp struct {
	ID tangle.MessageID
	Opinion
}

// Bytes returns the timestamp statement encoded as bytes.
func (t Timestamp) Bytes() (bytes []byte) {
	return marshalutil.New(TimestampLength).
		Write(t.ID).
		Write(t.Opinion).
		Bytes()
}

// TimestampFromBytes parses a conflict statement from a byte slice.
func TimestampFromBytes(bytes []byte) (timestamp Timestamp, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if timestamp, err = TimestampFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Timestamp from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// TimestampFromMarshalUtil is a wrapper for simplified unmarshaling in a byte stream using the marshalUtil package.
func TimestampFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (timestamp Timestamp, err error) {
	readStartOffset := marshalUtil.ReadOffset()

	timestamp = Timestamp{}
	bytesID, err := marshalUtil.ReadBytes(int(tangle.MessageIDLength))
	if err != nil {
		err = xerrors.Errorf("failed to parse ID from timestamp: %w", err)
		return
	}
	timestamp.ID, _, err = tangle.MessageIDFromBytes(bytesID)
	if err != nil {
		err = xerrors.Errorf("failed to parse ID from bytes: %w", err)
		return
	}

	timestamp.Opinion, err = OpinionFromMarshalUtil(marshalUtil)
	if err != nil {
		err = xerrors.Errorf("failed to parse opinion from timestamp: %w", err)
		return
	}

	// return the number of bytes we processed
	parsedBytes := marshalUtil.ReadOffset() - readStartOffset
	if parsedBytes != TimestampLength {
		err = xerrors.Errorf("parsed bytes (%d) did not match expected size (%d): %w", parsedBytes, TimestampLength, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// String returns a human readable version of the Conflict.
func (t Timestamp) String() string {
	structBuilder := stringify.StructBuilder("Timestamp:")
	structBuilder.AddField(stringify.StructField("ID", t.ID.String()))
	structBuilder.AddField(stringify.StructField("Opinion", t.Opinion))

	return structBuilder.String()
}

// endregion /////////////////////////////////////////////////////////////////////////////////////////////////////

// region Timestamps /////////////////////////////////////////////////////////////////////////////////////////////////////

// Timestamps is a slice of Timestamp.
type Timestamps []Timestamp

// Bytes returns the timestamps statements encoded as bytes.
func (t Timestamps) Bytes() (bytes []byte) {
	// initialize helper
	marshalUtil := marshalutil.New(TimestampLength * len(t))

	for _, timestamp := range t {
		marshalUtil.Write(timestamp)
	}

	return marshalUtil.Bytes()
}

// TimestampsFromBytes parses a slice of timestamp statements from a byte slice.
func TimestampsFromBytes(bytes []byte, n uint32) (timestamps Timestamps, consumedBytes int, err error) {
	if len(bytes)/TimestampLength < int(n) {
		err = xerrors.Errorf("not enough bytes to parse %d timestamps", n)
		return
	}

	marshalUtil := marshalutil.New(bytes)
	if timestamps, err = TimestampsFromMarshalUtil(marshalUtil, n); err != nil {
		err = xerrors.Errorf("failed to parse Timestamps from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// TimestampsFromMarshalUtil is a wrapper for simplified unmarshaling in a byte stream using the marshalUtil package.
func TimestampsFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil, n uint32) (timestamps Timestamps, err error) {
	readStartOffset := marshalUtil.ReadOffset()

	timestamps = Timestamps{}
	for i := 0; i < int(n); i++ {
		timestamp, e := TimestampFromMarshalUtil(marshalUtil)
		if e != nil {
			err = fmt.Errorf("failed to parse timestamp from marshalutil: %w", e)
			return
		}
		timestamps = append(timestamps, timestamp)
	}

	// return the number of bytes we processed
	parsedBytes := marshalUtil.ReadOffset() - readStartOffset
	if parsedBytes != int(TimestampLength*n) {
		err = xerrors.Errorf("parsed bytes (%d) did not match expected size (%d): %w", parsedBytes, TimestampLength*n, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// String returns a human readable version of the Timestamps.
func (t Timestamps) String() string {
	structBuilder := stringify.StructBuilder("Timestamps")
	for i, timestamp := range t {
		structBuilder.AddField(stringify.StructField(strconv.Itoa(i), timestamp))
	}

	return structBuilder.String()
}

// endregion /////////////////////////////////////////////////////////////////////////////////////////////////////
