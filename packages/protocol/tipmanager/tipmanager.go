package tipmanager

import (
	"fmt"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/randommap"
	"github.com/iotaledger/hive.go/core/types"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
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

	walkerCache   *memstorage.EpochStorage[models.BlockID, types.Empty]
	evictionMutex sync.RWMutex

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

		walkerCache: memstorage.NewEpochStorage[models.BlockID, types.Empty](),

		optsTimeSinceConfirmationThreshold: time.Minute,
		optsWidth:                          0,
	}, opts)
}

func (t *TipManager) LinkTo(engine *engine.Engine) {
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
	if !t.tips.Has(block) {
		t.tips.Set(block, block)
		// t.tipsConflictTracker.AddTip(block)
		t.Events.TipAdded.Trigger(block)
		return true
	}

	return false
}

func (t *TipManager) EvictTSCCache(index epoch.Index) {
	t.evictionMutex.Lock()
	defer t.evictionMutex.Unlock()

	t.walkerCache.Evict(index)
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

// RemoveStrongParents removes all tips that are parents of the given block.
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
			fmt.Println("(time: ", time.Now(), ") selecting root block because tip pool empty:", rootBlock)

			return parents.Add(rootBlock)
		}

		// at least one tip is returned
		for _, tip := range tips {
			if t.isPastConeTimestampCorrect(tip.Block.Block) {
				parents.Add(tip.ID())
			} else {
				fmt.Printf("(time: %s) cannot select tip due to TSC condition tip issuing time (%s), time (%s), min supported time (%s), block id (%s), tip pool size (%d), scheduled: (%t), orphaned: (%t), accepted: (%t)\n",
					time.Now(),
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
					fmt.Println("(time: ", time.Now(), ") >> deleted last TIP because it doesn't pass tsc check!", tip.ID())
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

// isPastConeTimestampCorrect performs the TSC check for the given tip.
// Conceptually, this involves the following steps:
//  1. Collect all accepted blocks in the tip's past cone at the boundary of accepted/unaccapted.
//  2. Order by timestamp (ascending), if the oldest accepted block > TSC threshold then return false.
//
// This function is optimized through the use of markers and the following assumption:
//
//	If there's any unaccepted block >TSC threshold, then the oldest accepted block will be >TSC threshold, too.
func (t *TipManager) isPastConeTimestampCorrect(block *booker.Block) (timestampValid bool) {
	now := time.Now()
	markersWalked, blocksWalked := 0, 0
	blocksTime, markersTime, durationUntilLoops := time.Duration(0), time.Duration(0), time.Duration(0)
	defer func() {
		if time.Since(now) > 5*time.Millisecond {
			fmt.Printf("TSC check taking long time (%s) timeUntilLoops(%s), markersWalked(%d, %s) blocksWalked(%d, %s), time \n", time.Since(now), durationUntilLoops, markersWalked, markersTime, blocksWalked, blocksTime)
		}
	}()
	minSupportedTimestamp := t.engine.Clock.AcceptedTime().Add(-t.optsTimeSinceConfirmationThreshold)

	if !t.engine.IsBootstrapped() {
		// If the node is not bootstrapped we do not have a valid timestamp to compare against.
		// In any case, a node should never perform tip selection if not bootstrapped (via issuer plugin).
		return true
	}

	if block.IssuingTime().Before(minSupportedTimestamp) {
		return false
	}

	if t.blockAcceptanceGadget.IsBlockAccepted(block.ID()) {
		// return true if block is accepted and has valid timestamp
		return true
	}

	blocksNow := time.Now()

	t.evictionMutex.RLock()
	defer t.evictionMutex.RUnlock()

	timestampValid = t.checkBlockRecursive(block, minSupportedTimestamp, &blocksWalked)

	blocksTime = time.Since(blocksNow)
	return
}

func (t *TipManager) checkBlockRecursive(block *booker.Block, minSupportedTimestamp time.Time, i *int) (timestampValid bool) {
	if storage := t.walkerCache.Get(block.ID().Index(), false); storage != nil {
		if _, exists := storage.Get(block.ID()); exists {
			return true
		}
	}

	// if block is older than TSC then it's incorrect no matter the acceptance status
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

		*i++
		if !t.checkBlockRecursive(parentBlock.Block.Block, minSupportedTimestamp, i) {
			return false
		}

		t.walkerCache.Get(parentBlock.ID().Index(), true).Set(parentBlock.ID(), types.Void)
	}

	return true
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// WithTimeSinceConfirmationThreshold returns an option that sets the time since confirmation threshold.
func WithTimeSinceConfirmationThreshold(timeSinceConfirmationThreshold time.Duration) options.Option[TipManager] {
	return func(o *TipManager) {
		o.optsTimeSinceConfirmationThreshold = timeSinceConfirmationThreshold
	}
}

// WithWidth returns an option that configures the TipManager to maintain a certain width of the tangle.
func WithWidth(maxWidth int) options.Option[TipManager] {
	return func(t *TipManager) {
		t.optsWidth = maxWidth
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
