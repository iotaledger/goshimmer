package tangle

import (
	"fmt"
	"sort"
	"strconv"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/datastructure/thresholdmap"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
)

// region MarkerIndexBranchIDMapping ///////////////////////////////////////////////////////////////////////////////////

// MarkerIndexBranchIDMapping is a data structure that allows to map marker Indexes to a BranchID.
type MarkerIndexBranchIDMapping struct {
	sequenceID   markers.SequenceID
	mapping      *thresholdmap.ThresholdMap
	mappingMutex sync.RWMutex

	objectstorage.StorableObjectFlags
}

// NewMarkerIndexBranchIDMapping creates a new MarkerIndexBranchIDMapping for the given SequenceID.
func NewMarkerIndexBranchIDMapping(sequenceID markers.SequenceID) (markerBranchMapping *MarkerIndexBranchIDMapping) {
	markerBranchMapping = &MarkerIndexBranchIDMapping{
		sequenceID: sequenceID,
		mapping:    thresholdmap.New(thresholdmap.LowerThresholdMode, markerIndexComparator),
	}

	markerBranchMapping.SetModified()
	markerBranchMapping.Persist()

	return
}

// MarkerIndexBranchIDMappingFromBytes unmarshals a MarkerIndexBranchIDMapping from a sequence of bytes.
func MarkerIndexBranchIDMappingFromBytes(bytes []byte) (markerIndexBranchIDMapping *MarkerIndexBranchIDMapping, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if markerIndexBranchIDMapping, err = MarkerIndexBranchIDMappingFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse MarkerIndexBranchIDMapping from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// MarkerIndexBranchIDMappingFromMarshalUtil unmarshals a MarkerIndexBranchIDMapping using a MarshalUtil (for easier
// unmarshalling).
func MarkerIndexBranchIDMappingFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (markerIndexBranchIDMapping *MarkerIndexBranchIDMapping, err error) {
	markerIndexBranchIDMapping = &MarkerIndexBranchIDMapping{}
	if markerIndexBranchIDMapping.sequenceID, err = markers.SequenceIDFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse SequenceID from MarshalUtil: %w", err)
		return
	}
	mappingCount, mappingCountErr := marshalUtil.ReadUint64()
	if mappingCountErr != nil {
		err = errors.Errorf("failed to parse reference count (%v): %w", mappingCountErr, cerrors.ErrParseBytesFailed)
		return
	}
	markerIndexBranchIDMapping.mapping = thresholdmap.New(thresholdmap.LowerThresholdMode, markerIndexComparator)
	for j := uint64(0); j < mappingCount; j++ {
		index, indexErr := marshalUtil.ReadUint64()
		if indexErr != nil {
			err = errors.Errorf("failed to parse Index (%v): %w", indexErr, cerrors.ErrParseBytesFailed)
			return
		}

		branchID, branchIDErr := ledgerstate.BranchIDFromMarshalUtil(marshalUtil)
		if branchIDErr != nil {
			err = errors.Errorf("failed to parse BranchID: %w", branchIDErr)
			return
		}

		markerIndexBranchIDMapping.mapping.Set(markers.Index(index), branchID)
	}

	return
}

// MarkerIndexBranchIDMappingFromObjectStorage restores a MarkerIndexBranchIDMapping that was stored in the object
// storage.
func MarkerIndexBranchIDMappingFromObjectStorage(key, data []byte) (markerIndexBranchIDMapping objectstorage.StorableObject, err error) {
	if markerIndexBranchIDMapping, _, err = MarkerIndexBranchIDMappingFromBytes(byteutils.ConcatBytes(key, data)); err != nil {
		err = errors.Errorf("failed to parse MarkerIndexBranchIDMapping from bytes: %w", err)
		return
	}

	return
}

// SequenceID returns the SequenceID that this MarkerIndexBranchIDMapping represents.
func (m *MarkerIndexBranchIDMapping) SequenceID() markers.SequenceID {
	return m.sequenceID
}

// BranchID returns the BranchID that is associated to the given marker Index.
func (m *MarkerIndexBranchIDMapping) BranchID(markerIndex markers.Index) (branchID ledgerstate.BranchID) {
	m.mappingMutex.RLock()
	defer m.mappingMutex.RUnlock()

	value, exists := m.mapping.Get(markerIndex)
	if !exists {
		panic(fmt.Sprintf("tried to retrieve the BranchID of unknown marker.%s", markerIndex))
	}

	return value.(ledgerstate.BranchID)
}

// SetBranchID creates a mapping between the given marker Index and the given BranchID.
func (m *MarkerIndexBranchIDMapping) SetBranchID(index markers.Index, branchID ledgerstate.BranchID) {
	m.mappingMutex.Lock()
	defer m.mappingMutex.Unlock()

	m.mapping.Set(index, branchID)
	m.SetModified()
}

// DeleteBranchID deletes a mapping between the given marker Index and the stored BranchID.
func (m *MarkerIndexBranchIDMapping) DeleteBranchID(index markers.Index) {
	m.mappingMutex.Lock()
	defer m.mappingMutex.Unlock()

	m.mapping.Delete(index)
	m.SetModified()
}

// Floor returns the largest Index that is <= the given Index which has a mapped BranchID (and a boolean value
// indicating if it exists).
func (m *MarkerIndexBranchIDMapping) Floor(index markers.Index) (marker markers.Index, branchID ledgerstate.BranchID, exists bool) {
	m.mappingMutex.RLock()
	defer m.mappingMutex.RUnlock()

	if untypedIndex, untypedBranchID, exists := m.mapping.Floor(index); exists {
		return untypedIndex.(markers.Index), untypedBranchID.(ledgerstate.BranchID), true
	}

	return 0, ledgerstate.UndefinedBranchID, false
}

// Ceiling returns the smallest Index that is >= the given Index which has a mapped BranchID (and a boolean value
// indicating if it exists).
func (m *MarkerIndexBranchIDMapping) Ceiling(index markers.Index) (marker markers.Index, branchID ledgerstate.BranchID, exists bool) {
	m.mappingMutex.RLock()
	defer m.mappingMutex.RUnlock()

	if untypedIndex, untypedBranchID, exists := m.mapping.Ceiling(index); exists {
		return untypedIndex.(markers.Index), untypedBranchID.(ledgerstate.BranchID), true
	}

	return 0, ledgerstate.UndefinedBranchID, false
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
	branchIDs := make(map[markers.Index]ledgerstate.BranchID)
	m.mapping.ForEach(func(node *thresholdmap.Element) bool {
		index := node.Key().(markers.Index)
		indexes = append(indexes, index)
		branchIDs[index] = node.Value().(ledgerstate.BranchID)

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
		stringify.StructField("sequenceID", m.sequenceID),
		stringify.StructField("mapping", mapping),
	)
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (m *MarkerIndexBranchIDMapping) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (m *MarkerIndexBranchIDMapping) ObjectStorageKey() []byte {
	return m.sequenceID.Bytes()
}

// ObjectStorageValue marshals the ConflictBranch into a sequence of bytes that are used as the value part in the
// object storage.
func (m *MarkerIndexBranchIDMapping) ObjectStorageValue() []byte {
	m.mappingMutex.RLock()
	defer m.mappingMutex.RUnlock()

	marshalUtil := marshalutil.New()
	marshalUtil.WriteUint64(uint64(m.mapping.Size()))
	m.mapping.ForEach(func(node *thresholdmap.Element) bool {
		marshalUtil.Write(node.Key().(markers.Index))
		marshalUtil.Write(node.Value().(ledgerstate.BranchID))

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

// region CachedMarkerIndexBranchIDMapping /////////////////////////////////////////////////////////////////////////////

// CachedMarkerIndexBranchIDMapping is a wrapper for the generic CachedObject returned by the object storage that
// overrides the accessor methods with a type-casted one.
type CachedMarkerIndexBranchIDMapping struct {
	objectstorage.CachedObject
}

// Retain marks the CachedObject to still be in use by the program.
func (c *CachedMarkerIndexBranchIDMapping) Retain() *CachedMarkerIndexBranchIDMapping {
	return &CachedMarkerIndexBranchIDMapping{c.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (c *CachedMarkerIndexBranchIDMapping) Unwrap() *MarkerIndexBranchIDMapping {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*MarkerIndexBranchIDMapping)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (c *CachedMarkerIndexBranchIDMapping) Consume(consumer func(markerIndexBranchIDMapping *MarkerIndexBranchIDMapping), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*MarkerIndexBranchIDMapping))
	}, forceRelease...)
}

// String returns a human-readable version of the CachedMarkerIndexBranchIDMapping.
func (c *CachedMarkerIndexBranchIDMapping) String() string {
	return stringify.Struct("CachedMarkerIndexBranchIDMapping",
		stringify.StructField("CachedObject", c.Unwrap()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MarkerMessageMapping /////////////////////////////////////////////////////////////////////////////////////////

// MarkerMessageMappingPartitionKeys defines the "layout" of the key. This enables prefix iterations in the object
// storage.
var MarkerMessageMappingPartitionKeys = objectstorage.PartitionKey(markers.SequenceIDLength, markers.IndexLength)

// MarkerMessageMapping is a data structure that denotes a mapping from a Marker to a Message.
type MarkerMessageMapping struct {
	marker    *markers.Marker
	messageID MessageID

	objectstorage.StorableObjectFlags
}

// NewMarkerMessageMapping is the constructor for the MarkerMessageMapping.
func NewMarkerMessageMapping(marker *markers.Marker, messageID MessageID) *MarkerMessageMapping {
	return &MarkerMessageMapping{
		marker:    marker,
		messageID: messageID,
	}
}

// MarkerMessageMappingFromBytes unmarshals an MarkerMessageMapping from a sequence of bytes.
func MarkerMessageMappingFromBytes(bytes []byte) (individuallyMappedMessage *MarkerMessageMapping, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if individuallyMappedMessage, err = MarkerMessageMappingFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse MarkerMessageMapping from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// MarkerMessageMappingFromMarshalUtil unmarshals an MarkerMessageMapping using a MarshalUtil (for easier unmarshalling).
func MarkerMessageMappingFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (markerMessageMapping *MarkerMessageMapping, err error) {
	markerMessageMapping = &MarkerMessageMapping{}
	if markerMessageMapping.marker, err = markers.MarkerFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse Marker from MarshalUtil: %w", err)
		return
	}
	if markerMessageMapping.messageID, err = ReferenceFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse MessageID from MarshalUtil: %w", err)
		return
	}

	return
}

// MarkerMessageMappingFromObjectStorage is a factory method that creates a new MarkerMessageMapping instance
// from a storage key of the object storage. It is used by the object storage, to create new instances of this entity.
func MarkerMessageMappingFromObjectStorage(key, value []byte) (result objectstorage.StorableObject, err error) {
	if result, _, err = MarkerMessageMappingFromBytes(byteutils.ConcatBytes(key, value)); err != nil {
		err = errors.Errorf("failed to parse MarkerMessageMapping from bytes: %w", err)
		return
	}

	return
}

// Marker returns the Marker that is mapped to a MessageID.
func (m *MarkerMessageMapping) Marker() *markers.Marker {
	return m.marker
}

// MessageID returns the MessageID of the Marker.
func (m *MarkerMessageMapping) MessageID() MessageID {
	return m.messageID
}

// Bytes returns a marshaled version of the MarkerMessageMapping.
func (m *MarkerMessageMapping) Bytes() []byte {
	return byteutils.ConcatBytes(m.ObjectStorageKey(), m.ObjectStorageValue())
}

// String returns a human-readable version of the MarkerMessageMapping.
func (m *MarkerMessageMapping) String() string {
	return stringify.Struct("MarkerMessageMapping",
		stringify.StructField("marker", m.marker),
		stringify.StructField("messageID", m.messageID),
	)
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (m *MarkerMessageMapping) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (m *MarkerMessageMapping) ObjectStorageKey() []byte {
	return m.marker.Bytes()
}

// ObjectStorageValue marshals the MarkerMessageMapping into a sequence of bytes that are used as the value part in
// the object storage.
func (m *MarkerMessageMapping) ObjectStorageValue() []byte {
	return m.messageID.Bytes()
}

// code contract (make sure the type implements all required methods).
var _ objectstorage.StorableObject = &MarkerMessageMapping{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedMarkerMessageMapping ///////////////////////////////////////////////////////////////////////////////////

// CachedMarkerMessageMapping is a wrapper for the generic CachedObject returned by the object storage that overrides
// the accessor methods with a type-casted one.
type CachedMarkerMessageMapping struct {
	objectstorage.CachedObject
}

// Retain marks the CachedObject to still be in use by the program.
func (c *CachedMarkerMessageMapping) Retain() *CachedMarkerMessageMapping {
	return &CachedMarkerMessageMapping{c.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (c *CachedMarkerMessageMapping) Unwrap() *MarkerMessageMapping {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*MarkerMessageMapping)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (c *CachedMarkerMessageMapping) Consume(consumer func(markerMessageMapping *MarkerMessageMapping), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*MarkerMessageMapping))
	}, forceRelease...)
}

// String returns a human-readable version of the CachedMarkerMessageMapping.
func (c *CachedMarkerMessageMapping) String() string {
	return stringify.Struct("CachedMarkerMessageMapping",
		stringify.StructField("CachedObject", c.Unwrap()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedMarkerMessageMappings //////////////////////////////////////////////////////////////////////////////////

// CachedMarkerMessageMappings defines a slice of *CachedMarkerMessageMapping.
type CachedMarkerMessageMappings []*CachedMarkerMessageMapping

// Unwrap is the type-casted equivalent of Get. It returns a slice of unwrapped objects with the object being nil if it
// does not exist.
func (c CachedMarkerMessageMappings) Unwrap() (unwrappedMarkerMessageMappings []*MarkerMessageMapping) {
	unwrappedMarkerMessageMappings = make([]*MarkerMessageMapping, len(c))
	for i, cachedMarkerMessageMapping := range c {
		untypedObject := cachedMarkerMessageMapping.Get()
		if untypedObject == nil {
			continue
		}

		typedObject := untypedObject.(*MarkerMessageMapping)
		if typedObject == nil || typedObject.IsDeleted() {
			continue
		}

		unwrappedMarkerMessageMappings[i] = typedObject
	}

	return
}

// Consume iterates over the CachedObjects, unwraps them and passes a type-casted version to the consumer (if the object
// is not empty - it exists). It automatically releases the object when the consumer finishes. It returns true, if at
// least one object was consumed.
func (c CachedMarkerMessageMappings) Consume(consumer func(markerMessageMapping *MarkerMessageMapping), forceRelease ...bool) (consumed bool) {
	for _, cachedMarkerMessageMapping := range c {
		consumed = cachedMarkerMessageMapping.Consume(consumer, forceRelease...) || consumed
	}

	return
}

// Release is a utility function that allows us to release all CachedObjects in the collection.
func (c CachedMarkerMessageMappings) Release(force ...bool) {
	for _, cachedMarkerMessageMapping := range c {
		cachedMarkerMessageMapping.Release(force...)
	}
}

// String returns a human-readable version of the CachedMarkerMessageMappings.
func (c CachedMarkerMessageMappings) String() string {
	structBuilder := stringify.StructBuilder("CachedMarkerMessageMappings")
	for i, cachedMarkerMessageMapping := range c {
		structBuilder.AddField(stringify.StructField(strconv.Itoa(i), cachedMarkerMessageMapping))
	}

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
