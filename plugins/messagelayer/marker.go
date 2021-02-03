package messagelayer

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/hive.go/objectstorage"
	flag "github.com/spf13/pflag"
	"go.uber.org/atomic"
)

const (
	// CfgMessageCountInterval defines the number of message to create a new marker.
	CfgMessageCountInterval = "marker.messageCountInterval"
)

func init() {
	flag.Int(CfgMessageCountInterval, 10, "number of message interval to create new marker")
}

var (
	messageCount         atomic.Int32
	messageCountInterval int
	markerManager        *markers.Manager
)

// TODO: hook this to MessageBookedEvent
func onMessageBooked(cachedMessage *tangle.CachedMessageEvent) {
	messageCount.Add(1)
	cachedMessage.Message.Consume(func(msg *tangle.Message) {
		var referenceStructureDetails []*markers.StructureDetails
		msg.ForEachParent(func(parent tangle.Parent) {
			cachedParentMetadata := Tangle().MessageMetadata(parent.ID)
			cachedParentMetadata.Consume(func(object objectstorage.StorableObject) {
				metadata := object.(*tangle.MessageMetadata)
				referenceStructureDetails = append(referenceStructureDetails, metadata.StructureDetails())
			})
		})
		var branchID ledgerstate.BranchID
		cachedMessage.MessageMetadata.Consume(func(object objectstorage.StorableObject) {
			metadata := object.(*tangle.MessageMetadata)
			branchID = metadata.BranchID()
		})
		alias, _, err := markers.SequenceAliasFromBytes(branchID.Bytes())
		if err != nil {
			log.Errorf("cannot get sequenceAlias from branchID: %v", err)
			return
		}
		structureDetails, _ := markerManager.InheritStructureDetails(referenceStructureDetails, increaseIndexCallback, alias)
		if structureDetails.IsPastMarker {
			msg.ForEachParent(func(parent tangle.Parent) {
				Tangle().Message(parent.ID).Consume(func(pm *tangle.Message) {
					discovered := make(map[*tangle.Message]bool)
					propagateStructureDetails(pm, structureDetails.PastMarkers.FirstMarker(), discovered)
				})
			})
		}
	})
}

func increaseIndexCallback(_ markers.SequenceID, _ markers.Index) bool {
	return shouldCreateNewMarker()
}
func shouldCreateNewMarker() bool {
	return int(messageCount.Load()) >= messageCountInterval
}

func propagateStructureDetails(msg *tangle.Message, marker *markers.Marker, discovered map[*tangle.Message]bool) {
	discovered[msg] = true
	var inheritFutureMarkersFurther bool
	Tangle().MessageMetadata(msg.ID()).Consume(func(object objectstorage.StorableObject) {
		metadata := object.(*tangle.MessageMetadata)
		_, inheritFutureMarkersFurther = markerManager.UpdateStructureDetails(metadata.StructureDetails(), marker)
	})
	if !inheritFutureMarkersFurther {
		return
	}
	msg.ForEachParent(func(parent tangle.Parent) {
		Tangle().Message(parent.ID).Consume(func(pm *tangle.Message) {
			if !discovered[pm] {
				propagateStructureDetails(pm, marker, discovered)
			}
		})
	})
}
