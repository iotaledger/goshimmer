package tresholdblockgadget

import (
	"sync"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/votes/conflicttracker"
	"github.com/iotaledger/goshimmer/packages/core/votes/sequencetracker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/causalorder"
	"github.com/iotaledger/hive.go/core/memstorage"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/ds/walker"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/module"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

// region Gadget ///////////////////////////////////////////////////////////////////////////////////////////////////////

type Gadget struct {
	events *blockgadget.Events

	booker   booker.Booker
	blockDAG blockdag.BlockDAG
	memPool  mempool.MemPool

	blocks           *memstorage.SlotStorage[models.BlockID, *blockgadget.Block]
	evictionState    *eviction.State
	evictionMutex    sync.RWMutex
	slotTimeProvider *slot.TimeProvider

	workers             *workerpool.Group
	validators          *sybilprotection.WeightedSet
	totalWeightCallback func() int64

	lastAcceptedMarker              *shrinkingmap.ShrinkingMap[markers.SequenceID, markers.Index]
	lastAcceptedMarkerMutex         sync.Mutex
	optsMarkerAcceptanceThreshold   float64
	acceptanceOrder                 *causalorder.CausalOrder[models.BlockID, *blockgadget.Block]
	lastConfirmedMarker             *shrinkingmap.ShrinkingMap[markers.SequenceID, markers.Index]
	lastConfirmedMarkerMutex        sync.Mutex
	optsMarkerConfirmationThreshold float64
	confirmationOrder               *causalorder.CausalOrder[models.BlockID, *blockgadget.Block]

	optsConflictAcceptanceThreshold float64

	module.Module
}

func NewProvider(opts ...options.Option[Gadget]) module.Provider[*engine.Engine, blockgadget.Gadget] {
	return module.Provide(func(e *engine.Engine) blockgadget.Gadget {
		g := New(opts...)

		e.HookConstructed(func() {
			e.SybilProtection.HookInitialized(func() {
				g.Initialize(e.Workers.CreateGroup("BlockGadget"), e.Tangle.Booker(), e.Tangle.BlockDAG(), e.Ledger.MemPool(), e.EvictionState, e.SlotTimeProvider(), e.SybilProtection.Validators(), e.SybilProtection.Weights().TotalWeightWithoutZeroIdentity)
			})
		})

		return g
	})
}

func New(opts ...options.Option[Gadget]) *Gadget {
	return options.Apply(&Gadget{
		events:              blockgadget.NewEvents(),
		blocks:              memstorage.NewSlotStorage[models.BlockID, *blockgadget.Block](),
		lastAcceptedMarker:  shrinkingmap.New[markers.SequenceID, markers.Index](),
		lastConfirmedMarker: shrinkingmap.New[markers.SequenceID, markers.Index](),

		optsMarkerAcceptanceThreshold:   0.67,
		optsMarkerConfirmationThreshold: 0.67,
		optsConflictAcceptanceThreshold: 0.67,
	}, opts)
}

func (g *Gadget) Initialize(workers *workerpool.Group, booker booker.Booker, blockDAG blockdag.BlockDAG, memPool mempool.MemPool, evictionState *eviction.State, slotTimeProvider *slot.TimeProvider, validators *sybilprotection.WeightedSet, totalWeightCallback func() int64) {
	g.workers = workers
	g.booker = booker
	g.blockDAG = blockDAG
	g.memPool = memPool
	g.evictionState = evictionState
	g.slotTimeProvider = slotTimeProvider
	g.validators = validators
	g.totalWeightCallback = totalWeightCallback

	wp := g.workers.CreatePool("Gadget", 2)

	g.booker.Events().SequenceTracker.VotersUpdated.Hook(func(evt *sequencetracker.VoterUpdatedEvent) {
		g.RefreshSequence(evt.SequenceID, evt.NewMaxSupportedIndex, evt.PrevMaxSupportedIndex)
	}, event.WithWorkerPool(wp))

	g.booker.Events().VirtualVoting.ConflictTracker.VoterAdded.Hook(func(evt *conflicttracker.VoterEvent[utxo.TransactionID]) {
		g.RefreshConflictAcceptance(evt.ConflictID)
	})

	g.booker.Events().SequenceEvicted.Hook(g.evictSequence, event.WithWorkerPool(wp))

	g.acceptanceOrder = causalorder.New(g.workers.CreatePool("AcceptanceOrder", 2), g.GetOrRegisterBlock, (*blockgadget.Block).IsStronglyAccepted, lo.Bind(false, g.markAsAccepted), g.acceptanceFailed, (*blockgadget.Block).StrongParents)
	g.confirmationOrder = causalorder.New(g.workers.CreatePool("ConfirmationOrder", 2), func(id models.BlockID) (entity *blockgadget.Block, exists bool) {
		g.evictionMutex.RLock()
		defer g.evictionMutex.RUnlock()

		if g.evictionState.InEvictedSlot(id) {
			return blockgadget.NewRootBlock(id, g.slotTimeProvider), true
		}

		return g.getOrRegisterBlock(id)
	}, (*blockgadget.Block).IsStronglyConfirmed, lo.Bind(false, g.markAsConfirmed), g.confirmationFailed, (*blockgadget.Block).StrongParents)

	g.evictionState.Events.SlotEvicted.Hook(g.EvictUntil, event.WithWorkerPool(g.workers.CreatePool("Eviction", 1)))

	g.TriggerConstructed()
	g.TriggerInitialized()
}

func (g *Gadget) Events() *blockgadget.Events {
	return g.events
}

// IsMarkerAccepted returns whether the given marker is accepted.
func (g *Gadget) IsMarkerAccepted(marker markers.Marker) (accepted bool) {
	g.evictionMutex.RLock()
	defer g.evictionMutex.RUnlock()

	return g.isMarkerAccepted(marker)
}

// IsMarkerConfirmed returns whether the given marker is confirmed.
func (g *Gadget) IsMarkerConfirmed(marker markers.Marker) (confirmed bool) {
	g.evictionMutex.RLock()
	defer g.evictionMutex.RUnlock()

	return g.isMarkerConfirmed(marker)
}

// IsBlockAccepted returns whether the given block is accepted.
func (g *Gadget) IsBlockAccepted(blockID models.BlockID) (accepted bool) {
	g.evictionMutex.RLock()
	defer g.evictionMutex.RUnlock()

	block, exists := g.block(blockID)
	return exists && block.IsAccepted()
}

func (g *Gadget) IsBlockConfirmed(blockID models.BlockID) bool {
	g.evictionMutex.RLock()
	defer g.evictionMutex.RUnlock()

	block, exists := g.block(blockID)
	return exists && block.IsConfirmed()
}

func (g *Gadget) isMarkerAccepted(marker markers.Marker) bool {
	if marker.Index() == 0 {
		return true
	}

	lastAcceptedIndex, exists := g.lastAcceptedMarker.Get(marker.SequenceID())
	return exists && lastAcceptedIndex >= marker.Index()
}

func (g *Gadget) isMarkerConfirmed(marker markers.Marker) bool {
	if marker.Index() == 0 {
		return true
	}

	lastConfirmedIndex, exists := g.lastConfirmedMarker.Get(marker.SequenceID())
	return exists && lastConfirmedIndex >= marker.Index()
}

func (g *Gadget) FirstUnacceptedIndex(sequenceID markers.SequenceID) (firstUnacceptedIndex markers.Index) {
	lastAcceptedIndex, exists := g.lastAcceptedMarker.Get(sequenceID)
	if !exists {
		return 1
	}

	return lastAcceptedIndex + 1
}

func (g *Gadget) FirstUnconfirmedIndex(sequenceID markers.SequenceID) (firstUnconfirmedIndex markers.Index) {
	lastConfirmedIndex, exists := g.lastConfirmedMarker.Get(sequenceID)
	if !exists {
		return 1
	}

	return lastConfirmedIndex + 1
}

// Block retrieves a Block with metadata from the in-memory storage of the Gadget.
func (g *Gadget) Block(id models.BlockID) (block *blockgadget.Block, exists bool) {
	g.evictionMutex.RLock()
	defer g.evictionMutex.RUnlock()

	return g.block(id)
}

func (g *Gadget) GetOrRegisterBlock(blockID models.BlockID) (block *blockgadget.Block, exists bool) {
	g.evictionMutex.RLock()
	defer g.evictionMutex.RUnlock()

	return g.getOrRegisterBlock(blockID)
}

func (g *Gadget) RefreshSequence(sequenceID markers.SequenceID, newMaxSupportedIndex, prevMaxSupportedIndex markers.Index) {
	g.evictionMutex.RLock()

	var acceptedBlocks, confirmedBlocks []*blockgadget.Block

	totalWeight := g.totalWeightCallback()

	if lastAcceptedIndex, exists := g.lastAcceptedMarker.Get(sequenceID); exists {
		prevMaxSupportedIndex = lo.Max(prevMaxSupportedIndex, lastAcceptedIndex)
	}

	for markerIndex := prevMaxSupportedIndex; markerIndex <= newMaxSupportedIndex; markerIndex++ {
		marker, markerExists := g.booker.BlockCeiling(markers.NewMarker(sequenceID, markerIndex))
		if !markerExists {
			break
		}

		blocksToAccept, blocksToConfirm := g.tryConfirmOrAccept(totalWeight, marker)
		acceptedBlocks = append(acceptedBlocks, blocksToAccept...)
		confirmedBlocks = append(confirmedBlocks, blocksToConfirm...)

		markerIndex = marker.Index()
	}

	g.evictionMutex.RUnlock()

	// EVICTION
	for _, block := range acceptedBlocks {
		g.acceptanceOrder.Queue(block)
	}
	for _, block := range confirmedBlocks {
		g.confirmationOrder.Queue(block)
	}
}

// tryConfirmOrAccept checks if there is enough active weight to confirm blocks and then checks
// if the marker has accumulated enough witness weight to be both accepted and confirmed.
// Acceptance and Confirmation use the same threshold if confirmation is possible.
// If there is not enough online weight to achieve confirmation, then acceptance condition is evaluated based on total active weight.
func (g *Gadget) tryConfirmOrAccept(totalWeight int64, marker markers.Marker) (blocksToAccept, blocksToConfirm []*blockgadget.Block) {
	markerTotalWeight := g.booker.MarkerVotersTotalWeight(marker)

	// check if enough weight is online to confirm based on total weight
	if IsThresholdReached(totalWeight, g.validators.TotalWeight(), g.optsMarkerConfirmationThreshold) {
		// check if marker weight has enough weight to be confirmed
		if IsThresholdReached(totalWeight, markerTotalWeight, g.optsMarkerConfirmationThreshold) {
			// need to mark outside 'if' statement, otherwise only the first condition would be executed due to lazy evaluation
			markerAccepted := g.setMarkerAccepted(marker)
			markerConfirmed := g.setMarkerConfirmed(marker)
			if markerAccepted || markerConfirmed {
				return g.propagateAcceptanceConfirmation(marker, true)
			}
		}
	} else if IsThresholdReached(g.validators.TotalWeight(), markerTotalWeight, g.optsMarkerAcceptanceThreshold) && g.setMarkerAccepted(marker) {
		return g.propagateAcceptanceConfirmation(marker, false)
	}

	return
}

func (g *Gadget) EvictUntil(index slot.Index) {
	g.acceptanceOrder.EvictUntil(index)
	g.confirmationOrder.EvictUntil(index)

	g.evictionMutex.Lock()
	defer g.evictionMutex.Unlock()

	if evictedStorage := g.blocks.Evict(index); evictedStorage != nil {
		g.events.SlotClosed.Trigger(evictedStorage)
	}
}

func (g *Gadget) block(id models.BlockID) (block *blockgadget.Block, exists bool) {
	if g.evictionState.IsRootBlock(id) {
		return blockgadget.NewRootBlock(id, g.slotTimeProvider), true
	}

	storage := g.blocks.Get(id.Index(), false)
	if storage == nil {
		return nil, false
	}

	return storage.Get(id)
}

func (g *Gadget) propagateAcceptanceConfirmation(marker markers.Marker, confirmed bool) (blocksToAccept, blocksToConfirm []*blockgadget.Block) {
	bookerBlock, blockExists := g.booker.BlockFromMarker(marker)
	if !blockExists {
		return
	}

	block, blockExists := g.getOrRegisterBlock(bookerBlock.ID())
	if !blockExists || block.IsStronglyAccepted() && !confirmed || block.IsStronglyConfirmed() && confirmed {
		// this can happen when block was a root block and while processing this method, the root blocks method has already been replaced
		return
	}

	pastConeWalker := walker.New[*blockgadget.Block](false).Push(block)
	for pastConeWalker.HasNext() {
		walkerBlock := pastConeWalker.Next()

		var acceptanceQueued, confirmationQueued bool

		if acceptanceQueued = walkerBlock.SetAcceptanceQueued(); acceptanceQueued {
			blocksToAccept = append(blocksToAccept, walkerBlock)
		}

		if confirmed {
			if confirmationQueued = walkerBlock.SetConfirmationQueued(); confirmationQueued {
				blocksToConfirm = append(blocksToConfirm, walkerBlock)
			}
		}

		if !acceptanceQueued && !confirmationQueued {
			continue
		}

		for parentBlockID := range walkerBlock.ParentsByType(models.StrongParentType) {
			parentBlock, parentExists := g.getOrRegisterBlock(parentBlockID)

			if !parentExists || !confirmed && parentBlock.IsStronglyAccepted() || confirmed && parentBlock.IsStronglyConfirmed() {
				continue
			}

			pastConeWalker.Push(parentBlock)
		}

		// Mark weak and shallow like parents as accepted/confirmed. Acceptance is not propagated past those parents,
		// therefore acceptance monotonicity can be broken, and that's why those blocks are marked as (weakly) accepted here.
		// If those blocks later become accepted through their strong children, BlockAcceptance will not be triggered again.
		for _, parentBlockID := range append(walkerBlock.ParentsByType(models.WeakParentType).Slice(), walkerBlock.ParentsByType(models.ShallowLikeParentType).Slice()...) {
			parentBlock, parentExists := g.getOrRegisterBlock(parentBlockID)
			if parentExists {
				// ignore the error, as it can only occur if parentBlock belongs to evictedEpoch, and here it will not affect
				// acceptance or confirmation monotonicity
				_ = g.markAsAccepted(parentBlock, true)

				if confirmed {
					_ = g.markAsConfirmed(parentBlock, true)
				}
			}
		}
	}

	return blocksToAccept, blocksToConfirm
}

func (g *Gadget) markAsAccepted(block *blockgadget.Block, weakly bool) (err error) {
	if g.evictionState.InEvictedSlot(block.ID()) {
		return errors.Errorf("block with %s belongs to an evicted slot", block.ID())
	}

	if block.SetAccepted(weakly) {
		// If block has been orphaned before acceptance, remove the flag from the block. Otherwise, remove the block from TimedHeap.
		if block.IsOrphaned() {
			g.blockDAG.SetOrphaned(block.Block.Block, false)
		}

		g.events.BlockAccepted.Trigger(block)

		// set ConfirmationState of payload (applicable only to transactions)
		if tx, ok := block.Transaction(); ok {
			g.memPool.SetTransactionInclusionSlot(tx.ID(), g.slotTimeProvider.IndexFromTime(block.IssuingTime()))
		}
	}

	return nil
}

func (g *Gadget) markAsConfirmed(block *blockgadget.Block, weakly bool) (err error) {
	if g.evictionState.InEvictedSlot(block.ID()) {
		return errors.Errorf("block with %s belongs to an evicted slot", block.ID())
	}

	if block.SetConfirmed(weakly) {
		g.events.BlockConfirmed.Trigger(block)
	}

	return nil
}

func (g *Gadget) setMarkerAccepted(marker markers.Marker) (wasUpdated bool) {
	// This method can be called from multiple goroutines, and we need to update the value atomically.
	// However, when reading lastAcceptedMarker we don't need to lock because the storage is already locking.
	g.lastAcceptedMarkerMutex.Lock()
	defer g.lastAcceptedMarkerMutex.Unlock()

	if index, exists := g.lastAcceptedMarker.Get(marker.SequenceID()); !exists || index < marker.Index() {
		g.lastAcceptedMarker.Set(marker.SequenceID(), marker.Index())
		return true
	}
	return false
}

func (g *Gadget) setMarkerConfirmed(marker markers.Marker) (wasUpdated bool) {
	// This method can be called from multiple goroutines, and we need to update the value atomically.
	// However, when reading lastConfirmedMarker we don't need to lock because the storage is already locking.
	g.lastConfirmedMarkerMutex.Lock()
	defer g.lastConfirmedMarkerMutex.Unlock()

	if index, exists := g.lastConfirmedMarker.Get(marker.SequenceID()); !exists || index < marker.Index() {
		g.lastConfirmedMarker.Set(marker.SequenceID(), marker.Index())
		return true
	}
	return false
}

func (g *Gadget) acceptanceFailed(block *blockgadget.Block, err error) {
	g.events.Error.Trigger(errors.Wrapf(err, "could not mark block %s as accepted", block.ID()))
}

func (g *Gadget) confirmationFailed(block *blockgadget.Block, err error) {
	g.events.Error.Trigger(errors.Wrapf(err, "could not mark block %s as confirmed", block.ID()))
}

func (g *Gadget) evictSequence(sequenceID markers.SequenceID) {
	g.evictionMutex.Lock()
	defer g.evictionMutex.Unlock()

	g.lastAcceptedMarker.Delete(sequenceID)
	g.lastConfirmedMarker.Delete(sequenceID)
}

func (g *Gadget) getOrRegisterBlock(blockID models.BlockID) (block *blockgadget.Block, exists bool) {
	block, exists = g.block(blockID)
	if !exists {
		virtualVotingBlock, virtualVotingBlockExists := g.booker.Block(blockID)
		if !virtualVotingBlockExists {
			return nil, false
		}

		var err error
		block, err = g.registerBlock(virtualVotingBlock)
		if err != nil {
			g.events.Error.Trigger(errors.Wrapf(err, "could not register block %s", blockID))
			return nil, false
		}
	}
	return block, true
}

func (g *Gadget) registerBlock(virtualVotingBlock *booker.Block) (block *blockgadget.Block, err error) {
	if g.evictionState.InEvictedSlot(virtualVotingBlock.ID()) {
		return nil, errors.Errorf("block %s belongs to an evicted slot", virtualVotingBlock.ID())
	}

	blockStorage := g.blocks.Get(virtualVotingBlock.ID().Index(), true)
	block, _ = blockStorage.GetOrCreate(virtualVotingBlock.ID(), func() *blockgadget.Block {
		return blockgadget.NewBlock(virtualVotingBlock)
	})

	return block, nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Conflict Acceptance //////////////////////////////////////////////////////////////////////////////////////////

func (g *Gadget) RefreshConflictAcceptance(conflictID utxo.TransactionID) {
	conflict, exists := g.memPool.ConflictDAG().Conflict(conflictID)
	if !exists {
		return
	}

	conflictWeight := g.booker.VirtualVoting().ConflictVotersTotalWeight(conflictID)

	if !IsThresholdReached(g.totalWeightCallback(), conflictWeight, g.optsConflictAcceptanceThreshold) {
		return
	}

	markAsAccepted := true

	conflict.ForEachConflictingConflict(func(conflictingConflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) bool {
		conflictingConflictWeight := g.booker.VirtualVoting().ConflictVotersTotalWeight(conflictingConflict.ID())

		// if the conflict is less than 66% ahead, then don't mark as accepted
		if !IsThresholdReached(g.totalWeightCallback(), conflictWeight-conflictingConflictWeight, g.optsConflictAcceptanceThreshold) {
			markAsAccepted = false
		}
		return markAsAccepted
	})

	if markAsAccepted {
		g.memPool.ConflictDAG().SetConflictAccepted(conflictID)
	}
}

func IsThresholdReached(weight, otherWeight int64, threshold float64) bool {
	return otherWeight > int64(float64(weight)*threshold)
}

var _ blockgadget.Gadget = new(Gadget)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithMarkerAcceptanceThreshold(acceptanceThreshold float64) options.Option[Gadget] {
	return func(gadget *Gadget) {
		gadget.optsMarkerAcceptanceThreshold = acceptanceThreshold
	}
}

func WithConflictAcceptanceThreshold(acceptanceThreshold float64) options.Option[Gadget] {
	return func(gadget *Gadget) {
		gadget.optsConflictAcceptanceThreshold = acceptanceThreshold
	}
}

func WithConfirmationThreshold(confirmationThreshold float64) options.Option[Gadget] {
	return func(gadget *Gadget) {
		gadget.optsMarkerConfirmationThreshold = confirmationThreshold
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
