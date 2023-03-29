package markermanager

import (
	"fmt"
	"sync"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/markers"
	"github.com/iotaledger/hive.go/ds/thresholdmap"
)

type MarkerIndexConflictIDMapping struct {
	t *thresholdmap.ThresholdMap[markers.Index, utxo.TransactionIDs]
	sync.RWMutex
}

// NewMarkerIndexConflictIDMapping creates a new MarkerIndexConflictIDMapping.
func NewMarkerIndexConflictIDMapping() (m *MarkerIndexConflictIDMapping) {
	return &MarkerIndexConflictIDMapping{
		t: thresholdmap.New[markers.Index, utxo.TransactionIDs](thresholdmap.LowerThresholdMode),
	}
}

// ConflictIDs returns the ConflictID that is associated to the given marker Index.
func (m *MarkerIndexConflictIDMapping) ConflictIDs(markerIndex markers.Index) (conflictIDs utxo.TransactionIDs) {
	m.RLock()
	defer m.RUnlock()

	value, exists := m.t.Get(markerIndex)
	if !exists {
		panic(fmt.Sprintf("tried to retrieve the ConflictID of unknown marker.%s", markerIndex))
	}

	return value.Clone()
}

// SetConflictIDs creates a mapping between the given marker Index and the given ConflictID.
func (m *MarkerIndexConflictIDMapping) SetConflictIDs(index markers.Index, conflictIDs utxo.TransactionIDs) {
	m.Lock()
	defer m.Unlock()

	m.t.Set(index, conflictIDs)
}

// DeleteConflictID deletes a mapping between the given marker Index and the stored ConflictID.
func (m *MarkerIndexConflictIDMapping) DeleteConflictID(index markers.Index) {
	m.Lock()
	defer m.Unlock()

	m.t.Delete(index)
}

// Floor returns the largest Index that is <= the given Index which has a mapped ConflictID (and a boolean value
// indicating if it exists).
func (m *MarkerIndexConflictIDMapping) Floor(index markers.Index) (marker markers.Index, conflictIDs utxo.TransactionIDs, exists bool) {
	m.RLock()
	defer m.RUnlock()

	if untypedIndex, untypedConflictIDs, exists := m.t.Floor(index); exists {
		return untypedIndex, untypedConflictIDs, true
	}

	return 0, utxo.NewTransactionIDs(), false
}

// Ceiling returns the smallest Index that is >= the given Index which has a mapped ConflictID (and a boolean value
// indicating if it exists).
func (m *MarkerIndexConflictIDMapping) Ceiling(index markers.Index) (marker markers.Index, conflictIDs utxo.TransactionIDs, exists bool) {
	m.RLock()
	defer m.RUnlock()

	if untypedIndex, untypedConflictIDs, exists := m.t.Ceiling(index); exists {
		return untypedIndex, untypedConflictIDs, true
	}

	return 0, utxo.NewTransactionIDs(), false
}
