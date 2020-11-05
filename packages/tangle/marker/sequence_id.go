package marker

import (
	"github.com/gogo/protobuf/types"
	"github.com/iotaledger/goshimmer/packages/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"golang.org/x/xerrors"
)

type SequenceID uint64

func SequenceIDFromBytes(sequenceIDBytes []byte) (sequenceID SequenceID, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(sequenceIDBytes)
	if sequenceID, err = SequenceIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse SequenceID from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func SequenceIDFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (sequenceID SequenceID, err error) {
	readMarkerSequenceID, err := marshalUtil.ReadUint64()
	if err != nil {
		err = xerrors.Errorf("failed to read SequenceID (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	sequenceID = SequenceID(readMarkerSequenceID)

	return
}

func (a SequenceID) Bytes() []byte {
	return marshalutil.New(marshalutil.UINT16_SIZE).WriteUint64(uint64(a)).Bytes()
}

type SequenceIDs map[SequenceID]types.Empty
