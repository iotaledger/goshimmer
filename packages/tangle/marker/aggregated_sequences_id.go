package marker

import (
	"github.com/iotaledger/goshimmer/packages/cerrors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/mr-tron/base58"
	"golang.org/x/xerrors"
)

// region AggregatedSequencesID ////////////////////////////////////////////////////////////////////////////////////////

const AggregatedSequencesIDLength = 32

type AggregatedSequencesID [AggregatedSequencesIDLength]byte

func AggregatedSequencesIDFromBytes(aggregatedSequencesIDBytes []byte) (aggregatedSequencesID AggregatedSequencesID, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(aggregatedSequencesIDBytes)
	if aggregatedSequencesID, err = AggregatedSequencesIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse AggregatedSequencesID from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func AggregatedSequencesIDFromBase58EncodedString(base58String string) (aggregatedSequencesID AggregatedSequencesID, err error) {
	bytes, err := base58.Decode(base58String)
	if err != nil {
		err = xerrors.Errorf("error while decoding base58 encoded AggregatedSequencesID (%v): %w", err, cerrors.ErrBase58DecodeFailed)
		return
	}

	if aggregatedSequencesID, _, err = AggregatedSequencesIDFromBytes(bytes); err != nil {
		err = xerrors.Errorf("failed to parse AggregatedSequencesID from bytes: %w", err)
		return
	}

	return
}

func AggregatedSequencesIDFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (aggregatedSequencesID AggregatedSequencesID, err error) {
	aggregatedSequencesIDBytes, err := marshalUtil.ReadBytes(AggregatedSequencesIDLength)
	if err != nil {
		err = xerrors.Errorf("failed to read AggregatedSequencesID (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	copy(aggregatedSequencesID[:], aggregatedSequencesIDBytes)

	return
}

func (a AggregatedSequencesID) Bytes() []byte {
	return a[:]
}

func (a AggregatedSequencesID) Base58() string {
	return base58.Encode(a.Bytes())
}

func (a AggregatedSequencesID) String() string {
	return "AggregatedSequencesID(" + a.Base58() + ")"
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region AggregatedSequencesIDMapping /////////////////////////////////////////////////////////////////////////////////

type AggregatedSequencesIDMapping struct {
	aggregatedSequencesID AggregatedSequencesID
	sequenceID            SequenceID

	objectstorage.StorableObjectFlags
}

func AggregatedSequencesIDMappingFromBytes(mappingBytes []byte) (mapping *AggregatedSequencesIDMapping, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(mappingBytes)
	if mapping, err = AggregatedSequencesIDMappingFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse AggregatedSequencesIDMapping from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func AggregatedSequencesIDMappingFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (mapping *AggregatedSequencesIDMapping, err error) {
	mapping = &AggregatedSequencesIDMapping{}
	if mapping.aggregatedSequencesID, err = AggregatedSequencesIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse AggregatedSequencesID from MarshalUtil: %w", err)
		return
	}
	if mapping.sequenceID, err = SequenceIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse AggregatedSequencesID from MarshalUtil: %w", err)
		return
	}

	return
}

func AggregatedSequencesIDMappingFromObjectStorage(key []byte, data []byte) (mapping objectstorage.StorableObject, err error) {
	if mapping, _, err = AggregatedSequencesIDMappingFromBytes(data); err != nil {
		err = xerrors.Errorf("failed to parse AggregatedSequencesIDMapping from bytes: %w", err)
		return
	}

	return
}

func (a *AggregatedSequencesIDMapping) AggregatedSequencesID() AggregatedSequencesID {
	return a.aggregatedSequencesID
}

func (a *AggregatedSequencesIDMapping) SequenceID() SequenceID {
	return a.sequenceID
}

func (a *AggregatedSequencesIDMapping) Bytes() []byte {
	return byteutils.ConcatBytes(a.ObjectStorageKey(), a.ObjectStorageValue())
}

func (a *AggregatedSequencesIDMapping) Update(other objectstorage.StorableObject) {
	panic("updates disabled")
}

func (a *AggregatedSequencesIDMapping) ObjectStorageKey() []byte {
	return a.aggregatedSequencesID.Bytes()
}

func (a *AggregatedSequencesIDMapping) ObjectStorageValue() []byte {
	return a.sequenceID.Bytes()
}

var _ objectstorage.StorableObject = &AggregatedSequencesIDMapping{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
