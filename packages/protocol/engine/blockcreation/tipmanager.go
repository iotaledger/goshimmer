package blockcreation

import (
	"time"

	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/randommap"
	"github.com/iotaledger/hive.go/core/generics/walker"

	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/congestioncontrol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/virtualvoting"
)

// region TipManager ///////////////////////////////////////////////////////////////////////////////////////////////////

type TipManager struct {
	tangle              *tangle.Tangle
	acceptanceGadget    *acceptance.Gadget
	congestionControl   *congestioncontrol.CongestionControl
	engine              *engine.Engine
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
	// TODO: wire up events
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
	// Check if any children that are confirmed or scheduled and return if true, to guarantee that the parents are not added to the tipset after its children.
	if t.checkChildren(block) {
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
func (t *TipManager) checkChildren(block *virtualvoting.Block) (anyScheduledOrAccepted bool) {
	for _, child := range block.Children() {
		if childBlock, exists := t.acceptanceGadget.Block(child.ID()); exists {
			if childBlock.Accepted() {
				return true
			}
		}

		if childBlock, exists := t.congestionControl.Block(child.ID()); exists {
			if childBlock.IsScheduled() {
				return true
			}
		}
	}

	return false
}

func (t *TipManager) removeStrongParents(block *virtualvoting.Block) {
	block.ForEachParent(func(parent models.Parent) {
		// TODO: what to do with this?
		// We do not want to remove the tip if it is the last one representing a pending conflict.
		// if t.isLastTipForConflict(parentBlockID) {
		// 	return true
		// }
		if parentBlock, exists := t.tangle.VirtualVoting.Block(parent.ID); exists {
			t.deleteTip(parentBlock)
		}
	})
}

// Tips returns count number of tips, maximum MaxParentsCount.
func (t *TipManager) Tips(countParents int) (parents virtualvoting.Blocks) {
	if countParents > models.MaxParentsCount {
		countParents = models.MaxParentsCount
	}
	if countParents < models.MinParentsCount {
		countParents = models.MinParentsCount
	}

	return t.selectTips(countParents)
}

func (t *TipManager) selectTips(count int) (parents virtualvoting.Blocks) {
	parents = virtualvoting.NewBlocks()

	tips := t.tips.RandomUniqueEntries(count)

	// only add genesis if no tips are available
	if len(tips) == 0 {
		// TODO: do we add one/multiple random solid entry point?
		// parents.Add(EmptyBlockID)
		return parents
	}

	// at least one tip is returned
	for _, tip := range tips {
		if t.isPastConeTimestampCorrect(tip) {
			parents.Add(tip)
		}
	}

	// TODO: should we retry if no valid tip is left?

	return parents
}

// AllTips returns a list of all tips that are stored in the TipManger.
func (t *TipManager) AllTips() []*virtualvoting.Block {
	return t.tips.Keys()
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
func (t *TipManager) isPastConeTimestampCorrect(block *virtualvoting.Block) (timestampValid bool) {
	minSupportedTimestamp := t.engine.Clock.AcceptedTime().Add(-t.tangle.Options.TimeSinceConfirmationThreshold)
	timestampValid = true

	// TODO: skip TSC check if no block has been confirmed to allow attaching to genesis
	// if t.engine.Clock.AcceptedTime().BlockID == EmptyBlockID {
	// 	// if the genesis block is the last confirmed block, then there is no point in performing tangle walk
	// 	// return true so that the network can start issuing blocks when the tangle starts
	// 	return true
	// }

	if timestampValid = block.IssuingTime().After(minSupportedTimestamp); !timestampValid {
		return false
	}
	if acceptanceBlock, exists := t.acceptanceGadget.Block(block.ID()); exists && acceptanceBlock.Accepted() {
		// return true if block is accepted and has valid timestamp
		return true
	}

	markerWalker := walker.New[markers.Marker](false)
	blockWalker := walker.New[models.BlockID](false)

	t.processBlock(block, blockWalker, markerWalker)
	previousBlockID := blockID
	for markerWalker.HasNext() {
		marker := markerWalker.Next()
		previousBlockID, timestampValid = t.checkMarker(marker, previousBlockID, blockWalker, markerWalker, minSupportedTimestamp)
		if !timestampValid {
			return false
		}
	}

	for blockWalker.HasNext() {
		timestampValid = t.checkBlock(blockWalker.Next(), blockWalker, minSupportedTimestamp)
		if !timestampValid {
			return false
		}
	}
	return true
}

func (t *TipManager) processBlock(block *virtualvoting.Block, blockWalker *walker.Walker[models.BlockID], markerWalker *walker.Walker[markers.Marker]) {
	if block.StructureDetails() == nil || block.StructureDetails().PastMarkers().Size() == 0 {
		// need to walk blocks
		blockWalker.Push(blockID) // TODO: take blocks or IDs here?
		return
	}

	block.StructureDetails().PastMarkers().ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
		if sequenceID == 0 && index == 0 {
			// need to walk blocks
			blockWalker.Push(blockID)
			return false
		}

		markerWalker.Push(markers.NewMarker(sequenceID, index))
		return true
	})
}

func (t *TipManager) checkMarker(marker markers.Marker, previousBlockID models.BlockID, blockWalker *walker.Walker[models.BlockID], markerWalker *walker.Walker[markers.Marker], minSupportedTimestamp time.Time) (blockID models.BlockID, timestampValid bool) {
	block, exists := t.tangle.Booker.BlockFromMarker(marker)
	if !exists {
		// TODO: what to do if it doesn't exist?
	}

	// marker before minSupportedTimestamp
	if block.IssuingTime().Before(minSupportedTimestamp) {
		// marker before minSupportedTimestamp
		if !t.acceptanceGadget.IsMarkerAccepted(marker) {
			// if not accepted, then incorrect
			markerWalker.StopWalk()
			return blockID, false
		}
		// if closest past marker is confirmed and before minSupportedTimestamp, then need to walk block past cone of the previously marker block
		blockWalker.Push(previousBlockID)
		return blockID, true
	}
	// accepted after minSupportedTimestamp
	if t.acceptanceGadget.IsMarkerAccepted(marker) {
		return blockID, true
	}

	// unconfirmed after minSupportedTimestamp

	// check oldest unconfirmed marker time without walking marker DAG
	oldestUnconfirmedMarker := markers.NewMarker(marker.SequenceID(), t.acceptanceGadget.FirstUnacceptedIndex(marker.SequenceID()))

	if timestampValid = t.processMarker(marker, minSupportedTimestamp, oldestUnconfirmedMarker); !timestampValid {
		return
	}

	sequence, exists := t.tangle.Booker.Sequence(marker.SequenceID())
	if !exists {
		// TODO: what to do if it doesn't exist?
	}

	// If there is a confirmed marker before the oldest unconfirmed marker, and it's older than minSupportedTimestamp, need to walk block past cone of oldestUnconfirmedMarker.
	if sequence.LowestIndex() < oldestUnconfirmedMarker.Index() {
		confirmedMarkerIdx := t.getPreviousConfirmedIndex(sequence, oldestUnconfirmedMarker.Index())
		if t.isMarkerOldAndConfirmed(markers.NewMarker(sequence.ID(), confirmedMarkerIdx), minSupportedTimestamp) {
			blockWalker.Push(t.tangle.Booker.MarkersManager.BlockID(oldestUnconfirmedMarker))
		}
	}

	// process markers from different sequences that are referenced by current marker's sequence, i.e., walk the sequence DAG
	sequence.ReferencedMarkers(marker.Index()).ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
		// Ignore Marker(0, 0) as it sometimes occurs in the past marker cone. Marker mysteries.
		if sequenceID == 0 && index == 0 {
			return true
		}

		referencedMarker := markers.NewMarker(sequenceID, index)
		// if referenced marker is confirmed and older than minSupportedTimestamp, walk unconfirmed block past cone of oldestUnconfirmedMarker
		if t.isMarkerOldAndConfirmed(referencedMarker, minSupportedTimestamp) {
			blockWalker.Push(t.tangle.Booker.MarkersManager.BlockID(oldestUnconfirmedMarker))
			return false
		}
		// otherwise, process the referenced marker
		markerWalker.Push(referencedMarker)
		return true
	})
	return blockID, true
}

// isMarkerOldAndConfirmed check whether previousMarker is confirmed and older than minSupportedTimestamp. It is used to check whether to walk blocks in the past cone of the current marker.
func (t *TipManager) isMarkerOldAndConfirmed(previousMarker markers.Marker, minSupportedTimestamp time.Time) bool {
	block, exists := t.tangle.Booker.BlockFromMarker(previousMarker)
	if !exists {
		// TODO: what to do if it doesn't exist?
	}
	if t.acceptanceGadget.IsMarkerAccepted(previousMarker) && block.IssuingTime().Before(minSupportedTimestamp) {
		return true
	}

	return false
}

func (t *TipManager) processMarker(pastMarker markers.Marker, minSupportedTimestamp time.Time, oldestUnconfirmedMarker markers.Marker) (tscValid bool) {
	// oldest unconfirmed marker is in the future cone of the past marker (same sequence), therefore past marker is confirmed and there is no need to check
	// this condition is covered by other checks but leaving it here just for safety
	if pastMarker.Index() < oldestUnconfirmedMarker.Index() {
		return true
	}

	block, exists := t.tangle.Booker.BlockFromMarker(oldestUnconfirmedMarker)
	if !exists {
		// TODO: what to do if it doesn't exist?
	}

	return block.IssuingTime().After(minSupportedTimestamp)
}

func (t *TipManager) checkBlock(blockID models.BlockID, blockWalker *walker.Walker[models.BlockID], minSupportedTimestamp time.Time) (timestampValid bool) {
	block, exists := t.tangle.VirtualVoting.Block(blockID)
	if !exists {
		// TODO: what to do if it doesn't exist?
	}

	// if block is older than TSC then it's incorrect no matter the confirmation status
	if block.IssuingTime().Before(minSupportedTimestamp) {
		blockWalker.StopWalk()
		return false
	}

	// if block is younger than TSC and accepted, then return timestampValid=true
	if acceptanceBlock, exists := t.acceptanceGadget.Block(block.ID()); exists && acceptanceBlock.Accepted() {
		return true
	}

	// if block is younger than TSC and not confirmed, walk through strong parents' past cones
	for parentID := range block.ParentsByType(models.StrongParentType) {
		blockWalker.Push(parentID)
	}

	return true
}

// getOldestUnconfirmedMarker is similar to FirstUnconfirmedMarkerIndex, except it skips any marker gaps an existing marker.
func (t *TipManager) getOldestUnconfirmedMarker(pastMarker markers.Marker) markers.Marker {
	unconfirmedMarkerIdx := t.tangle.ConfirmationOracle.FirstUnconfirmedMarkerIndex(pastMarker.SequenceID())

	// skip any gaps in marker indices
	for ; unconfirmedMarkerIdx <= pastMarker.Index(); unconfirmedMarkerIdx++ {
		currentMarker := markers.NewMarker(pastMarker.SequenceID(), unconfirmedMarkerIdx)

		// Skip if there is no marker at the given index, i.e., the sequence has a gap.
		if t.tangle.Booker.MarkersManager.BlockID(currentMarker) == EmptyBlockID {
			continue
		}
		break
	}

	oldestUnconfirmedMarker := markers.NewMarker(pastMarker.SequenceID(), unconfirmedMarkerIdx)
	return oldestUnconfirmedMarker
}

func (t *TipManager) getPreviousConfirmedIndex(sequence *markers.Sequence, markerIndex markers.Index) markers.Index {
	// skip any gaps in marker indices
	for ; sequence.LowestIndex() < markerIndex; markerIndex-- {
		currentMarker := markers.NewMarker(sequence.ID(), markerIndex)

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
