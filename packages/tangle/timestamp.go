package tangle

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"golang.org/x/xerrors"
)

var (
	// TimestampWindow defines the time window for assessing the timestamp quality.
	TimestampWindow time.Duration
	// GratuitousNetworkDelay defines the time after which we assume all messages are delivered.
	GratuitousNetworkDelay time.Duration
)

const (
	// TimestampOpinionLength defines the length of a TimestampOpinion object.
	// (1 byte of opinion, 1 byte of LoK).
	TimestampOpinionLength = 2
)

// TimestampOpinion contains the value of a timestamp opinion as well as its level of knowledge.
type TimestampOpinion struct {
	Value opinion.Opinion
	LoK   LevelOfKnowledge
}

// Bytes returns the timestamp statement encoded as bytes.
func (t TimestampOpinion) Bytes() (bytes []byte) {
	return marshalutil.New(TimestampOpinionLength).
		WriteByte(byte(t.Value)).
		WriteUint8(uint8(t.LoK)).
		Bytes()
}

// TimestampOpinionFromBytes parses a TimestampOpinion from a byte slice.
func TimestampOpinionFromBytes(bytes []byte) (timestampOpinion TimestampOpinion, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if timestampOpinion, err = TimestampOpinionFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse TimestampOpinion from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// TimestampOpinionFromMarshalUtil is a wrapper for simplified unmarshaling in a byte stream using the marshalUtil package.
func TimestampOpinionFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (result TimestampOpinion, err error) {
	readStartOffset := marshalUtil.ReadOffset()

	// read information that are required to identify the TimestampOpinion
	result = TimestampOpinion{}
	opinionByte, e := marshalUtil.ReadByte()
	if e != nil {
		err = xerrors.Errorf("failed to parse opinion from bytes: %w", e)
		return
	}
	result.Value = opinion.Opinion(opinionByte)

	if result.LoK, err = marshalUtil.ReadUint8(); err != nil {
		err = xerrors.Errorf("failed to parse Level of Knowledge from bytes: %w", err)
		return
	}
	// return the number of bytes we processed
	parsedBytes := marshalUtil.ReadOffset() - readStartOffset
	if parsedBytes != TimestampOpinionLength {
		err = xerrors.Errorf("parsed bytes (%d) did not match expected size (%d): %w", parsedBytes, TimestampOpinionLength, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// TimestampQuality returns the timestamp opinion based on the given times (e.g., arrival and current).
func TimestampQuality(target, current time.Time) (timestampOpinion TimestampOpinion) {
	diff := abs(current.Sub(target))

	timestampOpinion.Value = opinion.Like
	if diff >= TimestampWindow {
		timestampOpinion.Value = opinion.Dislike
	}

	switch {
	case abs(diff-TimestampWindow) < GratuitousNetworkDelay:
		timestampOpinion.LoK = One
	case abs(diff-TimestampWindow) < 2*GratuitousNetworkDelay:
		timestampOpinion.LoK = Two
	default:
		timestampOpinion.LoK = Three
	}

	return
}

// Equal returns true if the given timestampOpinion is equal to the given x.
func (t TimestampOpinion) Equal(x TimestampOpinion) bool {
	return t.Value == x.Value && t.LoK == x.LoK
}

func abs(a time.Duration) time.Duration {
	if a >= 0 {
		return a
	}
	return -a
}
