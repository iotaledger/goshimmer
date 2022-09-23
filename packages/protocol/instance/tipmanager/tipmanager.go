package tipmanager

import (
	"time"

	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/randommap"
	"github.com/iotaledger/hive.go/core/generics/walker"

	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type acceptanceGadget interface {
	IsBlockAccepted(blockID models.BlockID) (accepted bool)
	IsMarkerAccepted(marker markers.Marker) (accepted bool)
	FirstUnacceptedIndex(sequenceID markers.SequenceID) (firstUnacceptedIndex markers.Index)
}

// TimeRetrieverFunc is a function type to retrieve the time.
type timeRetrieverFunc func() time.Time

type blockRetrieverFunc func(id models.BlockID) (block *scheduler.Block, exists bool)

// region TipManager ///////////////////////////////////////////////////////////////////////////////////////////////////

type TipManager struct {
	Events *Events

	tangle             *tangle.Tangle
	acceptanceGadget   acceptanceGadget
	blockRetrieverFunc blockRetrieverFunc
	timeRetrieverFunc  timeRetrieverFunc
	genesisTime        time.Time

	tips *randommap.RandomMap[*scheduler.Block, *scheduler.Block]
	// TODO: reintroduce TipsConflictTracker
	// tipsConflictTracker *TipsConflictTracker

	optsTimeSinceConfirmationThreshold time.Duration
	optsWidth                          int
}

func New(tangle *tangle.Tangle, gadget acceptanceGadget, blockRetriever blockRetrieverFunc, timeRetriever timeRetrieverFunc, genesisTime time.Time, opts ...options.Option[TipManager]) *TipManager {
	return options.Apply(&TipManager{
		Events: NewEvents(),

		tangle:             tangle,
		acceptanceGadget:   gadget,
		blockRetrieverFunc: blockRetriever,
		timeRetrieverFunc:  timeRetriever,
		genesisTime:        genesisTime,

		tips: randommap.New[*scheduler.Block, *scheduler.Block](),
		// TODO: reintroduce TipsConflictTracker
		// tipsConflictTracker: NewTipsConflictTracker(tangle),

		optsTimeSinceConfirmationThreshold: time.Minute,
		optsWidth:                          0,
	}, opts)
}

func (t *TipManager) AddTip(block *scheduler.Block) {
	// Check if any children that are accepted or scheduled and return if true, to guarantee that parents are not added
	// to the tipset after their children.
	if t.checkMonotonicity(block) {
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
	t.RemoveStrongParents(block.ModelsBlock)
}

func (t *TipManager) addTip(block *scheduler.Block) (added bool) {
	if t.tips.Set(block, block) {
		// t.tipsConflictTracker.AddTip(block)
		t.Events.TipAdded.Trigger(block)
		return true
	}

	return false
}

func (t *TipManager) DeleteTip(block *scheduler.Block) (deleted bool) {
	if _, deleted = t.tips.Delete(block); deleted {
		// t.tipsConflictTracker.RemoveTip(block)
		t.Events.TipRemoved.Trigger(block)
	}
	return
}

// checkMonotonicity returns true if the block has any accepted or scheduled child.
func (t *TipManager) checkMonotonicity(block *scheduler.Block) (anyScheduledOrAccepted bool) {
	for _, child := range block.Children() {
		if t.acceptanceGadget.IsBlockAccepted(child.ID()) {
			return true
		}

		if childBlock, exists := t.blockRetrieverFunc(child.ID()); exists {
			if childBlock.IsScheduled() {
				return true
			}
		}
	}

	return false
}

func (t *TipManager) RemoveStrongParents(block *models.Block) {
	block.ForEachParent(func(parent models.Parent) {
		// TODO: reintroduce TipsConflictTracker
		// We do not want to remove the tip if it is the last one representing a pending conflict.
		// if t.isLastTipForConflict(parentBlockID) {
		// 	return true
		// }
		if parentBlock, exists := t.blockRetrieverFunc(parent.ID); exists {
			t.DeleteTip(parentBlock)
		}
	})
}

// Tips returns count number of tips, maximum MaxParentsCount.
func (t *TipManager) Tips(countParents int) (parents models.BlockIDs) {
	if countParents > models.MaxParentsCount {
		countParents = models.MaxParentsCount
	}
	if countParents < models.MinParentsCount {
		countParents = models.MinParentsCount
	}

	return t.selectTips(countParents)
}

func (t *TipManager) selectTips(count int) (parents models.BlockIDs) {
	parents = models.NewBlockIDs()

	tips := t.tips.RandomUniqueEntries(count)

	// only add genesis if no tips are available
	if len(tips) == 0 {
		for i, it := 0, t.tangle.EvictionManager.RootBlocks().Iterator(); it.HasNext() && i < count; i++ {
			blockID := it.Next()
			if block, exists := t.blockRetrieverFunc(blockID); exists {
				parents.Add(block.ID())
			}
		}
		return parents
	}

	// at least one tip is returned
	for _, tip := range tips {
		if t.isPastConeTimestampCorrect(tip) {
			parents.Add(tip.ID())
		}
	}

	// TODO: should we retry if no valid tip is left?

	return parents
}

// AllTips returns a list of all tips that are stored in the TipManger.
func (t *TipManager) AllTips() []*scheduler.Block {
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
//   1. Collect all accepted blocks in the tip's past cone at the boundary of accepted/unaccapted.
//   2. Order by timestamp (ascending), if the oldest accepted block > TSC threshold then return false.
//
// This function is optimized through the use of markers and the following assumption:
//   If there's any unaccepted block >TSC threshold, then the oldest accepted block will be >TSC threshold, too.
func (t *TipManager) isPastConeTimestampCorrect(block *scheduler.Block) (timestampValid bool) {
	minSupportedTimestamp := t.timeRetrieverFunc().Add(-t.optsTimeSinceConfirmationThreshold)

	if t.timeRetrieverFunc() == t.genesisTime {
		// if the genesis block is the last accepted block, then there is no point in performing tangle walk
		// return true so that the network can start issuing blocks when the tangle starts
		return true
	}

	if block.IssuingTime().Before(minSupportedTimestamp) {
		return false
	}

	if t.acceptanceGadget.IsBlockAccepted(block.ID()) {
		// return true if block is accepted and has valid timestamp
		return true
	}

	markerWalker := walker.New[markers.Marker](false)
	blockWalker := walker.New[*scheduler.Block](false)

	t.processInitialBlock(block, blockWalker, markerWalker)

	previousBlock := block
	for markerWalker.HasNext() {
		previousBlock, timestampValid = t.checkMarker(markerWalker.Next(), previousBlock, blockWalker, markerWalker, minSupportedTimestamp)
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

func (t *TipManager) processInitialBlock(block *scheduler.Block, blockWalker *walker.Walker[*scheduler.Block], markerWalker *walker.Walker[markers.Marker]) {
	if block.StructureDetails() == nil || block.StructureDetails().PastMarkers().Size() == 0 {
		// need to walk blocks
		blockWalker.Push(block)
		return
	}

	block.StructureDetails().PastMarkers().ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
		if sequenceID == 0 && index == 0 {
			// need to walk blocks
			blockWalker.Push(block)
			return false
		}

		markerWalker.Push(markers.NewMarker(sequenceID, index))
		return true
	})
}

func (t *TipManager) checkMarker(marker markers.Marker, previousBlock *scheduler.Block, blockWalker *walker.Walker[*scheduler.Block], markerWalker *walker.Walker[markers.Marker], minSupportedTimestamp time.Time) (block *scheduler.Block, timestampValid bool) {
	block, exists := t.schedulerBlockFromMarker(marker)
	if !exists {
		return nil, false
	}

	// marker before minSupportedTimestamp
	if block.IssuingTime().Before(minSupportedTimestamp) {
		// marker before minSupportedTimestamp
		if !t.acceptanceGadget.IsMarkerAccepted(marker) {
			// if not accepted, then incorrect
			markerWalker.StopWalk()
			return nil, false
		}
		// if closest past marker is accepted and before minSupportedTimestamp, then need to walk block past cone of the previously marker block
		blockWalker.Push(previousBlock)
		return block, true
	}
	// accepted after minSupportedTimestamp
	if t.acceptanceGadget.IsMarkerAccepted(marker) {
		return block, true
	}

	// unaccepted after minSupportedTimestamp

	// check oldest unaccepted marker time without walking sequence DAG
	firstUnacceptedMarker := t.firstUnacceptedMarker(marker)
	if timestampValid = t.processMarker(marker, minSupportedTimestamp, firstUnacceptedMarker); !timestampValid {
		return nil, false
	}

	sequence, exists := t.tangle.Booker.Sequence(marker.SequenceID())
	if !exists {
		return nil, false
	}

	// If there is a accepted marker before the oldest unaccepted marker, and it's older than minSupportedTimestamp, need to walk block past cone of firstUnacceptedMarker.
	if sequence.LowestIndex() < firstUnacceptedMarker.Index() {
		acceptedMarker, blockOfMarker := t.previousMarkerWithBlock(sequence, firstUnacceptedMarker.Index())
		if t.isMarkerOldAndAccepted(acceptedMarker, minSupportedTimestamp) {
			blockWalker.Push(blockOfMarker)
		}
	}

	// process markers from different sequences that are referenced by current marker's sequence, i.e., walk the sequence DAG
	sequence.ReferencedMarkers(marker.Index()).ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
		// Ignore Marker(0, 0) as it sometimes occurs in the past marker cone. Marker mysteries.
		if sequenceID == 0 && index == 0 {
			return true
		}

		referencedMarker := markers.NewMarker(sequenceID, index)

		// if referenced marker is accepted and older than minSupportedTimestamp, walk unaccepted block past cone of firstUnacceptedMarker
		if t.isMarkerOldAndAccepted(referencedMarker, minSupportedTimestamp) {
			referencedMarkerBlock, referencedMarkerBlockExists := t.schedulerBlockFromMarker(marker)
			if !referencedMarkerBlockExists {
				return false
			}
			blockWalker.Push(referencedMarkerBlock)
			return false
		}

		// otherwise, process the referenced marker
		markerWalker.Push(referencedMarker)
		return true
	})

	return block, true
}

func (t *TipManager) schedulerBlockFromMarker(marker markers.Marker) (block *scheduler.Block, exists bool) {
	bookerBlock, exists := t.tangle.Booker.BlockFromMarker(marker)
	if !exists {
		return nil, false
	}
	block, exists = t.blockRetrieverFunc(bookerBlock.ID())
	if !exists {
		return nil, false
	}
	return block, true
}

// isMarkerOldAndAccepted check whether previousMarker is accepted and older than minSupportedTimestamp. It is used to check whether to walk blocks in the past cone of the current marker.
func (t *TipManager) isMarkerOldAndAccepted(previousMarker markers.Marker, minSupportedTimestamp time.Time) bool {
	block, exists := t.tangle.Booker.BlockFromMarker(previousMarker)
	if !exists {
		return false
	}
	if t.acceptanceGadget.IsMarkerAccepted(previousMarker) && block.IssuingTime().Before(minSupportedTimestamp) {
		return true
	}

	return false
}

func (t *TipManager) processMarker(pastMarker markers.Marker, minSupportedTimestamp time.Time, firstUnacceptedMarker markers.Marker) (tscValid bool) {
	// oldest unaccepted marker is in the future cone of the past marker (same sequence), therefore past marker is accepted and there is no need to check
	// this condition is covered by other checks but leaving it here just for safety
	if pastMarker.Index() < firstUnacceptedMarker.Index() {
		return true
	}

	block, exists := t.tangle.Booker.BlockFromMarker(firstUnacceptedMarker)
	if !exists {
		return false
	}

	return block.IssuingTime().After(minSupportedTimestamp)
}

func (t *TipManager) checkBlock(block *scheduler.Block, blockWalker *walker.Walker[*scheduler.Block], minSupportedTimestamp time.Time) (timestampValid bool) {
	// if block is older than TSC then it's incorrect no matter the confirmation status
	if block.IssuingTime().Before(minSupportedTimestamp) {
		blockWalker.StopWalk()
		return false
	}

	// if block is younger than TSC and accepted, then return timestampValid=true
	if t.acceptanceGadget.IsBlockAccepted(block.ID()) {
		return true
	}

	// if block is younger than TSC and not accepted, walk through strong parents' past cones
	for parentID := range block.ParentsByType(models.StrongParentType) {
		parentBlock, exists := t.blockRetrieverFunc(parentID)
		if exists {
			blockWalker.Push(parentBlock)
		}
	}

	return true
}

// firstUnacceptedMarker is similar to acceptance.FirstUnacceptedIndex, except it skips any marker gaps and returns
// an existing marker.
func (t *TipManager) firstUnacceptedMarker(pastMarker markers.Marker) (firstUnacceptedMarker markers.Marker) {
	firstUnacceptedIndex := t.acceptanceGadget.FirstUnacceptedIndex(pastMarker.SequenceID())
	// skip any gaps in marker indices
	for ; firstUnacceptedIndex <= pastMarker.Index(); firstUnacceptedIndex++ {
		firstUnacceptedMarker = markers.NewMarker(pastMarker.SequenceID(), firstUnacceptedIndex)

		// Skip if there is no marker at the given index, i.e., the sequence has a gap.
		if _, exists := t.tangle.Booker.BlockFromMarker(firstUnacceptedMarker); !exists {
			continue
		}

		break
	}

	return firstUnacceptedMarker
}

func (t *TipManager) previousMarkerWithBlock(sequence *markers.Sequence, markerIndex markers.Index) (previousMarker markers.Marker, block *scheduler.Block) {
	// skip any gaps in marker indices and start from marker below current one.
	for markerIndex--; sequence.LowestIndex() < markerIndex; markerIndex-- {
		previousMarker = markers.NewMarker(sequence.ID(), markerIndex)

		// Skip if there is no marker at the given index, i.e., the sequence has a gap.
		if block, exists := t.schedulerBlockFromMarker(previousMarker); exists {
			return previousMarker, block
		}
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithTimeSinceConfirmationThreshold(timeSinceConfirmationThreshold time.Duration) options.Option[TipManager] {
	return func(o *TipManager) {
		o.optsTimeSinceConfirmationThreshold = timeSinceConfirmationThreshold
	}
}

func WithWidth(maxWidth int) options.Option[TipManager] {
	return func(t *TipManager) {
		t.optsWidth = maxWidth
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
