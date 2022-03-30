package tangle

import (
	"fmt"
	"sort"
	"strconv"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/generics/thresholdmap"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
)

// region MarkerIndexBranchIDMapping ///////////////////////////////////////////////////////////////////////////////////

// MarkerIndexBranchIDMapping is a data structure that allows to map marker Indexes to a BranchID.
type MarkerIndexBranchIDMapping struct {
	markerIndexBranchIDInner `serix:"0"`
}
type markerIndexBranchIDInner struct {
	SequenceID   markers.SequenceID
	Mapping      *thresholdmap.ThresholdMap[markers.Index, ledgerstate.BranchIDs] `serix:"0,lengthPrefixType=uint32"`
	mappingMutex sync.RWMutex

	objectstorage.StorableObjectFlags
}

// NewMarkerIndexBranchIDMapping creates a new MarkerIndexBranchIDMapping for the given SequenceID.
func NewMarkerIndexBranchIDMapping(sequenceID markers.SequenceID) (markerBranchMapping *MarkerIndexBranchIDMapping) {
	markerBranchMapping = &MarkerIndexBranchIDMapping{
		markerIndexBranchIDInner{
			SequenceID: sequenceID,
			Mapping:    thresholdmap.New[markers.Index, ledgerstate.BranchIDs](thresholdmap.LowerThresholdMode, markerIndexComparator),
		},
	}

	markerBranchMapping.SetModified()
	markerBranchMapping.Persist()

	return
}

// FromObjectStorage creates an MarkerIndexBranchIDMapping from sequences of key and bytes.
func (m *MarkerIndexBranchIDMapping) FromObjectStorage(key, bytes []byte) (objectstorage.StorableObject, error) {
	markerIndexBranchIDMapping, err := m.FromBytes(byteutils.ConcatBytes(key, bytes))
	if err != nil {
		err = errors.Errorf("failed to parse MarkerIndexBranchIDMapping from bytes: %w", err)
	}

	return markerIndexBranchIDMapping, err
}

// FromBytes unmarshals a MarkerIndexBranchIDMapping from a sequence of bytes.
func (m *MarkerIndexBranchIDMapping) FromBytes(bytes []byte) (markerIndexBranchIDMapping objectstorage.StorableObject, err error) {
	marshalUtil := marshalutil.New(bytes)
	if markerIndexBranchIDMapping, err = m.FromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse MarkerIndexBranchIDMapping from MarshalUtil: %w", err)
		return
	}

	return
}

// FromMarshalUtil unmarshals a MarkerIndexBranchIDMapping using a MarshalUtil (for easier unmarshalling).
func (m *MarkerIndexBranchIDMapping) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (markerIndexBranchIDMapping *MarkerIndexBranchIDMapping, err error) {
	markerIndexBranchIDMapping = m
	if m == nil {
		markerIndexBranchIDMapping = &MarkerIndexBranchIDMapping{}
	}
	if markerIndexBranchIDMapping.markerIndexBranchIDInner.SequenceID, err = markers.SequenceIDFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse SequenceID from MarshalUtil: %w", err)
		return
	}
	mappingCount, mappingCountErr := marshalUtil.ReadUint32()
	if mappingCountErr != nil {
		err = errors.Errorf("failed to parse reference count (%v): %w", mappingCountErr, cerrors.ErrParseBytesFailed)
		return
	}
	markerIndexBranchIDMapping.markerIndexBranchIDInner.Mapping = thresholdmap.New[markers.Index, ledgerstate.BranchIDs](thresholdmap.LowerThresholdMode, markerIndexComparator)
	for j := uint32(0); j < mappingCount; j++ {
		index, indexErr := marshalUtil.ReadUint64()
		if indexErr != nil {
			err = errors.Errorf("failed to parse Index (%v): %w", indexErr, cerrors.ErrParseBytesFailed)
			return
		}

		branchIDs, branchIDErr := ledgerstate.BranchIDsFromMarshalUtil(marshalUtil)
		if branchIDErr != nil {
			err = errors.Errorf("failed to parse BranchID: %w", branchIDErr)
			return
		}

		markerIndexBranchIDMapping.markerIndexBranchIDInner.Mapping.Set(markers.Index(index), branchIDs)
	}

	return
}

// SequenceID returns the SequenceID that this MarkerIndexBranchIDMapping represents.
func (m *MarkerIndexBranchIDMapping) SequenceID() markers.SequenceID {
	return m.markerIndexBranchIDInner.SequenceID
}

// BranchIDs returns the BranchID that is associated to the given marker Index.
func (m *MarkerIndexBranchIDMapping) BranchIDs(markerIndex markers.Index) (branchIDs ledgerstate.BranchIDs) {
	m.mappingMutex.RLock()
	defer m.mappingMutex.RUnlock()

	value, exists := m.markerIndexBranchIDInner.Mapping.Get(markerIndex)
	if !exists {
		panic(fmt.Sprintf("tried to retrieve the BranchID of unknown marker.%s", markerIndex))
	}

	return value
}

// SetBranchIDs creates a mapping between the given marker Index and the given BranchID.
func (m *MarkerIndexBranchIDMapping) SetBranchIDs(index markers.Index, branchIDs ledgerstate.BranchIDs) {
	m.mappingMutex.Lock()
	defer m.mappingMutex.Unlock()

	m.markerIndexBranchIDInner.Mapping.Set(index, branchIDs)
	m.SetModified()
}

// DeleteBranchID deletes a mapping between the given marker Index and the stored BranchID.
func (m *MarkerIndexBranchIDMapping) DeleteBranchID(index markers.Index) {
	m.mappingMutex.Lock()
	defer m.mappingMutex.Unlock()

	m.markerIndexBranchIDInner.Mapping.Delete(index)
	m.SetModified()
}

// Floor returns the largest Index that is <= the given Index which has a mapped BranchID (and a boolean value
// indicating if it exists).
func (m *MarkerIndexBranchIDMapping) Floor(index markers.Index) (marker markers.Index, branchIDs ledgerstate.BranchIDs, exists bool) {
	m.mappingMutex.RLock()
	defer m.mappingMutex.RUnlock()

	if untypedIndex, untypedBranchIDs, exists := m.markerIndexBranchIDInner.Mapping.Floor(index); exists {
		return untypedIndex, untypedBranchIDs, true
	}

	return 0, ledgerstate.NewBranchIDs(), false
}

// Ceiling returns the smallest Index that is >= the given Index which has a mapped BranchID (and a boolean value
// indicating if it exists).
func (m *MarkerIndexBranchIDMapping) Ceiling(index markers.Index) (marker markers.Index, branchIDs ledgerstate.BranchIDs, exists bool) {
	m.mappingMutex.RLock()
	defer m.mappingMutex.RUnlock()

	if untypedIndex, untypedBranchIDs, exists := m.markerIndexBranchIDInner.Mapping.Ceiling(index); exists {
		return untypedIndex, untypedBranchIDs, true
	}

	return 0, ledgerstate.NewBranchIDs(), false
}

// Bytes returns a marshaled version of the MarkerIndexBranchIDMapping.
func (m *MarkerIndexBranchIDMapping) Bytes() []byte {
	return byteutils.ConcatBytes(m.ObjectStorageKey(), m.ObjectStorageValue())
}

// String returns a human-readable version of the MarkerIndexBranchIDMapping.
func (m *MarkerIndexBranchIDMapping) String() string {
	m.mappingMutex.RLock()
	defer m.mappingMutex.RUnlock()

	indexes := make([]markers.Index, 0)
	branchIDs := make(map[markers.Index]ledgerstate.BranchIDs)
	m.markerIndexBranchIDInner.Mapping.ForEach(func(node *thresholdmap.Element[markers.Index, ledgerstate.BranchIDs]) bool {
		index := node.Key()
		indexes = append(indexes, index)
		branchIDs[index] = node.Value()

		return true
	})

	sort.Slice(indexes, func(i, j int) bool {
		return indexes[i] < indexes[j]
	})

	mapping := stringify.StructBuilder("Mapping")
	for i, referencingIndex := range indexes {
		thresholdStart := strconv.FormatUint(uint64(referencingIndex), 10)
		thresholdEnd := "INF"
		if len(indexes) > i+1 {
			thresholdEnd = strconv.FormatUint(uint64(indexes[i+1])-1, 10)
		}

		if thresholdStart == thresholdEnd {
			mapping.AddField(stringify.StructField("Index("+thresholdStart+")", branchIDs[referencingIndex]))
		} else {
			mapping.AddField(stringify.StructField("Index("+thresholdStart+" ... "+thresholdEnd+")", branchIDs[referencingIndex]))
		}
	}

	return stringify.Struct("MarkerIndexBranchIDMapping",
		stringify.StructField("sequenceID", m.markerIndexBranchIDInner.SequenceID),
		stringify.StructField("mapping", mapping),
	)
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (m *MarkerIndexBranchIDMapping) ObjectStorageKey() []byte {
	return m.markerIndexBranchIDInner.SequenceID.Bytes()
}

// ObjectStorageValue marshals the Branch into a sequence of bytes that are used as the value part in the
// object storage.
func (m *MarkerIndexBranchIDMapping) ObjectStorageValue() []byte {
	m.mappingMutex.RLock()
	defer m.mappingMutex.RUnlock()

	marshalUtil := marshalutil.New()
	marshalUtil.WriteUint32(uint32(m.markerIndexBranchIDInner.Mapping.Size()))
	m.markerIndexBranchIDInner.Mapping.ForEach(func(node *thresholdmap.Element[markers.Index, ledgerstate.BranchIDs]) bool {
		marshalUtil.Write(node.Key())
		marshalUtil.Write(node.Value())

		return true
	})

	return marshalUtil.Bytes()
}

// markerIndexComparator is a comparator for marker Indexes.
func markerIndexComparator(a, b interface{}) int {
	aCasted := a.(markers.Index)
	bCasted := b.(markers.Index)

	switch {
	case aCasted < bCasted:
		return -1
	case aCasted > bCasted:
		return 1
	default:
		return 0
	}
}

// code contract (make sure the type implements all required methods).
var _ objectstorage.StorableObject = &MarkerIndexBranchIDMapping{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MarkerMessageMapping /////////////////////////////////////////////////////////////////////////////////////////

// MarkerMessageMappingPartitionKeys defines the "layout" of the key. This enables prefix iterations in the object
// storage.
var MarkerMessageMappingPartitionKeys = objectstorage.PartitionKey(markers.SequenceIDLength, markers.IndexLength)

// MarkerMessageMapping is a data structure that denotes a mapping from a Marker to a Message.
type MarkerMessageMapping struct {
	markerMessageMappingInner `serix:"0"`
}

type markerMessageMappingInner struct {
	Marker    *markers.Marker
	MessageID MessageID `serix:"0"`

	objectstorage.StorableObjectFlags
}

// NewMarkerMessageMapping is the constructor for the MarkerMessageMapping.
func NewMarkerMessageMapping(marker *markers.Marker, messageID MessageID) *MarkerMessageMapping {
	return &MarkerMessageMapping{
		markerMessageMappingInner{
			Marker:    marker,
			MessageID: messageID,
		},
	}
}

// FromObjectStorage creates an MarkerMessageMapping from sequences of key and bytes.
func (m *MarkerMessageMapping) FromObjectStorage(key, bytes []byte) (objectstorage.StorableObject, error) {
	result, err := m.FromBytes(byteutils.ConcatBytes(key, bytes))
	if err != nil {
		err = errors.Errorf("failed to parse MarkerMessageMapping from bytes: %w", err)
	}
	return result, err
}

// FromBytes unmarshals an MarkerMessageMapping from a sequence of bytes.
func (m *MarkerMessageMapping) FromBytes(bytes []byte) (individuallyMappedMessage objectstorage.StorableObject, err error) {
	marshalUtil := marshalutil.New(bytes)
	if individuallyMappedMessage, err = m.FromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse MarkerMessageMapping from MarshalUtil: %w", err)
		return
	}

	return
}

// FromMarshalUtil unmarshals an MarkerMessageMapping using a MarshalUtil (for easier unmarshalling).
func (m *MarkerMessageMapping) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (markerMessageMapping *MarkerMessageMapping, err error) {
	markerMessageMapping = m
	if m == nil {
		markerMessageMapping = &MarkerMessageMapping{}
	}
	if markerMessageMapping.markerMessageMappingInner.Marker, err = markers.MarkerFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse Marker from MarshalUtil: %w", err)
		return
	}
	if markerMessageMapping.markerMessageMappingInner.MessageID, err = ReferenceFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse MessageID from MarshalUtil: %w", err)
		return
	}

	return
}

// Marker returns the Marker that is mapped to a MessageID.
func (m *MarkerMessageMapping) Marker() *markers.Marker {
	return m.markerMessageMappingInner.Marker
}

// MessageID returns the MessageID of the Marker.
func (m *MarkerMessageMapping) MessageID() MessageID {
	return m.markerMessageMappingInner.MessageID
}

// Bytes returns a marshaled version of the MarkerMessageMapping.
func (m *MarkerMessageMapping) Bytes() []byte {
	return byteutils.ConcatBytes(m.ObjectStorageKey(), m.ObjectStorageValue())
}

// String returns a human-readable version of the MarkerMessageMapping.
func (m *MarkerMessageMapping) String() string {
	return stringify.Struct("MarkerMessageMapping",
		stringify.StructField("marker", m.markerMessageMappingInner.Marker),
		stringify.StructField("messageID", m.markerMessageMappingInner.MessageID),
	)
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (m *MarkerMessageMapping) ObjectStorageKey() []byte {
	return m.markerMessageMappingInner.Marker.Bytes()
}

// ObjectStorageValue marshals the MarkerMessageMapping into a sequence of bytes that are used as the value part in
// the object storage.
func (m *MarkerMessageMapping) ObjectStorageValue() []byte {
	return m.markerMessageMappingInner.MessageID.Bytes()
}

// code contract (make sure the type implements all required methods).
var _ objectstorage.StorableObject = &MarkerMessageMapping{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
