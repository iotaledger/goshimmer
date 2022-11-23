package tipmanager

import (
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/randommap"
	"github.com/iotaledger/hive.go/core/generics/walker"

	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type acceptanceGadget interface {
	IsBlockAccepted(blockID models.BlockID) (accepted bool)
	IsMarkerAccepted(marker markers.Marker) (accepted bool)
	FirstUnacceptedIndex(sequenceID markers.SequenceID) (firstUnacceptedIndex markers.Index)
}

type blockRetrieverFunc func(id models.BlockID) (block *scheduler.Block, exists bool)

// region TipManager ///////////////////////////////////////////////////////////////////////////////////////////////////

type TipManager struct {
	Events *Events

	engine                *engine.Engine
	blockAcceptanceGadget acceptanceGadget

	schedulerBlockRetrieverFunc blockRetrieverFunc

	tips *randommap.RandomMap[*scheduler.Block, *scheduler.Block]
	// TODO: reintroduce TipsConflictTracker
	// tipsConflictTracker *TipsConflictTracker

	optsTimeSinceConfirmationThreshold time.Duration
	optsWidth                          int
}

// New creates a new TipManager.
func New(schedulerBlockRetrieverFunc blockRetrieverFunc, opts ...options.Option[TipManager]) *TipManager {
	return options.Apply(&TipManager{
		Events: NewEvents(),

		schedulerBlockRetrieverFunc: schedulerBlockRetrieverFunc,

		tips: randommap.New[*scheduler.Block, *scheduler.Block](),
		// TODO: reintroduce TipsConflictTracker
		// tipsConflictTracker: NewTipsConflictTracker(tangle),

		optsTimeSinceConfirmationThreshold: time.Minute,
		optsWidth:                          0,
	}, opts)
}

func (t *TipManager) ActivateEngine(engine *engine.Engine) {
	t.tips = randommap.New[*scheduler.Block, *scheduler.Block]()
	t.engine = engine
	t.blockAcceptanceGadget = engine.Consensus.BlockGadget
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
		if child.IsOrphaned() {
			continue
		}

		if t.blockAcceptanceGadget.IsBlockAccepted(child.ID()) {
			return true
		}

		if childBlock, exists := t.schedulerBlockRetrieverFunc(child.ID()); exists {
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
		if parentBlock, exists := t.schedulerBlockRetrieverFunc(parent.ID); exists {
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
	for {
		tips := t.tips.RandomUniqueEntries(count)

		// only add genesis if no tips are available
		if len(tips) == 0 {
			rootBlock := t.engine.EvictionState.LatestRootBlock()
			fmt.Println("selecting root block because tip pool empty:", rootBlock)

			return parents.Add(rootBlock)
		}

		// at least one tip is returned
		for _, tip := range tips {
			if t.isPastConeTimestampCorrect(tip.Block.Block) {
				parents.Add(tip.ID())
			} else {
				fmt.Printf("cannot select tip due to TSC condition tip issuing time (%s), time (%s), min supported time (%s), block id (%s), tip pool size (%d), scheduled: (%t), orphaned: (%t), accepted: (%t)\n",
					tip.IssuingTime(),
					t.engine.Clock.AcceptedTime(),
					t.engine.Clock.AcceptedTime().Add(-t.optsTimeSinceConfirmationThreshold),
					tip.ID().Base58(),
					t.tips.Size(),
					tip.IsScheduled(),
					tip.IsOrphaned(),
					t.engine.Consensus.BlockGadget.IsBlockAccepted(tip.ID()),
				)
				// tip.ForEachParent(func(parent models.Parent) {
				// 	fmt.Println("parent block id", parent.ID.Base58())
				// 	if parentBlock, exists := t.engine.(parent.ID); exists {
				// 		fmt.Println(parentBlock.IssuingTime(), parentBlock.IsScheduled())
				// 	}
				// 	if t.acceptanceGadget.IsBlockAccepted(parent.ID) {
				// 		fmt.Println("parent block accepted")
				// 	}
				// })
				t.DeleteTip(tip)
				if t.tips.Size() == 0 {
					fmt.Println(">> deleted last TIP because it doesn't pass tsc check!", tip.ID())
				}
			}
		}

		if len(parents) > 0 {
			return parents
		}
	}
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

type markerPreviousBlockPair struct {
	Marker        markers.Marker
	PreviousBlock *booker.Block
}

// isPastConeTimestampCorrect performs the TSC check for the given tip.
// Conceptually, this involves the following steps:
//  1. Collect all accepted blocks in the tip's past cone at the boundary of accepted/unaccapted.
//  2. Order by timestamp (ascending), if the oldest accepted block > TSC threshold then return false.
//
// This function is optimized through the use of markers and the following assumption:
//
//	If there's any unaccepted block >TSC threshold, then the oldest accepted block will be >TSC threshold, too.
func (t *TipManager) isPastConeTimestampCorrect(block *booker.Block) (timestampValid bool) {
	minSupportedTimestamp := t.engine.Clock.AcceptedTime().Add(-t.optsTimeSinceConfirmationThreshold)

	if !t.engine.IsBootstrapped() {
		// If the node is not bootstrapped we do not have a valid timestamp to compare against.
		// In any case, a node should never perform tip selection if not bootstrapped (via issuer plugin).
		return true
	}

	if block.IssuingTime().Before(minSupportedTimestamp) {
		fmt.Println("block before min supported timestamp", block.ID(), "issuing time(", block.IssuingTime().String(), "), minsupportedtime(", minSupportedTimestamp, ")")
		return false
	}

	if t.blockAcceptanceGadget.IsBlockAccepted(block.ID()) {
		// return true if block is accepted and has valid timestamp
		return true
	}

	markerWalker := walker.New[markerPreviousBlockPair](false)
	blockWalker := walker.New[*booker.Block](false)

	processInitialBlock(block, blockWalker, markerWalker)

	for markerWalker.HasNext() {
		marker := markerWalker.Next()
		timestampValid = t.checkPair(marker, blockWalker, markerWalker, minSupportedTimestamp)
		if !timestampValid {
			markerBlock, exists := t.engine.Tangle.Booker.BlockFromMarker(marker.Marker)
			if !exists {
				fmt.Println("marker does not exist?", marker)
			} else {
				fmt.Println("walked on marker ", marker, markerBlock.ID(), " before min supported timestamp issuing time(", block.IssuingTime().String(), "), minsupportedtime(", minSupportedTimestamp, ")")
			}
			return false
		}
	}

	for blockWalker.HasNext() {
		blockW := blockWalker.Next()
		timestampValid = t.checkBlock(blockW, blockWalker, minSupportedTimestamp)
		if !timestampValid {
			fmt.Println("walked on block before min supported timestamp", blockW.ID(), "issuing time(", blockW.IssuingTime().String(), "), minsupportedtime(", minSupportedTimestamp, ")")
			return false
		}
	}
	return true
}

func processInitialBlock(block *booker.Block, blockWalker *walker.Walker[*booker.Block], markerWalker *walker.Walker[markerPreviousBlockPair]) {
	if block.StructureDetails() == nil || block.StructureDetails().PastMarkers().Size() == 0 {
		// need to walk blocks
		// should never go here
		blockWalker.Push(block)
		return
	}

	block.StructureDetails().PastMarkers().ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
		if index == 0 {
			// need to walk blocks
			// should never go here
			blockWalker.Push(block)
			return false
		}

		markerWalker.Push(markerPreviousBlockPair{Marker: markers.NewMarker(sequenceID, index), PreviousBlock: block})
		return true
	})
}

func (t *TipManager) checkPair(pair markerPreviousBlockPair, blockWalker *walker.Walker[*booker.Block], markerWalker *walker.Walker[markerPreviousBlockPair], minSupportedTimestamp time.Time) (timestampValid bool) {
	block, exists := t.engine.Tangle.BlockFromMarker(pair.Marker)
	if !exists {
		return false
	}

	// marker before minSupportedTimestamp
	if block.IssuingTime().Before(minSupportedTimestamp) {
		// marker before minSupportedTimestamp
		if !t.blockAcceptanceGadget.IsMarkerAccepted(pair.Marker) {
			// if not accepted, then incorrect
			return false
		}
		// if closest past marker is accepted and before minSupportedTimestamp, then need to walk block past cone of the previously marker block
		blockWalker.Push(pair.PreviousBlock)
		return true
	}
	// accepted after minSupportedTimestamp
	if t.blockAcceptanceGadget.IsMarkerAccepted(pair.Marker) {
		return true
	}

	// unaccepted after minSupportedTimestamp
	sequence, exists := t.engine.Tangle.Booker.Sequence(pair.Marker.SequenceID())
	if !exists {
		return false
	}

	// check oldest unaccepted marker time without walking sequence DAG
	firstUnacceptedMarker := findFirstUnacceptedMarker(sequence, t.engine.Tangle.BlockFromMarker, t.engine.FirstUnacceptedMarker)
	if timestampValid = t.processMarker(pair.Marker, minSupportedTimestamp, firstUnacceptedMarker); !timestampValid {
		return false
	}

	// If there is an accepted marker before the oldest unaccepted marker, and it's older than minSupportedTimestamp, need to walk block past cone of firstUnacceptedMarker.
	if sequence.LowestIndex() < firstUnacceptedMarker.Index() {
		acceptedMarker := previousMarker(sequence, firstUnacceptedMarker.Index(), t.engine.Tangle.BlockFromMarker)

		if t.isMarkerOldAndAccepted(acceptedMarker, minSupportedTimestamp) {
			firstUnacceptedMarkerBlock, blockExists := t.engine.Tangle.Booker.BlockFromMarker(firstUnacceptedMarker)
			if !blockExists {
				return false
			}
			blockWalker.Push(firstUnacceptedMarkerBlock)
		}
	}

	// process markers from different sequences that are referenced by current marker's sequence, i.e., walk the sequence DAG
	sequence.ReferencedMarkers(pair.Marker.Index()).ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
		// Ignore Marker(_, 0) as it sometimes occurs in the past marker cone. Marker mysteries.
		// This should not be necessary anymore?
		if index == 0 {
			fmt.Println("ignore Marker(_, 0) during tip fishing")
			return true
		}

		referencedMarker := markers.NewMarker(sequenceID, index)

		// if referenced marker is accepted and older than minSupportedTimestamp, walk unaccepted block past cone of firstUnacceptedMarker
		if t.isMarkerOldAndAccepted(referencedMarker, minSupportedTimestamp) {
			referencedMarkerBlock, referencedMarkerBlockExists := t.engine.Tangle.BlockFromMarker(pair.Marker)
			if !referencedMarkerBlockExists {
				return false
			}
			blockWalker.Push(referencedMarkerBlock)
			return false
		}

		// otherwise, process the referenced marker
		markerWalker.Push(markerPreviousBlockPair{Marker: referencedMarker, PreviousBlock: block})
		return true
	})

	return true
}

// isMarkerOldAndAccepted check whether previousMarker is accepted and older than minSupportedTimestamp. It is used to check whether to walk blocks in the past cone of the current marker.
func (t *TipManager) isMarkerOldAndAccepted(previousMarker markers.Marker, minSupportedTimestamp time.Time) bool {
	// check if previous marker belongs to a root block,
	if previousMarker.Index() == 0 {
		return true
	}

	block, exists := t.engine.Tangle.Booker.BlockFromMarker(previousMarker)
	if !exists {
		return false
	}

	if t.blockAcceptanceGadget.IsMarkerAccepted(previousMarker) && block.IssuingTime().Before(minSupportedTimestamp) {
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

	block, exists := t.engine.Tangle.Booker.BlockFromMarker(firstUnacceptedMarker)
	if !exists {
		return false
	}

	return block.IssuingTime().After(minSupportedTimestamp)
}

func (t *TipManager) checkBlock(block *booker.Block, blockWalker *walker.Walker[*booker.Block], minSupportedTimestamp time.Time) (timestampValid bool) {
	// if block is older than TSC then it's incorrect no matter the confirmation status
	if block.IssuingTime().Before(minSupportedTimestamp) {
		return false
	}

	// if block is younger than TSC and accepted, then return timestampValid=true
	if t.blockAcceptanceGadget.IsBlockAccepted(block.ID()) {
		return true
	}

	if block.IsOrphaned() {
		return false
	}

	// if block is younger than TSC and not accepted, walk through strong parents' past cones
	for parentID := range block.ParentsByType(models.StrongParentType) {
		parentBlock, exists := t.schedulerBlockRetrieverFunc(parentID)
		if !exists {
			return false
		}
		blockWalker.Push(parentBlock.Block.Block)
	}

	return true
}

// firstUnacceptedMarker is similar to acceptance.FirstUnacceptedIndex, except it skips any marker gaps and returns
// an existing marker.
func findFirstUnacceptedMarker(sequence *markers.Sequence, blockFromMarkerCallback func(marker markers.Marker) (block *booker.Block, exists bool), firstUnacceptedIndexCallback func(id markers.SequenceID) markers.Index) (firstUnacceptedMarker markers.Marker) {
	firstUnacceptedMarker = markers.NewMarker(sequence.ID(), firstUnacceptedIndexCallback(sequence.ID()))

	// skip any gaps in marker indices
	for {
		if firstUnacceptedMarker.Index() >= sequence.HighestIndex() {
			return firstUnacceptedMarker
		}

		// Skip if there is no marker at the given index, i.e., the sequence has a gap.
		if _, exists := blockFromMarkerCallback(firstUnacceptedMarker); exists {
			return firstUnacceptedMarker
		}

		firstUnacceptedMarker = markers.NewMarker(sequence.ID(), firstUnacceptedMarker.Index()+1)
	}
}

func previousMarker(sequence *markers.Sequence, markerIndex markers.Index, blockFromMarkerCallback func(marker markers.Marker) (block *booker.Block, exists bool)) (previousMarker markers.Marker) {
	previousMarker = markers.NewMarker(sequence.ID(), markerIndex-1)

	// skip any gaps in marker indices and start from marker below current one.
	for {
		// if index=0 then it's a root block, and we don't need to check if a block exists.
		if previousMarker.Index() == 0 {
			return previousMarker
		}

		// if lowest index is reached, then do not continue
		if previousMarker.Index() <= sequence.LowestIndex() {
			return previousMarker
		}

		// Skip if there is no marker at the given index, i.e., the sequence has a gap.
		if _, exists := blockFromMarkerCallback(previousMarker); exists {
			return previousMarker
		}

		previousMarker = markers.NewMarker(sequence.ID(), previousMarker.Index()-1)
	}
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
