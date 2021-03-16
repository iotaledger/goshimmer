package statement

import (
	"sort"
	"strconv"

	"github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/xerrors"
)

const (
	// OpinionLength defines the opinion length in bytes.
	OpinionLength = 2
)

// region Opinion /////////////////////////////////////////////////////////////////////////////////////////////////////

// Opinion holds the opinion at a specific round.
type Opinion struct {
	Value opinion.Opinion
	Round uint8
}

// Bytes returns a marshaled version of the opinion.
func (o Opinion) Bytes() (bytes []byte) {
	return marshalutil.New(OpinionLength).
		WriteByte(byte(o.Value)).
		WriteUint8(o.Round).
		Bytes()
}

// OpinionFromBytes parses an opinion from a byte slice.
func OpinionFromBytes(bytes []byte) (opinion Opinion, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if opinion, err = OpinionFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Opinion from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// OpinionFromMarshalUtil is a wrapper for simplified unmarshaling in a byte stream using the marshalUtil package.
func OpinionFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (result Opinion, err error) {
	readStartOffset := marshalUtil.ReadOffset()

	// read information that are required to identify the Opinion
	result = Opinion{}
	opinionByte, e := marshalUtil.ReadByte()
	if e != nil {
		err = xerrors.Errorf("failed to parse opinion from bytes: %w", e)
		return
	}
	result.Value = opinion.Opinion(opinionByte)

	if result.Round, err = marshalUtil.ReadUint8(); err != nil {
		err = xerrors.Errorf("failed to parse round from bytes: %w", err)
		return
	}
	// return the number of bytes we processed
	parsedBytes := marshalUtil.ReadOffset() - readStartOffset
	if parsedBytes != OpinionLength {
		err = xerrors.Errorf("parsed bytes (%d) did not match expected size (%d): %w", parsedBytes, OpinionLength, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// String returns a human readable version of the Opinion.
func (o Opinion) String() string {
	structBuilder := stringify.StructBuilder("Opinion:")
	structBuilder.AddField(stringify.StructField("Value", o.Value.String()))
	structBuilder.AddField(stringify.StructField("Round", strconv.Itoa(int(o.Round))))

	return structBuilder.String()
}

// endregion /////////////////////////////////////////////////////////////////////////////////////////////////////

// region Opinions /////////////////////////////////////////////////////////////////////////////////////////////////////

// Opinions is a slice of Opinion.
type Opinions []Opinion

func (o Opinions) Len() int           { return len(o) }
func (o Opinions) Less(i, j int) bool { return o[i].Round < o[j].Round }
func (o Opinions) Swap(i, j int)      { o[i], o[j] = o[j], o[i] }

// Last returns the opinion of the last round.
func (o Opinions) Last() Opinion {
	sort.Sort(o)
	return o[len(o)-1]
}

// Finalized returns true if the given opinion has been finalized.
func (o Opinions) Finalized(l int) bool {
	// return if the number of opinions is less than the FPC parameter l
	if len(o) < l {
		return false
	}

	// Sort the opinions by round
	sort.Sort(o)

	// check for l consecutive opinions with the same value.
	target := o[len(o)-1].Value
	for i := len(o) - 2; i >= l; i-- {
		if o[i].Value != target {
			return false
		}
	}
	return true
}

// endregion /////////////////////////////////////////////////////////////////////////////////////////////////////
