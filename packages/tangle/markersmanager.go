package tangle

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/hive.go/datastructure/walker"
)

type MarkersManager struct {
	tangle                 *Tangle
	newMarkerIndexStrategy markers.IncreaseIndexCallback

	*markers.Manager
}

func NewMarkersManager(tangle *Tangle) *MarkersManager {
	return &MarkersManager{
		tangle: tangle,
	}
}

func (m *MarkersManager) InheritStructureDetails(message *Message, branchID ledgerstate.BranchID, parentsStructureDetails []*markers.StructureDetails) (structureDetails *markers.StructureDetails) {
	structureDetails, _ = m.Manager.InheritStructureDetails(parentsStructureDetails, m.newMarkerIndexStrategy, markers.NewSequenceAlias(branchID.Bytes()))

	if structureDetails.IsPastMarker {
		m.tangle.Utils.WalkMessageMetadata(m.propagatePastMarkerToFutureMarkers(structureDetails.PastMarkers.FirstMarker()), message.StrongParents())
	}

	return
}

func (m *MarkersManager) propagatePastMarkerToFutureMarkers(pastMarkerToInherit *markers.Marker) func(messageMetadata *MessageMetadata, walker *walker.Walker) {
	return func(messageMetadata *MessageMetadata, walker *walker.Walker) {
		_, inheritFurther := m.UpdateStructureDetails(messageMetadata.StructureDetails(), pastMarkerToInherit)
		if inheritFurther {
			m.tangle.Storage.Message(messageMetadata.ID()).Consume(func(message *Message) {
				for _, strongParentMessageID := range message.StrongParents() {
					walker.Push(strongParentMessageID)
				}
			})
		}
	}
}

// structureDetailsOfStrongParents is an internal utility function that returns a list of StructureDetails of all the
// strong parents.
func (m *MarkersManager) structureDetailsOfStrongParents(message *Message) (structureDetails []*markers.StructureDetails) {
	structureDetails = make([]*markers.StructureDetails, 0)
	message.ForEachStrongParent(func(parentMessageID MessageID) {
		if !m.tangle.Storage.MessageMetadata(parentMessageID).Consume(func(messageMetadata *MessageMetadata) {
			structureDetails = append(structureDetails, messageMetadata.StructureDetails())
		}) {
			panic(fmt.Errorf("failed to load MessageMetadata of Message with %s", parentMessageID))
		}
	})

	return
}
