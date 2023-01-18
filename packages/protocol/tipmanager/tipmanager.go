package tipmanager

import (
	"fmt"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/randommap"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/types"
	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
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

	tips       *randommap.RandomMap[models.BlockID, *scheduler.Block]
	futureTips *memstorage.EpochStorage[commitment.ID, *memstorage.Storage[models.BlockID, *scheduler.Block]]
	// TODO: reintroduce TipsConflictTracker
	// tipsConflictTracker *TipsConflictTracker

	commitmentRecentBoundary epoch.Index

	optsTimeSinceConfirmationThreshold time.Duration
	optsWidth                          int
}

// New creates a new TipManager.
func New(schedulerBlockRetrieverFunc blockRetrieverFunc, opts ...options.Option[TipManager]) (t *TipManager) {
	t = options.Apply(&TipManager{
		Events: NewEvents(),

		schedulerBlockRetrieverFunc: schedulerBlockRetrieverFunc,

		tips:       randommap.New[models.BlockID, *scheduler.Block](),
		futureTips: memstorage.NewEpochStorage[commitment.ID, *memstorage.Storage[models.BlockID, *scheduler.Block]](),
		// TODO: reintroduce TipsConflictTracker
		// tipsConflictTracker: NewTipsConflictTracker(tangle),

		walkerCache: memstorage.NewEpochStorage[models.BlockID, types.Empty](),

		optsTimeSinceConfirmationThreshold: time.Minute,
		optsWidth:                          0,
	}, opts)

	t.commitmentRecentBoundary = epoch.Index(int64(t.optsTimeSinceConfirmationThreshold.Seconds()) / epoch.Duration)

	return
}

func (t *TipManager) LinkTo(engine *engine.Engine) {
	t.tips = randommap.New[models.BlockID, *scheduler.Block]()
	t.engine = engine
	t.blockAcceptanceGadget = engine.Consensus.BlockGadget
}

func (t *TipManager) AddTip(block *scheduler.Block) {
	// Check if any children that are accepted or scheduled and return if true, to guarantee that parents are not added
	// to the tipset after their children.
	if t.checkMonotonicity(block) {
		return
	}

	// If the commitment is in the future, and not known to be forking, we cannot yet add it to the main tipset.
	if t.isFutureCommitment(block) {
		t.addFutureTip(block)
		return
	}

	// Check if the block commits to an old epoch.
	if !t.isRecentCommitment(block) {
		return
	}

	t.addTip(block)
}

func (t *TipManager) EvictTSCCache(index epoch.Index) {
	t.evictionMutex.Lock()
	defer t.evictionMutex.Unlock()

	t.walkerCache.Evict(index)
}

func (t *TipManager) DeleteTip(block *scheduler.Block) (deleted bool) {
	if _, deleted = t.tips.Delete(block.ID()); deleted {
		// t.tipsConflictTracker.RemoveTip(block)
		t.Events.TipRemoved.Trigger(block)
	}
	return
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
			if err := t.isValidTip(tip); err == nil {
				parents.Add(tip.ID())
			} else {
				t.DeleteTip(tip)

				// DEBUG
				fmt.Printf("(time: %s) cannot select tip due to error: %s\n", time.Now(), err)
				if t.tips.Size() == 0 {
					fmt.Println("(time: ", time.Now(), ") >> deleted last TIP because it doesn't pass checks!", tip.ID())
				}
			}
		}

		if len(parents) > 0 {
			return parents
		}
	}
}

// AllTips returns a list of all tips that are stored in the TipManger.
func (t *TipManager) AllTips() (allTips []*scheduler.Block) {
	allTips = make([]*scheduler.Block, 0, t.tips.Size())
	t.tips.ForEach(func(_ models.BlockID, value *scheduler.Block) bool {
		allTips = append(allTips, value)
		return true
	})

	return
}

// TipCount the amount of tips.
func (t *TipManager) TipCount() int {
	return t.tips.Size()
}

// FutureTipCount returns the amount of future tips per epoch.
func (t *TipManager) FutureTipCount() (futureTipsPerEpoch map[epoch.Index]int) {
	futureTipsPerEpoch = make(map[epoch.Index]int)
	t.futureTips.ForEach(func(index epoch.Index, commitmentStorage *memstorage.Storage[commitment.ID, *memstorage.Storage[models.BlockID, *scheduler.Block]]) {
		commitmentStorage.ForEach(func(cm commitment.ID, tipStorage *memstorage.Storage[models.BlockID, *scheduler.Block]) bool {
			futureTipsPerEpoch[index] += tipStorage.Size()
			return true
		})
	})

	return
}

// PromoteFutureTips promotes to the main tippool all future tips that belong to the given commitment.
func (t *TipManager) PromoteFutureTips(cm *commitment.Commitment) {
	t.evictionMutex.RLock()
	defer func() {
		t.evictionMutex.RUnlock()
		t.Evict(cm.Index())
	}()

	if futureEpochTips := t.futureTips.Get(cm.Index()); futureEpochTips != nil {
		if tipsForCommitment, exists := futureEpochTips.Get(cm.ID()); exists {
			tipsToPromote := make(map[models.BlockID]*scheduler.Block)
			tipsToNotPromote := set.NewAdvancedSet[models.BlockID]()

			tipsForCommitment.ForEach(func(blockID models.BlockID, tip *scheduler.Block) bool {
				for _, tipParent := range tip.Parents() {
					tipsToNotPromote.Add(tipParent)
				}
				tipsToPromote[blockID] = tip
				return true
			})

			for tipID, tip := range tipsToPromote {
				// regardless if the tip makes it into the tippool, we remove its strong parents anyway
				// currentTip <- futureTipNotToAdd <- futureTipToAdd
				// We want to remove currentTip even if futureTipNotToAdd is not added to the tippool.
				t.RemoveStrongParents(tip.ModelsBlock)
				if !tipsToNotPromote.Has(tipID) {
					t.addTip(tip)
				}
			}
		}
	}
}

// Evict removes all parked tips that belong to an evicted epoch.
func (t *TipManager) Evict(index epoch.Index) {
	t.evictionMutex.Lock()
	defer t.evictionMutex.Unlock()

	t.futureTips.Evict(index)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TSC to prevent lazy tips /////////////////////////////////////////////////////////////////////////////////////

func (t *TipManager) addTip(block *scheduler.Block) (added bool) {
	if !t.tips.Has(block.ID()) {
		t.tips.Set(block.ID(), block)
		// t.tipsConflictTracker.AddTip(block)
		t.Events.TipAdded.Trigger(block)

		// skip removing tips if a width is set -> allows to artificially create a wide Tangle.
		if t.TipCount() <= t.optsWidth {
			return true
		}

		// a tip loses its tip status if it is referenced by another block
		t.RemoveStrongParents(block.ModelsBlock)

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

func (t *TipManager) addFutureTip(block *scheduler.Block) (added bool) {
	t.evictionMutex.RLock()
	defer t.evictionMutex.RUnlock()

	return lo.Return1(t.futureTips.Get(block.Commitment().Index(), true).RetrieveOrCreate(block.Commitment().ID(), func() *memstorage.Storage[models.BlockID, *scheduler.Block] {
		return memstorage.New[models.BlockID, *scheduler.Block]()
	})).Set(block.ID(), block)
}

func (t *TipManager) isValidTip(tip *scheduler.Block) (err error) {
	// TODO: fix this check when node is bootstrapping and commits a bunch of epochs at once.
	//if !t.isRecentCommitment(tip) {
	//	return errors.Errorf("cannot select tip due to commitment not being recent (%s - %d), current commitment (%d)", tip.ID().String(), tip.Commitment().Index(), t.engine.Storage.Settings.LatestCommitment().Index())
	//}

	if !t.isPastConeTimestampCorrect(tip.Block.Block) {
		return errors.Errorf("cannot select tip due to TSC condition tip issuing time (%s), time (%s), min supported time (%s), block id (%s), tip pool size (%d), scheduled: (%t), orphaned: (%t), accepted: (%t)",
			tip.IssuingTime(),
			t.engine.Clock.AcceptedTime(),
			t.engine.Clock.AcceptedTime().Add(-t.optsTimeSinceConfirmationThreshold),
			tip.ID().Base58(),
			t.tips.Size(),
			tip.IsScheduled(),
			tip.IsOrphaned(),
			t.engine.Consensus.BlockGadget.IsBlockAccepted(tip.ID()),
		)
	}

	return nil
}

// isRecentCommitment returns true if the commitment of the given block is not in the future, and it is not older than TSC threshold
// epoch with respect to our latest commitment.
func (t *TipManager) isRecentCommitment(block *scheduler.Block) (isFresh bool) {
	return block.Commitment().Index() >= (t.engine.Storage.Settings.LatestCommitment().Index() - t.commitmentRecentBoundary).Max(0)
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
		return false
	}

	if t.blockAcceptanceGadget.IsBlockAccepted(block.ID()) {
		// return true if block is accepted and has valid timestamp
		return true
	}

	t.evictionMutex.RLock()
	defer t.evictionMutex.RUnlock()

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

		if !t.checkBlockRecursive(parentBlock.Block.Block, minSupportedTimestamp) {
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
