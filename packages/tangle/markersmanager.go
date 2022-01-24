package tangle

import (
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/iotaledger/hive.go/identity"
)

// region MarkersManager ///////////////////////////////////////////////////////////////////////////////////////////////

// MarkersManager is a Tangle component that takes care of managing the Markers which are used to infer structural
// information about the Tangle in an efficient way.
type MarkersManager struct {
	tangle         *Tangle
	discardedNodes map[identity.ID]time.Time
	*markers.Manager
}

// NewMarkersManager is the constructor of the MarkersManager.
func NewMarkersManager(tangle *Tangle) *MarkersManager {
	return &MarkersManager{
		tangle:         tangle,
		discardedNodes: make(map[identity.ID]time.Time),
		Manager:        markers.NewManager(tangle.Options.Store, tangle.Options.CacheTimeProvider),
	}
}

// InheritStructureDetails returns the structure Details of a Message that are derived from the StructureDetails of its
// strong and like parents.
func (m *MarkersManager) InheritStructureDetails(message *Message, structureDetails []*markers.StructureDetails, sequenceAlias markers.SequenceAlias) (newStructureDetails *markers.StructureDetails, newSequenceCreated bool) {
	newStructureDetails, newSequenceCreated = m.Manager.InheritStructureDetails(structureDetails, func(sequenceID markers.SequenceID, currentHighestIndex markers.Index) bool {
		nodeID := identity.NewID(message.IssuerPublicKey())
		bufferUsedRatio := float64(m.tangle.Scheduler.BufferSize()) / float64(m.tangle.Scheduler.MaxBufferSize())
		nodeQueueRatio := float64(m.tangle.Scheduler.NodeQueueSize(nodeID)) / float64(m.tangle.Scheduler.BufferSize())
		if bufferUsedRatio > 0.01 && nodeQueueRatio > 0.1 {
			return false
		}
		if discardTime, ok := m.discardedNodes[nodeID]; ok && time.Since(discardTime) < time.Minute {
			return false
		} else if ok && time.Since(discardTime) >= time.Minute {
			delete(m.discardedNodes, nodeID)
		}
		return m.tangle.Options.IncreaseMarkersIndexCallback(sequenceID, currentHighestIndex)
	}, sequenceAlias)
	if newStructureDetails.IsPastMarker {
		m.SetMessageID(newStructureDetails.PastMarkers.Marker(), message.ID())
		m.tangle.Utils.WalkMessageMetadata(m.propagatePastMarkerToFutureMarkers(newStructureDetails.PastMarkers.Marker()), message.ParentsByType(StrongParentType))
	}

	return
}

// MessageID retrieves the MessageID of the given Marker.
func (m *MarkersManager) MessageID(marker *markers.Marker) (messageID MessageID) {
	m.tangle.Storage.MarkerMessageMapping(marker).Consume(func(markerMessageMapping *MarkerMessageMapping) {
		messageID = markerMessageMapping.MessageID()
	})

	return
}

// SetMessageID associates a MessageID with the given Marker.
func (m *MarkersManager) SetMessageID(marker *markers.Marker, messageID MessageID) {
	m.tangle.Storage.StoreMarkerMessageMapping(NewMarkerMessageMapping(marker, messageID))
}

// BranchID returns the BranchID that is associated with the given Marker.
func (m *MarkersManager) BranchID(marker *markers.Marker) (branchID ledgerstate.BranchID) {
	if marker.SequenceID() == 0 {
		return ledgerstate.MasterBranchID
	}

	m.tangle.Storage.MarkerIndexBranchIDMapping(marker.SequenceID()).Consume(func(markerIndexBranchIDMapping *MarkerIndexBranchIDMapping) {
		branchID = markerIndexBranchIDMapping.BranchID(marker.Index())
	})

	return
}

// ConflictBranchIDs returns the ConflictBranchIDs that are associated with the given Marker.
func (m *MarkersManager) ConflictBranchIDs(marker *markers.Marker) (branchIDs ledgerstate.BranchIDs, err error) {
	if branchIDs, err = m.tangle.LedgerState.ResolvePendingConflictBranchIDs(ledgerstate.NewBranchIDs(m.BranchID(marker))); err != nil {
		err = errors.Errorf("failed to resolve ConflictBranchIDs of marker %s: %w", marker, err)
	}
	return
}

// SetBranchID associates a BranchID with the given Marker.
func (m *MarkersManager) SetBranchID(marker *markers.Marker, branchID ledgerstate.BranchID) (updated bool) {
	if floorMarker, floorBranchID, exists := m.Floor(marker); exists {
		if floorBranchID == branchID {
			return false
		}

		if floorMarker == marker.Index() {
			m.UnregisterSequenceAliasMapping(markers.NewSequenceAlias(floorBranchID.Bytes()), marker.SequenceID())

			m.deleteBranchIDMapping(markers.NewMarker(marker.SequenceID(), floorMarker))
		}

		m.registerSequenceAliasMappingIfLastMappedMarker(marker, branchID)
	}

	m.setBranchIDMapping(marker, branchID)

	return true
}

func (m *MarkersManager) setBranchIDMapping(marker *markers.Marker, branchID ledgerstate.BranchID) bool {
	return m.tangle.Storage.MarkerIndexBranchIDMapping(marker.SequenceID(), NewMarkerIndexBranchIDMapping).Consume(func(markerIndexBranchIDMapping *MarkerIndexBranchIDMapping) {
		markerIndexBranchIDMapping.SetBranchID(marker.Index(), branchID)
	})
}

func (m *MarkersManager) deleteBranchIDMapping(marker *markers.Marker) bool {
	return m.tangle.Storage.MarkerIndexBranchIDMapping(marker.SequenceID(), NewMarkerIndexBranchIDMapping).Consume(func(markerIndexBranchIDMapping *MarkerIndexBranchIDMapping) {
		markerIndexBranchIDMapping.DeleteBranchID(marker.Index())
	})
}

func (m *MarkersManager) registerSequenceAliasMappingIfLastMappedMarker(marker *markers.Marker, branchID ledgerstate.BranchID) {
	if _, _, exists := m.Ceiling(markers.NewMarker(marker.SequenceID(), marker.Index()+1)); !exists {
		m.RegisterSequenceAliasMapping(markers.NewSequenceAlias(branchID.Bytes()), marker.SequenceID())
	}
}

// Floor returns the largest Index that is <= the given Marker, it's BranchID and a boolean value indicating if it
// exists.
func (m *MarkersManager) Floor(referenceMarker *markers.Marker) (marker markers.Index, branchID ledgerstate.BranchID, exists bool) {
	m.tangle.Storage.MarkerIndexBranchIDMapping(referenceMarker.SequenceID(), NewMarkerIndexBranchIDMapping).Consume(func(markerIndexBranchIDMapping *MarkerIndexBranchIDMapping) {
		marker, branchID, exists = markerIndexBranchIDMapping.Floor(referenceMarker.Index())
	})

	return
}

// Ceiling returns the smallest Index that is >= the given Marker, it's BranchID and a boolean value indicating if it
// exists.
func (m *MarkersManager) Ceiling(referenceMarker *markers.Marker) (marker markers.Index, branchID ledgerstate.BranchID, exists bool) {
	m.tangle.Storage.MarkerIndexBranchIDMapping(referenceMarker.SequenceID(), NewMarkerIndexBranchIDMapping).Consume(func(markerIndexBranchIDMapping *MarkerIndexBranchIDMapping) {
		marker, branchID, exists = markerIndexBranchIDMapping.Ceiling(referenceMarker.Index())
	})

	return
}

// ForEachBranchIDMapping iterates over all BranchID mappings in the given Sequence that are bigger than the given
// thresholdIndex. Setting the thresholdIndex to 0 will iterate over all existing mappings.
func (m *MarkersManager) ForEachBranchIDMapping(sequenceID markers.SequenceID, thresholdIndex markers.Index, callback func(mappedMarker *markers.Marker, mappedBranchID ledgerstate.BranchID)) {
	currentMarker := markers.NewMarker(sequenceID, thresholdIndex)
	referencingMarkerIndexInSameSequence, mappedBranchID, exists := m.Ceiling(markers.NewMarker(currentMarker.SequenceID(), currentMarker.Index()+1))
	for ; exists; referencingMarkerIndexInSameSequence, mappedBranchID, exists = m.Ceiling(markers.NewMarker(currentMarker.SequenceID(), currentMarker.Index()+1)) {
		currentMarker = markers.NewMarker(currentMarker.SequenceID(), referencingMarkerIndexInSameSequence)
		callback(currentMarker, mappedBranchID)
	}
}

// ForEachMarkerReferencingMarker executes the callback function for each Marker of other Sequences that directly
// reference the given Marker.
func (m *MarkersManager) ForEachMarkerReferencingMarker(referencedMarker *markers.Marker, callback func(referencingMarker *markers.Marker)) {
	m.Sequence(referencedMarker.SequenceID()).Consume(func(sequence *markers.Sequence) {
		sequence.ReferencingMarkers(referencedMarker.Index()).ForEachSorted(func(referencingSequenceID markers.SequenceID, referencingIndex markers.Index) bool {
			if referencingSequenceID == referencedMarker.SequenceID() {
				return true
			}

			callback(markers.NewMarker(referencingSequenceID, referencingIndex))

			return true
		})
	})
}

// propagatePastMarkerToFutureMarkers updates the FutureMarkers of the strong parents of a given message when a new
// PastMaster was assigned.
func (m *MarkersManager) propagatePastMarkerToFutureMarkers(pastMarkerToInherit *markers.Marker) func(messageMetadata *MessageMetadata, walker *walker.Walker) {
	return func(messageMetadata *MessageMetadata, walker *walker.Walker) {
		updated, inheritFurther := m.UpdateStructureDetails(messageMetadata.StructureDetails(), pastMarkerToInherit)
		if updated {
			messageMetadata.SetModified(true)
		}
		if inheritFurther {
			m.tangle.Storage.Message(messageMetadata.ID()).Consume(func(message *Message) {
				for _, strongParentMessageID := range message.ParentsByType(StrongParentType) {
					walker.Push(strongParentMessageID)
				}
			})
		}
	}
}

// increaseMarkersIndexCallbackStrategy implements the default strategy for increasing marker Indexes in the Tangle.
func increaseMarkersIndexCallbackStrategy(markers.SequenceID, markers.Index) bool {
	return true
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
