package tangle

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/generics/thresholdmap"
	"github.com/iotaledger/hive.go/serix"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/markers"
)

// region markerIndexBranchIDMap /////////////////////////////////////////////////////////////////////////////////////////

type markerIndexBranchIDMap struct {
	*thresholdmap.ThresholdMap[markers.Index, utxo.TransactionIDs]
}

func newMarkerIndexBranchIDMap() *markerIndexBranchIDMap {
	return &markerIndexBranchIDMap{thresholdmap.New[markers.Index, utxo.TransactionIDs](thresholdmap.LowerThresholdMode, markers.IndexComparator)}
}

// Encode returns a serialized byte slice of the object.
func (m *markerIndexBranchIDMap) Encode() ([]byte, error) {
	return m.ThresholdMap.Encode()
}

// Decode deserializes bytes into a valid object.
func (m *markerIndexBranchIDMap) Decode(b []byte) (bytesRead int, err error) {
	m.ThresholdMap = thresholdmap.New[markers.Index, utxo.TransactionIDs](thresholdmap.LowerThresholdMode, markers.IndexComparator)
	return m.ThresholdMap.Decode(b)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MarkerIndexBranchIDMapping ///////////////////////////////////////////////////////////////////////////////////

// MarkerIndexBranchIDMapping is a data structure that allows to map marker Indexes to a BranchID.
type MarkerIndexBranchIDMapping struct {
	markerIndexBranchIDInner `serix:"0"`
}
type markerIndexBranchIDInner struct {
	SequenceID   markers.SequenceID
	Mapping      *markerIndexBranchIDMap `serix:"0"`
	mappingMutex sync.RWMutex

	objectstorage.StorableObjectFlags
}

// NewMarkerIndexBranchIDMapping creates a new MarkerIndexBranchIDMapping for the given SequenceID.
func NewMarkerIndexBranchIDMapping(sequenceID markers.SequenceID) (markerBranchMapping *MarkerIndexBranchIDMapping) {
	markerBranchMapping = &MarkerIndexBranchIDMapping{
		markerIndexBranchIDInner{
			SequenceID: sequenceID,
			Mapping:    newMarkerIndexBranchIDMap(),
		},
	}

	markerBranchMapping.SetModified()
	markerBranchMapping.Persist()

	return
}

// FromObjectStorage creates an MarkerIndexBranchIDMapping from sequences of key and bytes.
func (m *MarkerIndexBranchIDMapping) FromObjectStorage(key, value []byte) error {
	_, err := m.FromBytes(byteutils.ConcatBytes(key, value))
	if err != nil {
		return errors.Errorf("failed to parse MarkerIndexBranchIDMapping from bytes: %w", err)
	}

	return nil
}

// FromBytes unmarshals a MarkerIndexBranchIDMapping from a sequence of bytes.
func (m *MarkerIndexBranchIDMapping) FromBytes(data []byte) (result *MarkerIndexBranchIDMapping, err error) {
	if result = m; result == nil {
		result = new(MarkerIndexBranchIDMapping)
	}
	sequenceID := new(markers.SequenceID)
	bytesRead, err := serix.DefaultAPI.Decode(context.Background(), data, sequenceID, serix.WithValidation())
	if err != nil {
		return nil, errors.Errorf("failed to parse MarkerIndexBranchIDMapping.SequenceID: %w", err)
	}

	_, err = serix.DefaultAPI.Decode(context.Background(), data[bytesRead:], result, serix.WithValidation())
	if err != nil {
		return nil, errors.Errorf("failed to parse MarkerIndexBranchIDMapping: %w", err)
	}

	result.markerIndexBranchIDInner.SequenceID = *sequenceID
	return result, err
}

// SequenceID returns the SequenceID that this MarkerIndexBranchIDMapping represents.
func (m *MarkerIndexBranchIDMapping) SequenceID() markers.SequenceID {
	return m.markerIndexBranchIDInner.SequenceID
}

// BranchIDs returns the BranchID that is associated to the given marker Index.
func (m *MarkerIndexBranchIDMapping) BranchIDs(markerIndex markers.Index) (branchIDs utxo.TransactionIDs) {
	m.mappingMutex.RLock()
	defer m.mappingMutex.RUnlock()

	value, exists := m.Mapping.Get(markerIndex)
	if !exists {
		panic(fmt.Sprintf("tried to retrieve the BranchID of unknown marker.%s", markerIndex))
	}

	return value
}

// SetBranchIDs creates a mapping between the given marker Index and the given BranchID.
func (m *MarkerIndexBranchIDMapping) SetBranchIDs(index markers.Index, branchIDs utxo.TransactionIDs) {
	m.mappingMutex.Lock()
	defer m.mappingMutex.Unlock()

	m.Mapping.Set(index, branchIDs)
	m.SetModified()
}

// DeleteBranchID deletes a mapping between the given marker Index and the stored BranchID.
func (m *MarkerIndexBranchIDMapping) DeleteBranchID(index markers.Index) {
	m.mappingMutex.Lock()
	defer m.mappingMutex.Unlock()

	m.Mapping.Delete(index)
	m.SetModified()
}

// Floor returns the largest Index that is <= the given Index which has a mapped BranchID (and a boolean value
// indicating if it exists).
func (m *MarkerIndexBranchIDMapping) Floor(index markers.Index) (marker markers.Index, branchIDs utxo.TransactionIDs, exists bool) {
	m.mappingMutex.RLock()
	defer m.mappingMutex.RUnlock()

	if untypedIndex, untypedBranchIDs, exists := m.Mapping.Floor(index); exists {
		return untypedIndex, untypedBranchIDs, true
	}

	return 0, utxo.NewTransactionIDs(), false
}

// Ceiling returns the smallest Index that is >= the given Index which has a mapped BranchID (and a boolean value
// indicating if it exists).
func (m *MarkerIndexBranchIDMapping) Ceiling(index markers.Index) (marker markers.Index, branchIDs utxo.TransactionIDs, exists bool) {
	m.mappingMutex.RLock()
	defer m.mappingMutex.RUnlock()

	if untypedIndex, untypedBranchIDs, exists := m.Mapping.Ceiling(index); exists {
		return untypedIndex, untypedBranchIDs, true
	}

	return 0, utxo.NewTransactionIDs(), false
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
	branchIDs := make(map[markers.Index]utxo.TransactionIDs)
	m.Mapping.ForEach(func(node *thresholdmap.Element[markers.Index, utxo.TransactionIDs]) bool {
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
		stringify.StructField("sequenceID", m.SequenceID()),
		stringify.StructField("mapping", mapping),
	)
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (m *MarkerIndexBranchIDMapping) ObjectStorageKey() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), m.SequenceID(), serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// ObjectStorageValue marshals the MarkerIndexBranchIDMapping into a sequence of bytes. The ID is not serialized here as it is only used as
// a key in the ObjectStorage.
func (m *MarkerIndexBranchIDMapping) ObjectStorageValue() []byte {
	m.mappingMutex.RLock()
	defer m.mappingMutex.RUnlock()
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), m, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
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
func (m *MarkerMessageMapping) FromObjectStorage(key, value []byte) error {
	_, err := m.FromBytes(byteutils.ConcatBytes(key, value))
	if err != nil {
		return errors.Errorf("failed to parse MarkerMessageMapping from bytes: %w", err)
	}
	return nil
}

// FromBytes unmarshals an MarkerMessageMapping from a sequence of bytes.
func (m *MarkerMessageMapping) FromBytes(data []byte) (result *MarkerMessageMapping, err error) {
	if result = m; result == nil {
		result = new(MarkerMessageMapping)
	}
	decodedMarker := new(markers.Marker)
	bytesRead, err := serix.DefaultAPI.Decode(context.Background(), data, decodedMarker, serix.WithValidation())
	if err != nil {
		return nil, errors.Errorf("failed to parse MarkerMessageMapping.Marker: %w", err)
	}

	_, err = serix.DefaultAPI.Decode(context.Background(), data[bytesRead:], result, serix.WithValidation())
	if err != nil {
		return nil, errors.Errorf("failed to parse MarkerMessageMapping: %w", err)
	}
	result.markerMessageMappingInner.Marker = decodedMarker

	return result, nil
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
		stringify.StructField("marker", m.Marker()),
		stringify.StructField("messageID", m.MessageID()),
	)
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (m *MarkerMessageMapping) ObjectStorageKey() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), m.Marker(), serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// ObjectStorageValue marshals the Output into a sequence of bytes. The ID is not serialized here as it is only used as
// a key in the ObjectStorage.
func (m *MarkerMessageMapping) ObjectStorageValue() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), m, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// code contract (make sure the type implements all required methods).
var _ objectstorage.StorableObject = &MarkerMessageMapping{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
