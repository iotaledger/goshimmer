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
	thresholdmap.ThresholdMap[markers.Index, utxo.TransactionIDs] `serix:"0"`
}

func newMarkerIndexBranchIDMap() *markerIndexBranchIDMap {
	return &markerIndexBranchIDMap{*thresholdmap.New[markers.Index, utxo.TransactionIDs](thresholdmap.LowerThresholdMode)}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MarkerIndexBranchIDMapping ///////////////////////////////////////////////////////////////////////////////////

// MarkerIndexBranchIDMapping is a data structure that allows to map marker Indexes to a BranchID.
type MarkerIndexBranchIDMapping struct {
	model.Storable[markers.SequenceID, *markerIndexBranchIDMap] `serix:"0"`
}

// NewMarkerIndexBranchIDMapping creates a new MarkerIndexBranchIDMapping for the given SequenceID.
func NewMarkerIndexBranchIDMapping(sequenceID markers.SequenceID) (markerBranchMapping *MarkerIndexBranchIDMapping) {
	markerBranchMapping = &MarkerIndexBranchIDMapping{
		model.NewStorable[markers.SequenceID, *markerIndexBranchIDMap](
			newMarkerIndexBranchIDMap(),
		),
	}
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

	value, exists := m.M.Get(markerIndex)
	if !exists {
		panic(fmt.Sprintf("tried to retrieve the BranchID of unknown marker.%s", markerIndex))
	}

	return value
}

// SetBranchIDs creates a mapping between the given marker Index and the given BranchID.
func (m *MarkerIndexBranchIDMapping) SetBranchIDs(index markers.Index, branchIDs utxo.TransactionIDs) {
	m.Lock()
	defer m.Unlock()

	m.M.Set(index, branchIDs)
	m.SetModified()
}

// DeleteBranchID deletes a mapping between the given marker Index and the stored BranchID.
func (m *MarkerIndexBranchIDMapping) DeleteBranchID(index markers.Index) {
	m.Lock()
	defer m.Unlock()

	m.M.Delete(index)
	m.SetModified()
}

// Floor returns the largest Index that is <= the given Index which has a mapped BranchID (and a boolean value
// indicating if it exists).
func (m *MarkerIndexBranchIDMapping) Floor(index markers.Index) (marker markers.Index, branchIDs utxo.TransactionIDs, exists bool) {
	m.RLock()
	defer m.RUnlock()

	if untypedIndex, untypedBranchIDs, exists := m.M.Floor(index); exists {
		return untypedIndex, untypedBranchIDs, true
	}

	return 0, utxo.NewTransactionIDs(), false
}

// Ceiling returns the smallest Index that is >= the given Index which has a mapped BranchID (and a boolean value
// indicating if it exists).
func (m *MarkerIndexBranchIDMapping) Ceiling(index markers.Index) (marker markers.Index, branchIDs utxo.TransactionIDs, exists bool) {
	m.RLock()
	defer m.RUnlock()

	if untypedIndex, untypedBranchIDs, exists := m.M.Ceiling(index); exists {
		return untypedIndex, untypedBranchIDs, true
	}

	return 0, utxo.NewTransactionIDs(), false
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MarkerMessageMapping /////////////////////////////////////////////////////////////////////////////////////////

// MarkerMessageMappingPartitionKeys defines the "layout" of the key. This enables prefix iterations in the object
// storage.
var MarkerMessageMappingPartitionKeys = objectstorage.PartitionKey(markers.SequenceID(0).Length(), markers.Index(0).Length())

// MarkerMessageMapping is a data structure that denotes a mapping from a Marker to a Message.
type MarkerMessageMapping struct {
	model.Storable[markers.Marker, MessageID] `serix:"0"`
}

// NewMarkerMessageMapping is the constructor for the MarkerMessageMapping.
func NewMarkerMessageMapping(marker *markers.Marker, messageID MessageID) *MarkerMessageMapping {
	markerMessageMapping := &MarkerMessageMapping{
		model.NewStorable[markers.Marker, MessageID](messageID),
	}
	markerMessageMapping.SetID(*marker)
	return markerMessageMapping
}

// Marker returns the Marker that is mapped to a MessageID.
func (m *MarkerMessageMapping) Marker() *markers.Marker {
	marker := m.ID()
	return &marker
}

// MessageID returns the MessageID of the Marker.
func (m *MarkerMessageMapping) MessageID() MessageID {
	m.RLock()
	defer m.RUnlock()

	return m.M
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
