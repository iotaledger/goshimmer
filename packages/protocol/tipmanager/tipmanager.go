package tipmanager

import (
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/memstorage"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/ds/randommap"
	"github.com/iotaledger/hive.go/ds/types"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

type blockRetrieverFunc func(id models.BlockID) (block *scheduler.Block, exists bool)

// region TipManager ///////////////////////////////////////////////////////////////////////////////////////////////////

type TipManager struct {
	Events *Events

	engine                *engine.Engine
	blockAcceptanceGadget blockgadget.Gadget

	workers                     *workerpool.Group
	schedulerBlockRetrieverFunc blockRetrieverFunc

	walkerCache *memstorage.SlotStorage[models.BlockID, types.Empty]

	mutex               sync.RWMutex
	tips                *randommap.RandomMap[models.BlockID, *scheduler.Block]
	TipsConflictTracker *TipsConflictTracker

	commitmentRecentBoundary slot.Index

	optsTimeSinceConfirmationThreshold time.Duration
	optsWidth                          int
}

// New creates a new TipManager.
func New(workers *workerpool.Group, schedulerBlockRetrieverFunc blockRetrieverFunc, opts ...options.Option[TipManager]) (t *TipManager) {
	t = options.Apply(&TipManager{
		Events: NewEvents(),

		workers:                     workers,
		schedulerBlockRetrieverFunc: schedulerBlockRetrieverFunc,

		tips: randommap.New[models.BlockID, *scheduler.Block](),

		walkerCache: memstorage.NewSlotStorage[models.BlockID, types.Empty](),

		optsTimeSinceConfirmationThreshold: time.Minute,
		optsWidth:                          0,
	}, opts)

	return
}

func (t *TipManager) LinkTo(engine *engine.Engine) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.commitmentRecentBoundary = slot.Index(int64(t.optsTimeSinceConfirmationThreshold.Seconds()) / engine.SlotTimeProvider().Duration())

	t.walkerCache = memstorage.NewSlotStorage[models.BlockID, types.Empty]()
	t.tips = randommap.New[models.BlockID, *scheduler.Block]()

	t.engine = engine
	t.blockAcceptanceGadget = engine.Consensus.BlockGadget()
	if t.TipsConflictTracker != nil {
		t.TipsConflictTracker.Cleanup()
	}
	t.TipsConflictTracker = NewTipsConflictTracker(t.workers.CreatePool("TipsConflictTracker", 1), engine)
}

func (t *TipManager) AddTip(block *scheduler.Block) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	// Check if any children that are accepted or scheduled and return if true, to guarantee that parents are not added
	// to the tipset after their children.
	if t.checkMonotonicity(block) {
		return
	}

	t.AddTipNonMonotonic(block)
}

func (t *TipManager) AddTipNonMonotonic(block *scheduler.Block) {
	if block.IsSubjectivelyInvalid() {
		return
	}

	// Do not add a tip booked on a reject branch, we won't use it as a tip and it will otherwise remove parent tips.
	blockConflictIDs := t.engine.Tangle.Booker().BlockConflicts(block.Block)
	if t.engine.Ledger.MemPool().ConflictDAG().ConfirmationState(blockConflictIDs).IsRejected() {
		return
	}

	if t.addTip(block) {
		t.TipsConflictTracker.AddTip(block, blockConflictIDs)
	}
}

func (t *TipManager) EvictTSCCache(index slot.Index) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.walkerCache.Evict(index)
}

func (t *TipManager) deleteTip(block *scheduler.Block) (deleted bool) {
	if _, deleted = t.tips.Delete(block.ID()); deleted {
		t.TipsConflictTracker.RemoveTip(block)
		t.Events.TipRemoved.Trigger(block)
	}
	return
}

func (t *TipManager) DeleteTip(block *scheduler.Block) (deleted bool) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	return t.deleteTip(block)
}

// RemoveStrongParents removes all tips that are parents of the given block.
func (t *TipManager) RemoveStrongParents(block *models.Block) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.removeStrongParents(block)
}

// RemoveStrongParents removes all tips that are parents of the given block.
func (t *TipManager) removeStrongParents(block *models.Block) {
	block.ForEachParent(func(parent models.Parent) {
		if parentBlock, exists := t.schedulerBlockRetrieverFunc(parent.ID); exists {
			t.deleteTip(parentBlock)
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
	t.mutex.Lock() // deleteTip might get called, so we need a write-lock here
	defer t.mutex.Unlock()

	parents = models.NewBlockIDs()
	tips := t.tips.RandomUniqueEntries(count)

	// We obtain up to 8 latest root blocks if there is no valid tip and we submit them to the TSC check as some
	// could be old in case of a slow growing BlockDAG.
	if len(tips) == 0 {
		rootBlocks := t.engine.EvictionState.LatestRootBlocks()

		for blockID := range rootBlocks {
			if block, exist := t.schedulerBlockRetrieverFunc(blockID); exist {
				tips = append(tips, block)
			}
		}
		fmt.Println("(time: ", time.Now(), ") selecting root blocks because tip pool empty:", rootBlocks)
	}

	for _, tip := range tips {
		if err := t.isValidTip(tip); err == nil {
			parents.Add(tip.ID())
		} else {
			t.deleteTip(tip)

			// DEBUG
			fmt.Printf("(time: %s) cannot select tip due to error: %s\n", time.Now(), err)
			if t.tips.Size() == 0 {
				fmt.Println("(time: ", time.Now(), ") >> deleted last TIP because it doesn't pass checks!", tip.ID())
			}
		}
	}

	return parents
}

// AllTips returns a list of all tips that are stored in the TipManger.
func (t *TipManager) AllTips() (allTips []*scheduler.Block) {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	allTips = make([]*scheduler.Block, 0, t.tips.Size())
	t.tips.ForEach(func(_ models.BlockID, value *scheduler.Block) bool {
		allTips = append(allTips, value)
		return true
	})

	return
}

// TipCount the amount of tips.
func (t *TipManager) TipCount() int {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	return t.tips.Size()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TSC to prevent lazy tips /////////////////////////////////////////////////////////////////////////////////////

func (t *TipManager) addTip(block *scheduler.Block) (added bool) {
	if !t.tips.Has(block.ID()) {
		t.tips.Set(block.ID(), block)
		// t.tipsConflictTracker.AddTip(block)
		t.Events.TipAdded.Trigger(block)

		// skip removing tips if a width is set -> allows to artificially create a wide Tangle.
		if t.tips.Size() <= t.optsWidth {
			return true
		}

		// a tip loses its tip status if it is referenced by another block
		t.removeStrongParents(block.ModelsBlock)

		return true
	}

	return false
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

// isFutureCommitment returns true if the block belongs to a commitment that is not yet known.
func (t *TipManager) isFutureCommitment(block *scheduler.Block) (isUnknown bool) {
	return block.Commitment().Index() > t.engine.Storage.Settings.LatestCommitment().Index()
}

func (t *TipManager) isValidTip(tip *scheduler.Block) (err error) {
	if !t.isPastConeTimestampCorrect(tip.Block) {
		return errors.Errorf("cannot select tip due to TSC condition tip issuing time (%s), time (%s), min supported time (%s), block id (%s), tip pool size (%d), scheduled: (%t), orphaned: (%t), accepted: (%t)",
			tip.IssuingTime(),
			t.engine.Clock.Accepted().Time(),
			t.engine.Clock.Accepted().Time().Add(-t.optsTimeSinceConfirmationThreshold),
			tip.ID().Base58(),
			t.tips.Size(),
			tip.IsScheduled(),
			tip.IsOrphaned(),
			t.blockAcceptanceGadget.IsBlockAccepted(tip.ID()),
		)
	}

	return nil
}

func (t *TipManager) IsPastConeTimestampCorrect(block *booker.Block) (timestampValid bool) {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	return t.isPastConeTimestampCorrect(block)
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
	minSupportedTimestamp := t.engine.Clock.Accepted().Time().Add(-t.optsTimeSinceConfirmationThreshold)

	if !t.engine.IsBootstrapped() {
		// If the node is not bootstrapped we do not have a valid timestamp to compare against.
		// In any case, a node should never perform tip selection if not bootstrapped (via issuer plugin).
		return true
	}

	timestampValid = t.checkBlockRecursive(block, minSupportedTimestamp)

	return
}

func (t *TipManager) checkBlockRecursive(block *booker.Block, minSupportedTimestamp time.Time) (timestampValid bool) {
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
		t.walkerCache.Get(block.ID().Index(), true).Set(block.ID(), types.Void)
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

		if !t.checkBlockRecursive(parentBlock.Block, minSupportedTimestamp) {
			return false
		}
	}

	t.walkerCache.Get(block.ID().Index(), true).Set(block.ID(), types.Void)
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
