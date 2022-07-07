package tangle

import (
	"fmt"

	"github.com/iotaledger/hive.go/generics/model"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/generics/thresholdmap"

	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/markers"
)

// region markerIndexBranchIDMap /////////////////////////////////////////////////////////////////////////////////////////

type markerIndexBranchIDMap struct {
	T *thresholdmap.ThresholdMap[markers.Index, utxo.TransactionIDs] `serix:"0"`
}

func newMarkerIndexBranchIDMap() *markerIndexBranchIDMap {
	return &markerIndexBranchIDMap{thresholdmap.New[markers.Index, utxo.TransactionIDs](thresholdmap.LowerThresholdMode)}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MarkerIndexBranchIDMapping ///////////////////////////////////////////////////////////////////////////////////

// MarkerIndexBranchIDMapping is a data structure that allows to map marker Indexes to a BranchID.
type MarkerIndexBranchIDMapping struct {
	model.Storable[markers.SequenceID, MarkerIndexBranchIDMapping, *MarkerIndexBranchIDMapping, markerIndexBranchIDMap] `serix:"0"`
}

// NewMarkerIndexBranchIDMapping creates a new MarkerIndexBranchIDMapping for the given SequenceID.
func NewMarkerIndexBranchIDMapping(sequenceID markers.SequenceID) (markerBranchMapping *MarkerIndexBranchIDMapping) {
	markerBranchMapping = model.NewStorable[markers.SequenceID, MarkerIndexBranchIDMapping](
		newMarkerIndexBranchIDMap(),
	)
	markerBranchMapping.SetID(sequenceID)
	return
}

// SequenceID returns the SequenceID that this MarkerIndexBranchIDMapping represents.
func (m *MarkerIndexBranchIDMapping) SequenceID() markers.SequenceID {
	return m.ID()
}

// BranchIDs returns the BranchID that is associated to the given marker Index.
func (m *MarkerIndexBranchIDMapping) BranchIDs(markerIndex markers.Index) (branchIDs utxo.TransactionIDs) {
	m.RLock()
	defer m.RUnlock()

	value, exists := m.M.T.Get(markerIndex)
	if !exists {
		panic(fmt.Sprintf("tried to retrieve the BranchID of unknown marker.%s", markerIndex))
	}

	return value.Clone()
}

// SetBranchIDs creates a mapping between the given marker Index and the given BranchID.
func (m *MarkerIndexBranchIDMapping) SetBranchIDs(index markers.Index, branchIDs utxo.TransactionIDs) {
	m.Lock()
	defer m.Unlock()

	m.M.T.Set(index, branchIDs)
	m.SetModified()
}

// DeleteBranchID deletes a mapping between the given marker Index and the stored BranchID.
func (m *MarkerIndexBranchIDMapping) DeleteBranchID(index markers.Index) {
	m.Lock()
	defer m.Unlock()

	m.M.T.Delete(index)
	m.SetModified()
}

// Floor returns the largest Index that is <= the given Index which has a mapped BranchID (and a boolean value
// indicating if it exists).
func (m *MarkerIndexBranchIDMapping) Floor(index markers.Index) (marker markers.Index, branchIDs utxo.TransactionIDs, exists bool) {
	m.RLock()
	defer m.RUnlock()

	if untypedIndex, untypedBranchIDs, exists := m.M.T.Floor(index); exists {
		return untypedIndex, untypedBranchIDs, true
	}

	return 0, utxo.NewTransactionIDs(), false
}

// Ceiling returns the smallest Index that is >= the given Index which has a mapped BranchID (and a boolean value
// indicating if it exists).
func (m *MarkerIndexBranchIDMapping) Ceiling(index markers.Index) (marker markers.Index, branchIDs utxo.TransactionIDs, exists bool) {
	m.RLock()
	defer m.RUnlock()

	if untypedIndex, untypedBranchIDs, exists := m.M.T.Ceiling(index); exists {
		return untypedIndex, untypedBranchIDs, true
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
