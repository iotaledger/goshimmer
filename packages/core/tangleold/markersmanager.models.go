package tangleold

import (
	"fmt"

	"github.com/iotaledger/hive.go/core/generics/model"
	"github.com/iotaledger/hive.go/core/generics/objectstorage"
	"github.com/iotaledger/hive.go/core/generics/thresholdmap"

	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/markers"
)

// region markerIndexConflictIDMap /////////////////////////////////////////////////////////////////////////////////////////

type markerIndexConflictIDMap struct {
	T *thresholdmap.ThresholdMap[markers.Index, utxo.TransactionIDs] `serix:"0"`
}

func newMarkerIndexConflictIDMap() *markerIndexConflictIDMap {
	return &markerIndexConflictIDMap{thresholdmap.New[markers.Index, utxo.TransactionIDs](thresholdmap.LowerThresholdMode)}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MarkerIndexConflictIDMapping ///////////////////////////////////////////////////////////////////////////////////

// MarkerIndexConflictIDMapping is a data structure that allows to map marker Indexes to a ConflictID.
type MarkerIndexConflictIDMapping struct {
	model.Storable[markers.SequenceID, MarkerIndexConflictIDMapping, *MarkerIndexConflictIDMapping, markerIndexConflictIDMap] `serix:"0"`
}

// NewMarkerIndexConflictIDMapping creates a new MarkerIndexConflictIDMapping for the given SequenceID.
func NewMarkerIndexConflictIDMapping(sequenceID markers.SequenceID) (markerConflictMapping *MarkerIndexConflictIDMapping) {
	markerConflictMapping = model.NewStorable[markers.SequenceID, MarkerIndexConflictIDMapping](
		newMarkerIndexConflictIDMap(),
	)
	markerConflictMapping.SetID(sequenceID)
	return
}

// SequenceID returns the SequenceID that this MarkerIndexConflictIDMapping represents.
func (m *MarkerIndexConflictIDMapping) SequenceID() markers.SequenceID {
	return m.ID()
}

// ConflictIDs returns the ConflictID that is associated to the given marker Index.
func (m *MarkerIndexConflictIDMapping) ConflictIDs(markerIndex markers.Index) (conflictIDs utxo.TransactionIDs) {
	m.RLock()
	defer m.RUnlock()

	value, exists := m.M.T.Get(markerIndex)
	if !exists {
		panic(fmt.Sprintf("tried to retrieve the ConflictID of unknown marker.%s", markerIndex))
	}

	return value.Clone()
}

// SetConflictIDs creates a mapping between the given marker Index and the given ConflictID.
func (m *MarkerIndexConflictIDMapping) SetConflictIDs(index markers.Index, conflictIDs utxo.TransactionIDs) {
	m.Lock()
	defer m.Unlock()

	m.M.T.Set(index, conflictIDs)
	m.SetModified()
}

// DeleteConflictID deletes a mapping between the given marker Index and the stored ConflictID.
func (m *MarkerIndexConflictIDMapping) DeleteConflictID(index markers.Index) {
	m.Lock()
	defer m.Unlock()

	m.M.T.Delete(index)
	m.SetModified()
}

// Floor returns the largest Index that is <= the given Index which has a mapped ConflictID (and a boolean value
// indicating if it exists).
func (m *MarkerIndexConflictIDMapping) Floor(index markers.Index) (marker markers.Index, conflictIDs utxo.TransactionIDs, exists bool) {
	m.RLock()
	defer m.RUnlock()

	if untypedIndex, untypedConflictIDs, exists := m.M.T.Floor(index); exists {
		return untypedIndex, untypedConflictIDs, true
	}

	return 0, utxo.NewTransactionIDs(), false
}

// Ceiling returns the smallest Index that is >= the given Index which has a mapped ConflictID (and a boolean value
// indicating if it exists).
func (m *MarkerIndexConflictIDMapping) Ceiling(index markers.Index) (marker markers.Index, conflictIDs utxo.TransactionIDs, exists bool) {
	m.RLock()
	defer m.RUnlock()

	if untypedIndex, untypedConflictIDs, exists := m.M.T.Ceiling(index); exists {
		return untypedIndex, untypedConflictIDs, true
	}

	return 0, utxo.NewTransactionIDs(), false
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MarkerBlockMapping /////////////////////////////////////////////////////////////////////////////////////////

// MarkerBlockMappingPartitionKeys defines the "layout" of the key. This enables prefix iterations in the object
// storage.
var MarkerBlockMappingPartitionKeys = objectstorage.PartitionKey(markers.SequenceID(0).Length(), markers.Index(0).Length())

// MarkerBlockMapping is a data structure that denotes a mapping from a Marker to a Block.
type MarkerBlockMapping struct {
	model.Storable[markers.Marker, MarkerBlockMapping, *MarkerBlockMapping, BlockID] `serix:"0"`
}

// NewMarkerBlockMapping is the constructor for the MarkerBlockMapping.
func NewMarkerBlockMapping(marker markers.Marker, blockID BlockID) *MarkerBlockMapping {
	markerBlockMapping := model.NewStorable[markers.Marker, MarkerBlockMapping](&blockID)
	markerBlockMapping.SetID(marker)
	return markerBlockMapping
}

// Marker returns the Marker that is mapped to a BlockID.
func (m *MarkerBlockMapping) Marker() *markers.Marker {
	marker := m.ID()
	return &marker
}

// BlockID returns the BlockID of the Marker.
func (m *MarkerBlockMapping) BlockID() BlockID {
	m.RLock()
	defer m.RUnlock()

	return m.M
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
