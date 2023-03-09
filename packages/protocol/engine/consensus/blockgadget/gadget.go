package blockgadget

import (
	"fmt"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/votes/conflicttracker"
	"github.com/iotaledger/goshimmer/packages/core/votes/sequencetracker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/causalorder"
	"github.com/iotaledger/hive.go/core/memstorage"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/ds/walker"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

// region Gadget ///////////////////////////////////////////////////////////////////////////////////////////////////////

type Gadget struct {
	Events *Events

	tangle           *tangle.Tangle
	blocks           *memstorage.SlotStorage[models.BlockID, *Block]
	evictionState    *eviction.State
	evictionMutex    sync.RWMutex
	slotTimeProvider *slot.TimeProvider

	optsConflictAcceptanceThreshold float64

	totalWeightCallback func() int64

	lastAcceptedMarker              *shrinkingmap.ShrinkingMap[markers.SequenceID, markers.Index]
	lastAcceptedMarkerMutex         sync.Mutex
	optsMarkerAcceptanceThreshold   float64
	acceptanceOrder                 *causalorder.CausalOrder[models.BlockID, *Block]
	lastConfirmedMarker             *shrinkingmap.ShrinkingMap[markers.SequenceID, markers.Index]
	lastConfirmedMarkerMutex        sync.Mutex
	optsMarkerConfirmationThreshold float64
	confirmationOrder               *causalorder.CausalOrder[models.BlockID, *Block]

	workers *workerpool.Group
}

func New(workers *workerpool.Group, tangleInstance *tangle.Tangle, evictionState *eviction.State, slotTimeProvider *slot.TimeProvider, totalWeightCallback func() int64, opts ...options.Option[Gadget]) (gadget *Gadget) {
	return options.Apply(&Gadget{
		Events:                          NewEvents(),
		workers:                         workers,
		tangle:                          tangleInstance,
		blocks:                          memstorage.NewSlotStorage[models.BlockID, *Block](),
		lastAcceptedMarker:              shrinkingmap.New[markers.SequenceID, markers.Index](),
		evictionState:                   evictionState,
		slotTimeProvider:                slotTimeProvider,
		optsMarkerAcceptanceThreshold:   0.67,
		optsMarkerConfirmationThreshold: 0.67,
		optsConflictAcceptanceThreshold: 0.67,
	}, opts, func(a *Gadget) {
		a.Events = NewEvents()

		a.tangle = tangleInstance
		a.totalWeightCallback = totalWeightCallback
		a.lastAcceptedMarker = shrinkingmap.New[markers.SequenceID, markers.Index]()
		a.lastConfirmedMarker = shrinkingmap.New[markers.SequenceID, markers.Index]()
		a.blocks = memstorage.NewSlotStorage[models.BlockID, *Block]()

		a.acceptanceOrder = causalorder.New(workers.CreatePool("AcceptanceOrder", 2), a.GetOrRegisterBlock, (*Block).IsStronglyAccepted, lo.Bind(false, a.markAsAccepted), a.acceptanceFailed, (*Block).StrongParents)
		a.confirmationOrder = causalorder.New(workers.CreatePool("ConfirmationOrder", 2), func(id models.BlockID) (entity *Block, exists bool) {
			a.evictionMutex.RLock()
			defer a.evictionMutex.RUnlock()

			if a.evictionState.InEvictedSlot(id) {
				return NewRootBlock(id, a.slotTimeProvider), true
			}

			return a.getOrRegisterBlock(id)
		}, (*Block).IsStronglyConfirmed, lo.Bind(false, a.markAsConfirmed), a.confirmationFailed, (*Block).StrongParents)
	}, (*Gadget).setup)
}

// IsMarkerAccepted returns whether the given marker is accepted.
func (a *Gadget) IsMarkerAccepted(marker markers.Marker) (accepted bool) {
	a.evictionMutex.RLock()
	defer a.evictionMutex.RUnlock()

	return a.isMarkerAccepted(marker)
}

// IsMarkerConfirmed returns whether the given marker is confirmed.
func (a *Gadget) IsMarkerConfirmed(marker markers.Marker) (confirmed bool) {
	a.evictionMutex.RLock()
	defer a.evictionMutex.RUnlock()

	return a.isMarkerConfirmed(marker)
}

// IsBlockAccepted returns whether the given block is accepted.
func (a *Gadget) IsBlockAccepted(blockID models.BlockID) (accepted bool) {
	a.evictionMutex.RLock()
	defer a.evictionMutex.RUnlock()

	return a.isBlockAccepted(blockID)
}

func (a *Gadget) isBlockAccepted(blockID models.BlockID) bool {
	block, exists := a.block(blockID)
	return exists && block.IsAccepted()
}

func (a *Gadget) isBlockConfirmed(blockID models.BlockID) bool {
	block, exists := a.block(blockID)
	return exists && block.IsConfirmed()
}

func (a *Gadget) isMarkerAccepted(marker markers.Marker) bool {
	if marker.Index() == 0 {
		return true
	}

	lastAcceptedIndex, exists := a.lastAcceptedMarker.Get(marker.SequenceID())
	return exists && lastAcceptedIndex >= marker.Index()
}

func (a *Gadget) isMarkerConfirmed(marker markers.Marker) bool {
	if marker.Index() == 0 {
		return true
	}

	lastConfirmedIndex, exists := a.lastConfirmedMarker.Get(marker.SequenceID())
	return exists && lastConfirmedIndex >= marker.Index()
}

func (a *Gadget) FirstUnacceptedIndex(sequenceID markers.SequenceID) (firstUnacceptedIndex markers.Index) {
	lastAcceptedIndex, exists := a.lastAcceptedMarker.Get(sequenceID)
	if !exists {
		return 1
	}

	return lastAcceptedIndex + 1
}

func (a *Gadget) FirstUnconfirmedIndex(sequenceID markers.SequenceID) (firstUnconfirmedIndex markers.Index) {
	lastConfirmedIndex, exists := a.lastConfirmedMarker.Get(sequenceID)
	if !exists {
		return 1
	}

	return lastConfirmedIndex + 1
}

// Block retrieves a Block with metadata from the in-memory storage of the Gadget.
func (a *Gadget) Block(id models.BlockID) (block *Block, exists bool) {
	a.evictionMutex.RLock()
	defer a.evictionMutex.RUnlock()

	return a.block(id)
}

func (a *Gadget) GetOrRegisterBlock(blockID models.BlockID) (block *Block, exists bool) {
	a.evictionMutex.RLock()
	defer a.evictionMutex.RUnlock()

	return a.getOrRegisterBlock(blockID)
}

func (a *Gadget) RefreshSequence(sequenceID markers.SequenceID, newMaxSupportedIndex, prevMaxSupportedIndex markers.Index) {
	a.evictionMutex.RLock()

	var acceptedBlocks, confirmedBlocks []*Block

	totalWeight := a.totalWeightCallback()

	if lastAcceptedIndex, exists := a.lastAcceptedMarker.Get(sequenceID); exists {
		prevMaxSupportedIndex = lo.Max(prevMaxSupportedIndex, lastAcceptedIndex)
	}

	for markerIndex := prevMaxSupportedIndex; markerIndex <= newMaxSupportedIndex; markerIndex++ {
		marker, markerExists := a.tangle.Booker.BlockCeiling(markers.NewMarker(sequenceID, markerIndex))
		if !markerExists {
			break
		}

		blocksToAccept, blocksToConfirm := a.tryConfirmOrAccept(totalWeight, marker)
		acceptedBlocks = append(acceptedBlocks, blocksToAccept...)
		confirmedBlocks = append(confirmedBlocks, blocksToConfirm...)

		markerIndex = marker.Index()
	}

	a.evictionMutex.RUnlock()

	// EVICTION
	for _, block := range acceptedBlocks {
		a.acceptanceOrder.Queue(block)
	}
	for _, block := range confirmedBlocks {
		a.confirmationOrder.Queue(block)
	}
}

// tryConfirmOrAccept checks if there is enough active weight to confirm blocks and then checks
// if the marker has accumulated enough witness weight to be both accepted and confirmed.
// Acceptance and Confirmation use the same threshold if confirmation is possible.
// If there is not enough online weight to achieve confirmation, then acceptance condition is evaluated based on total active weight.
func (a *Gadget) tryConfirmOrAccept(totalWeight int64, marker markers.Marker) (blocksToAccept, blocksToConfirm []*Block) {
	markerTotalWeight := a.tangle.Booker.VirtualVoting.MarkerVotersTotalWeight(marker)

	// check if enough weight is online to confirm based on total weight
	if IsThresholdReached(totalWeight, a.tangle.Booker.VirtualVoting.Validators.TotalWeight(), a.optsMarkerConfirmationThreshold) {
		// check if marker weight has enough weight to be confirmed
		if IsThresholdReached(totalWeight, markerTotalWeight, a.optsMarkerConfirmationThreshold) {
			// need to mark outside 'if' statement, otherwise only the first condition would be executed due to lazy evaluation
			markerAccepted := a.setMarkerAccepted(marker)
			markerConfirmed := a.setMarkerConfirmed(marker)
			if markerAccepted || markerConfirmed {
				return a.propagateAcceptanceConfirmation(marker, true)
			}
		}
	} else if IsThresholdReached(a.tangle.Booker.VirtualVoting.Validators.TotalWeight(), markerTotalWeight, a.optsMarkerAcceptanceThreshold) && a.setMarkerAccepted(marker) {
		return a.propagateAcceptanceConfirmation(marker, false)
	}

	return
}

func (a *Gadget) EvictUntil(index slot.Index) {
	a.acceptanceOrder.EvictUntil(index)
	a.confirmationOrder.EvictUntil(index)

	a.evictionMutex.Lock()
	defer a.evictionMutex.Unlock()

	if evictedStorage := a.blocks.Evict(index); evictedStorage != nil {
		a.Events.SlotClosed.Trigger(evictedStorage)
	}
}

func (a *Gadget) setup() {
	wp := a.workers.CreatePool("Gadget", 2)

	a.tangle.Booker.VirtualVoting.Events.SequenceTracker.VotersUpdated.Hook(func(evt *sequencetracker.VoterUpdatedEvent) {
		a.RefreshSequence(evt.SequenceID, evt.NewMaxSupportedIndex, evt.PrevMaxSupportedIndex)
	}, event.WithWorkerPool(wp))

	a.tangle.Booker.VirtualVoting.Events.ConflictTracker.VoterAdded.Hook(func(evt *conflicttracker.VoterEvent[utxo.TransactionID]) {
		fmt.Println(">> addSupport", evt.BlockID, evt.ConflictID, evt.Voter)
		a.RefreshConflictAcceptance(evt.ConflictID)
	})

	a.tangle.Booker.Events.MarkerManager.SequenceEvicted.Hook(a.evictSequence, event.WithWorkerPool(wp))
}

func (a *Gadget) block(id models.BlockID) (block *Block, exists bool) {
	if a.evictionState.IsRootBlock(id) {
		return NewRootBlock(id, a.slotTimeProvider), true
	}

	storage := a.blocks.Get(id.Index(), false)
	if storage == nil {
		return nil, false
	}

	return storage.Get(id)
}

func (a *Gadget) propagateAcceptanceConfirmation(marker markers.Marker, confirmed bool) (blocksToAccept, blocksToConfirm []*Block) {
	bookerBlock, blockExists := a.tangle.Booker.BlockFromMarker(marker)
	if !blockExists {
		return
	}

	block, blockExists := a.getOrRegisterBlock(bookerBlock.ID())
	if !blockExists || block.IsAccepted() && !confirmed || block.IsConfirmed() && confirmed {
		// this can happen when block was a root block and while processing this method, the root blocks method has already been replaced
		return
	}

	var otherParentsToAccept []models.BlockID

	pastConeWalker := walker.New[*Block](false).Push(block)
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
			if !confirmed && a.isBlockAccepted(parentBlockID) || confirmed && a.isBlockConfirmed(parentBlockID) {
				continue
			}

			parentBlock, parentExists := a.getOrRegisterBlock(parentBlockID)
			if parentExists {
				pastConeWalker.Push(parentBlock)
			}
		}

		// We don't want to disturb the monotonicity of the strong parents and propagate through the entire past cone.
		// Therefore, we mark through the weak and shallow like parents as accepted/confirmed only later.
		otherParentsToAccept = append(otherParentsToAccept, walkerBlock.ParentsByType(models.WeakParentType).Slice()...)
		otherParentsToAccept = append(otherParentsToAccept, walkerBlock.ParentsByType(models.ShallowLikeParentType).Slice()...)
	}

	for _, parentBlockID := range otherParentsToAccept {
		parentBlock, parentExists := a.getOrRegisterBlock(parentBlockID)
		if parentExists {
			if err := a.markAsAccepted(parentBlock, true); err != nil {
				fmt.Println("error while marking weak/like parent ", parentBlockID, " as accepted: ", err)
			}

			if confirmed {
				if err := a.markAsAccepted(parentBlock, true); err != nil {
					fmt.Println("error while marking weak/like parent ", parentBlockID, " as confirmed: ", err)
				}
			}
		}
	}

	return blocksToAccept, blocksToConfirm
}

func (a *Gadget) markAsAccepted(block *Block, weakly bool) (err error) {
	if a.evictionState.InEvictedSlot(block.ID()) {
		return errors.Errorf("block with %s belongs to an evicted slot", block.ID())
	}

	if block.SetAccepted(weakly) {
		// If block has been orphaned before acceptance, remove the flag from the block. Otherwise, remove the block from TimedHeap.
		if block.IsOrphaned() {
			a.tangle.BlockDAG.SetOrphaned(block.Block.Block, false)
		}

		a.Events.BlockAccepted.Trigger(block)

		// set ConfirmationState of payload (applicable only to transactions)
		if tx, ok := block.Transaction(); ok {
			a.tangle.Ledger.SetTransactionInclusionSlot(tx.ID(), a.slotTimeProvider.IndexFromTime(block.IssuingTime()))
		}
	}

	return nil
}

func (a *Gadget) markAsConfirmed(block *Block, weakly bool) (err error) {
	if a.evictionState.InEvictedSlot(block.ID()) {
		return errors.Errorf("block with %s belongs to an evicted slot", block.ID())
	}

	if block.SetConfirmed(weakly) {
		a.Events.BlockConfirmed.Trigger(block)
	}

	return nil
}

func (a *Gadget) setMarkerAccepted(marker markers.Marker) (wasUpdated bool) {
	// This method can be called from multiple goroutines and we need to update the value atomically.
	// However, when reading lastAcceptedMarker we don't need to lock because the storage is already locking.
	a.lastAcceptedMarkerMutex.Lock()
	defer a.lastAcceptedMarkerMutex.Unlock()

	if index, exists := a.lastAcceptedMarker.Get(marker.SequenceID()); !exists || index < marker.Index() {
		a.lastAcceptedMarker.Set(marker.SequenceID(), marker.Index())
		return true
	}
	return false
}

func (a *Gadget) setMarkerConfirmed(marker markers.Marker) (wasUpdated bool) {
	// This method can be called from multiple goroutines and we need to update the value atomically.
	// However, when reading lastConfirmedMarker we don't need to lock because the storage is already locking.
	a.lastConfirmedMarkerMutex.Lock()
	defer a.lastConfirmedMarkerMutex.Unlock()

	if index, exists := a.lastConfirmedMarker.Get(marker.SequenceID()); !exists || index < marker.Index() {
		a.lastConfirmedMarker.Set(marker.SequenceID(), marker.Index())
		return true
	}
	return false
}

func (a *Gadget) acceptanceFailed(block *Block, err error) {
	a.Events.Error.Trigger(errors.Wrapf(err, "could not mark block %s as accepted", block.ID()))
}

func (a *Gadget) confirmationFailed(block *Block, err error) {
	a.Events.Error.Trigger(errors.Wrapf(err, "could not mark block %s as confirmed", block.ID()))
}

func (a *Gadget) evictSequence(sequenceID markers.SequenceID) {
	a.evictionMutex.Lock()
	defer a.evictionMutex.Unlock()

	a.lastAcceptedMarker.Delete(sequenceID)
	a.lastConfirmedMarker.Delete(sequenceID)
}

func (a *Gadget) getOrRegisterBlock(blockID models.BlockID) (block *Block, exists bool) {
	block, exists = a.block(blockID)
	if !exists {
		virtualVotingBlock, virtualVotingBlockExists := a.tangle.Booker.Block(blockID)
		if !virtualVotingBlockExists {
			return nil, false
		}

		var err error
		block, err = a.registerBlock(virtualVotingBlock)
		if err != nil {
			a.Events.Error.Trigger(errors.Wrapf(err, "could not register block %s", blockID))
			return nil, false
		}
	}
	return block, true
}

func (a *Gadget) registerBlock(virtualVotingBlock *virtualvoting.Block) (block *Block, err error) {
	if a.evictionState.InEvictedSlot(virtualVotingBlock.ID()) {
		return nil, errors.Errorf("block %s belongs to an evicted slot", virtualVotingBlock.ID())
	}

	blockStorage := a.blocks.Get(virtualVotingBlock.ID().Index(), true)
	block, _ = blockStorage.GetOrCreate(virtualVotingBlock.ID(), func() *Block {
		return NewBlock(virtualVotingBlock)
	})

	return block, nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Conflict Acceptance //////////////////////////////////////////////////////////////////////////////////////////

func (a *Gadget) RefreshConflictAcceptance(conflictID utxo.TransactionID) {
	conflict, exists := a.tangle.Booker.Ledger.ConflictDAG.Conflict(conflictID)
	if !exists {
		return
	}

	conflictWeight := a.tangle.Booker.VirtualVoting.ConflictVotersTotalWeight(conflictID)

	if !IsThresholdReached(a.tangle.Booker.VirtualVoting.Validators.TotalWeight(), conflictWeight, a.optsConflictAcceptanceThreshold) {
		return
	}

	markAsAccepted := true

	conflict.ForEachConflictingConflict(func(conflictingConflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) bool {
		conflictingConflictWeight := a.tangle.Booker.VirtualVoting.ConflictVotersTotalWeight(conflictingConflict.ID())

		// if the conflict is less than 66% ahead, then don't mark as accepted
		if !IsThresholdReached(a.tangle.Booker.VirtualVoting.Validators.TotalWeight(), conflictWeight-conflictingConflictWeight, a.optsConflictAcceptanceThreshold) {
			markAsAccepted = false
		}
		return markAsAccepted
	})

	if markAsAccepted {
		a.tangle.Booker.Ledger.ConflictDAG.SetConflictAccepted(conflictID)
	}
}

func IsThresholdReached(weight, otherWeight int64, threshold float64) bool {
	return otherWeight > int64(float64(weight)*threshold)
}

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
