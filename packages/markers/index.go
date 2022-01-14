package markers

import (
	"strconv"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
)

// region Index ////////////////////////////////////////////////////////////////////////////////////////////////////////

// IndexLength represents the amount of bytes of a marshaled Index.
const IndexLength = marshalutil.Uint64Size

// Index represents the ever increasing number of the Markers in a Sequence.
type Index uint64

// IndexFromBytes unmarshals an Index from a sequence of bytes.
func IndexFromBytes(sequenceBytes []byte) (index Index, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(sequenceBytes)
	if index, err = IndexFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse Index from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// IndexFromMarshalUtil unmarshals an Index using a MarshalUtil (for easier unmarshaling).
func IndexFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (index Index, err error) {
	untypedIndex, err := marshalUtil.ReadUint64()
	if err != nil {
		err = errors.Errorf("failed to parse Index (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	index = Index(untypedIndex)

	return
}

// Bytes returns a marshaled version of the Index.
func (i Index) Bytes() (marshaledIndex []byte) {
	return marshalutil.New(marshalutil.Uint64Size).
		WriteUint64(uint64(i)).
		Bytes()
}

// String returns a human readable version of the Index.
func (i Index) String() (humanReadableIndex string) {
	return "Index(" + strconv.FormatUint(uint64(i), 10) + ")"
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region IncreaseIndexCallback ////////////////////////////////////////////////////////////////////////////////////////

// IncreaseIndexCallback is the type of the callback function that is used to determine if a new Index is supposed to be
// assigned in a given Sequence.
type IncreaseIndexCallback func(sequenceID SequenceID, currentHighestIndex Index) bool

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region IndexComparator //////////////////////////////////////////////////////////////////////////////////////////////

func IndexComparator(a interface{}, b interface{}) int {
	aCasted := a.(Index)
	bCasted := b.(Index)
	switch {
	case aCasted < bCasted:
		return -1
	case aCasted > bCasted:
		return 1
	default:
		return 0
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
