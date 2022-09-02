package blockcreation

import (
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/randommap"
	"github.com/iotaledger/hive.go/core/generics/walker"

	"github.com/iotaledger/goshimmer/packages/core/tangleold/payload"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/virtualvoting"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/markersold"
)

// region TipManager ///////////////////////////////////////////////////////////////////////////////////////////////////

type TipManager struct {
	tangle              *tangle.Tangle
	tips                *randommap.RandomMap[*virtualvoting.Block, *virtualvoting.Block]
	tipsConflictTracker *TipsConflictTracker
	Events              *TipManagerEvents

	optsWidth int
}

func NewTipManager(tangle *tangle.Tangle, opts ...options.Option[TipManager]) *TipManager {
	return options.Apply(&TipManager{
		tangle:              tangle,
		tips:                randommap.New[*virtualvoting.Block, *virtualvoting.Block](),
		tipsConflictTracker: NewTipsConflictTracker(tangle),
		Events:              newTipManagerEvents(),
	}, opts)
}

// Setup sets up the behavior of the component by making it attach to the relevant events of other components.
func (t *TipManager) Setup() {
	// TODO:
	// t.tangle.Scheduler.Events.BlockScheduled.Attach(event.NewClosure(func(event *BlockScheduledEvent) {
	// 	t.tangle.Storage.Block(event.BlockID).Consume(t.AddTip)
	// }))
	//
	// t.tangle.ConfirmationOracle.Events().BlockAccepted.Attach(event.NewClosure(func(event *BlockAcceptedEvent) {
	// 	t.removeStrongParents(event.Block)
	// }))
	//
	// t.tangle.OrphanageManager.Events.BlockOrphaned.Hook(event.NewClosure(func(event *BlockOrphanedEvent) {
	// 	t.deleteTip(event.Block.ID())
	// }))
	//
	// t.tangle.TimeManager.Events.AcceptanceTimeUpdated.Attach(event.NewClosure(func(event *TimeUpdate) {
	// 	t.tipsCleaner.RemoveBefore(event.UpdateTime.Add(-t.tangle.Options.TimeSinceConfirmationThreshold))
	// }))
	//
	// t.tangle.OrphanageManager.Events.AllChildrenOrphaned.Hook(event.NewClosure(func(block *Block) {
	// 	if clock.Since(block.IssuingTime()) > tipLifeGracePeriod {
	// 		return
	// 	}
	//
	// 	t.addTip(block)
	// }))

	t.tipsConflictTracker.Setup()
}

func (t *TipManager) AddTip(block *virtualvoting.Block) {
	blockID := block.ID()

	// Check if any children that are confirmed or scheduled and return if true, to guarantee that the parents are not added to the tipset after its children.
	if t.checkChildren(blockID) {
		return
	}

	if !t.addTip(block) {
		return
	}

	// skip removing tips if a width is set -> allows to artificially create a wide Tangle.
	if t.TipCount() <= t.optsWidth {
		return
	}

	// a tip loses its tip status if it is referenced by another block
	t.removeStrongParents(block)
}

func (t *TipManager) addTip(block *virtualvoting.Block) (added bool) {
	if t.tips.Set(block, block) {
		t.tipsConflictTracker.AddTip(block)
		t.Events.TipAdded.Trigger(block)
		return true
	}

	return false
}

func (t *TipManager) deleteTip(block *virtualvoting.Block) (deleted bool) {
	if _, deleted = t.tips.Delete(block); deleted {
		t.tipsConflictTracker.RemoveTip(block)
		t.Events.TipRemoved.Trigger(block)
	}
	return
}

// checkChildren returns true if the block has any confirmed or scheduled child.
func (t *TipManager) checkChildren(blockID BlockID) bool {
	childScheduledConfirmed := false
	t.tangle.Storage.Children(blockID).Consume(func(child *Child) {
		if childScheduledConfirmed {
			return
		}

		childScheduledConfirmed = t.tangle.ConfirmationOracle.IsBlockConfirmed(child.ChildBlockID())
		if !childScheduledConfirmed {
			t.tangle.Storage.BlockMetadata(child.ChildBlockID()).Consume(func(blockMetadata *BlockMetadata) {
				childScheduledConfirmed = blockMetadata.Scheduled()
			})
		}
	})
	return childScheduledConfirmed
}

func (t *TipManager) removeStrongParents(block *Block) {
	block.ForEachParentByType(StrongParentType, func(parentBlockID BlockID) bool {
		// We do not want to remove the tip if it is the last one representing a pending conflict.
		// if t.isLastTipForConflict(parentBlockID) {
		// 	return true
		// }

		t.deleteTip(parentBlockID)

		return true
	})
}

// Tips returns count number of tips, maximum MaxParentsCount.
func (t *TipManager) Tips(p payload.Payload, countParents int) (parents models.BlockIDs) {
	if countParents > models.MaxParentsCount {
		countParents = models.MaxParentsCount
	}
	if countParents < models.MinParentsCount {
		countParents = models.MinParentsCount
	}

	return t.selectTips(p, countParents)
}

func (t *TipManager) selectTips(p payload.Payload, count int) (parents models.BlockIDs) {
	parents = models.NewBlockIDs()

	tips := t.tips.RandomUniqueEntries(count)

	// only add genesis if no tips are available and not previously referenced (in case of a transaction),
	// or selected ones had incorrect time-since-confirmation
	if len(tips) == 0 {
		parents.Add(EmptyBlockID)
		return
	}

	// at least one tip is returned
	for _, tip := range tips {
		blockID := tip
		if !parents.Contains(blockID) && t.isPastConeTimestampCorrect(blockID) {
			parents.Add(blockID)
		}
	}
	return
}

// AllTips returns a list of all tips that are stored in the TipManger.
func (t *TipManager) AllTips() BlockIDs {
	return retrieveAllTips(t.tips)
}

func retrieveAllTips(tipsMap *randommap.RandomMap[BlockID, BlockID]) BlockIDs {
	mapKeys := tipsMap.Keys()
	tips := NewBlockIDs()

	for _, key := range mapKeys {
		tips.Add(key)
	}
	return tips
}

// TipCount the amount of tips.
func (t *TipManager) TipCount() int {
	return t.tips.Size()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TSC to prevent lazy tips /////////////////////////////////////////////////////////////////////////////////////

// isPastConeTimestampCorrect performs the TSC check for the given tip.
// Conceptually, this involves the following steps:
//   1. Collect all confirmed blocks in the tip's past cone at the boundary of confirmed/unconfirmed.
//   2. Order by timestamp (ascending), if the oldest confirmed block > TSC threshold then return false.
//
// This function is optimized through the use of markers and the following assumption:
//   If there's any unconfirmed block >TSC threshold, then the oldest confirmed block will be >TSC threshold, too.
func (t *TipManager) isPastConeTimestampCorrect(blockID BlockID) (timestampValid bool) {
	minSupportedTimestamp := t.tangle.TimeManager.ATT().Add(-t.tangle.Options.TimeSinceConfirmationThreshold)
	timestampValid = true

	// skip TSC check if no block has been confirmed to allow attaching to genesis
	if t.tangle.TimeManager.LastAcceptedBlock().BlockID == EmptyBlockID {
		// if the genesis block is the last confirmed block, then there is no point in performing tangle walk
		// return true so that the network can start issuing blocks when the tangle starts
		return true
	}

	t.tangle.Storage.Block(blockID).Consume(func(block *Block) {
		timestampValid = block.IssuingTime().After(minSupportedTimestamp)
	})

	if !timestampValid {
		return false
	}
	if t.tangle.ConfirmationOracle.IsBlockConfirmed(blockID) {
		// return true if block is confirmed and has valid timestamp
		return true
	}

	markerWalker := walker.New[markersold.Marker](false)
	blockWalker := walker.New[BlockID](false)

	t.processBlock(blockID, blockWalker, markerWalker)
	previousBlockID := blockID
	for markerWalker.HasNext() && timestampValid {
		marker := markerWalker.Next()
		previousBlockID, timestampValid = t.checkMarker(marker, previousBlockID, blockWalker, markerWalker, minSupportedTimestamp)
	}

	for blockWalker.HasNext() && timestampValid {
		timestampValid = t.checkBlock(blockWalker.Next(), blockWalker, minSupportedTimestamp)
	}
	return timestampValid
}

func (t *TipManager) processBlock(blockID BlockID, blockWalker *walker.Walker[BlockID], markerWalker *walker.Walker[markersold.Marker]) {
	t.tangle.Storage.BlockMetadata(blockID).Consume(func(blockMetadata *BlockMetadata) {
		if blockMetadata.StructureDetails() == nil || blockMetadata.StructureDetails().PastMarkers().Size() == 0 {
			// need to walk blocks
			blockWalker.Push(blockID)
			return
		}
		blockMetadata.StructureDetails().PastMarkers().ForEach(func(sequenceID markersold.SequenceID, index markersold.Index) bool {
			if sequenceID == 0 && index == 0 {
				// need to walk blocks
				blockWalker.Push(blockID)
				return false
			}
			pastMarker := markersold.NewMarker(sequenceID, index)
			markerWalker.Push(pastMarker)
			return true
		})
	})
}

func (t *TipManager) checkMarker(marker markersold.Marker, previousBlockID BlockID, blockWalker *walker.Walker[BlockID], markerWalker *walker.Walker[markersold.Marker], minSupportedTimestamp time.Time) (blockID BlockID, timestampValid bool) {
	blockID, blockIssuingTime := t.getMarkerBlock(marker)

	// marker before minSupportedTimestamp
	if blockIssuingTime.Before(minSupportedTimestamp) {
		// marker before minSupportedTimestamp
		if !t.tangle.ConfirmationOracle.IsBlockConfirmed(blockID) {
			// if unconfirmed, then incorrect
			markerWalker.StopWalk()
			return blockID, false
		}
		// if closest past marker is confirmed and before minSupportedTimestamp, then need to walk block past cone of the previously marker block
		blockWalker.Push(previousBlockID)
		return blockID, true
	}
	// confirmed after minSupportedTimestamp
	if t.tangle.ConfirmationOracle.IsBlockConfirmed(blockID) {
		return blockID, true
	}

	// unconfirmed after minSupportedTimestamp

	// check oldest unconfirmed marker time without walking marker DAG
	oldestUnconfirmedMarker := t.getOldestUnconfirmedMarker(marker)

	if timestampValid = t.processMarker(marker, minSupportedTimestamp, oldestUnconfirmedMarker); !timestampValid {
		return
	}

	t.tangle.Booker.MarkersManager.Manager.Sequence(marker.SequenceID()).Consume(func(sequence *markersold.Sequence) {
		// If there is a confirmed marker before the oldest unconfirmed marker, and it's older than minSupportedTimestamp, need to walk block past cone of oldestUnconfirmedMarker.
		if sequence.LowestIndex() < oldestUnconfirmedMarker.Index() {
			confirmedMarkerIdx := t.getPreviousConfirmedIndex(sequence, oldestUnconfirmedMarker.Index())
			if t.isMarkerOldAndConfirmed(markersold.NewMarker(sequence.ID(), confirmedMarkerIdx), minSupportedTimestamp) {
				blockWalker.Push(t.tangle.Booker.MarkersManager.BlockID(oldestUnconfirmedMarker))
			}
		}

		// process markers from different sequences that are referenced by current marker's sequence, i.e., walk the sequence DAG
		referencedMarkers := sequence.ReferencedMarkers(marker.Index())
		referencedMarkers.ForEach(func(sequenceID markersold.SequenceID, index markersold.Index) bool {
			// Ignore Marker(0, 0) as it sometimes occurs in the past marker cone. Marker mysteries.
			if sequenceID == 0 && index == 0 {
				return true
			}

			referencedMarker := markersold.NewMarker(sequenceID, index)
			// if referenced marker is confirmed and older than minSupportedTimestamp, walk unconfirmed block past cone of oldestUnconfirmedMarker
			if t.isMarkerOldAndConfirmed(referencedMarker, minSupportedTimestamp) {
				blockWalker.Push(t.tangle.Booker.MarkersManager.BlockID(oldestUnconfirmedMarker))
				return false
			}
			// otherwise, process the referenced marker
			markerWalker.Push(referencedMarker)
			return true
		})
	})
	return blockID, true
}

// isMarkerOldAndConfirmed check whether previousMarker is confirmed and older than minSupportedTimestamp. It is used to check whether to walk blocks in the past cone of the current marker.
func (t *TipManager) isMarkerOldAndConfirmed(previousMarker markersold.Marker, minSupportedTimestamp time.Time) bool {
	referencedMarkerBlkID, referenceMarkerBlkIssuingTime := t.getMarkerBlock(previousMarker)
	if t.tangle.ConfirmationOracle.IsBlockConfirmed(referencedMarkerBlkID) && referenceMarkerBlkIssuingTime.Before(minSupportedTimestamp) {
		return true
	}
	return false
}

func (t *TipManager) processMarker(pastMarker markersold.Marker, minSupportedTimestamp time.Time, oldestUnconfirmedMarker markersold.Marker) (tscValid bool) {
	// oldest unconfirmed marker is in the future cone of the past marker (same sequence), therefore past marker is confirmed and there is no need to check
	// this condition is covered by other checks but leaving it here just for safety
	if pastMarker.Index() < oldestUnconfirmedMarker.Index() {
		return true
	}
	_, oldestUnconfirmedMarkerBlkIssuingTime := t.getMarkerBlock(oldestUnconfirmedMarker)
	return oldestUnconfirmedMarkerBlkIssuingTime.After(minSupportedTimestamp)
}

func (t *TipManager) checkBlock(blockID BlockID, blockWalker *walker.Walker[BlockID], minSupportedTimestamp time.Time) (timestampValid bool) {
	timestampValid = true

	t.tangle.Storage.Block(blockID).Consume(func(block *Block) {
		// if block is older than TSC then it's incorrect no matter the confirmation status
		if block.IssuingTime().Before(minSupportedTimestamp) {
			timestampValid = false
			blockWalker.StopWalk()
			return
		}

		// if block is younger than TSC and confirmed, then return timestampValid=true
		if t.tangle.ConfirmationOracle.IsBlockConfirmed(blockID) {
			return
		}

		// if block is younger than TSC and not confirmed, walk through strong parents' past cones
		for parentID := range block.ParentsByType(StrongParentType) {
			blockWalker.Push(parentID)
		}
	})
	return timestampValid
}

func (t *TipManager) getMarkerBlock(marker markersold.Marker) (markerBlockID BlockID, markerBlockIssuingTime time.Time) {
	if marker.SequenceID() == 0 && marker.Index() == 0 {
		return EmptyBlockID, time.Unix(epoch.GenesisTime, 0)
	}
	blockID := t.tangle.Booker.MarkersManager.BlockID(marker)
	if blockID == EmptyBlockID {
		panic(fmt.Errorf("failed to retrieve marker block for %s", marker.String()))
	}
	t.tangle.Storage.Block(blockID).Consume(func(block *Block) {
		markerBlockID = block.ID()
		markerBlockIssuingTime = block.IssuingTime()
	})

	return
}

// getOldestUnconfirmedMarker is similar to FirstUnconfirmedMarkerIndex, except it skips any marker gaps an existing marker.
func (t *TipManager) getOldestUnconfirmedMarker(pastMarker markersold.Marker) markersold.Marker {
	unconfirmedMarkerIdx := t.tangle.ConfirmationOracle.FirstUnconfirmedMarkerIndex(pastMarker.SequenceID())

	// skip any gaps in marker indices
	for ; unconfirmedMarkerIdx <= pastMarker.Index(); unconfirmedMarkerIdx++ {
		currentMarker := markersold.NewMarker(pastMarker.SequenceID(), unconfirmedMarkerIdx)

		// Skip if there is no marker at the given index, i.e., the sequence has a gap.
		if t.tangle.Booker.MarkersManager.BlockID(currentMarker) == EmptyBlockID {
			continue
		}
		break
	}

	oldestUnconfirmedMarker := markersold.NewMarker(pastMarker.SequenceID(), unconfirmedMarkerIdx)
	return oldestUnconfirmedMarker
}

func (t *TipManager) getPreviousConfirmedIndex(sequence *markersold.Sequence, markerIndex markersold.Index) markersold.Index {
	// skip any gaps in marker indices
	for ; sequence.LowestIndex() < markerIndex; markerIndex-- {
		currentMarker := markersold.NewMarker(sequence.ID(), markerIndex)

		// Skip if there is no marker at the given index, i.e., the sequence has a gap or marker is not yet confirmed (should not be the case).
		if blkID := t.tangle.Booker.MarkersManager.BlockID(currentMarker); blkID == EmptyBlockID || !t.tangle.ConfirmationOracle.IsBlockConfirmed(blkID) {
			continue
		}
		break
	}
	return markerIndex
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func Width(maxWidth int) options.Option[TipManager] {
	return func(t *TipManager) {
		t.optsWidth = maxWidth
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
