package acceptance

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/generics/walker"

	"github.com/iotaledger/goshimmer/packages/core/causalorder"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/eviction"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/core/votes/conflicttracker"
	"github.com/iotaledger/goshimmer/packages/core/votes/sequencetracker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// region Gadget ///////////////////////////////////////////////////////////////////////////////////////////////////////

type Gadget struct {
	Events *Events

	tangle                   *tangle.Tangle
	evictionState            *eviction.LockableState[models.BlockID]
	blocks                   *memstorage.EpochStorage[models.BlockID, *Block]
	lastAcceptedMarker       *memstorage.Storage[markers.SequenceID, markers.Index]
	lastAcceptedMarkerMutex  sync.Mutex
	lastConfirmedMarker      *memstorage.Storage[markers.SequenceID, markers.Index]
	lastConfirmedMarkerMutex sync.Mutex
	totalWeightCallback      func() int64

	optsMarkerAcceptanceThreshold   float64
	optsConfirmationThreshold       float64
	optsConflictAcceptanceThreshold float64
	acceptanceOrder                 *causalorder.CausalOrder[models.BlockID, *Block]
	confirmationOrder               *causalorder.CausalOrder[models.BlockID, *Block]
}

func New(tangle *tangle.Tangle, totalWeightCallback func() int64, opts ...options.Option[Gadget]) (gadget *Gadget) {
	return options.Apply(&Gadget{
		optsMarkerAcceptanceThreshold:   0.67,
		optsConfirmationThreshold:       0.67,
		optsConflictAcceptanceThreshold: 0.67,
	}, opts, func(a *Gadget) {
		a.Events = NewEvents()

		a.tangle = tangle
		a.totalWeightCallback = totalWeightCallback
		a.evictionState = tangle.EvictionState.Lockable()
		a.lastAcceptedMarker = memstorage.New[markers.SequenceID, markers.Index]()
		a.lastConfirmedMarker = memstorage.New[markers.SequenceID, markers.Index]()
		a.blocks = memstorage.NewEpochStorage[models.BlockID, *Block]()

		a.acceptanceOrder = causalorder.New(a.evictionState.State, a.GetOrRegisterBlock, (*Block).IsAccepted, a.markAsAccepted, a.acceptanceFailed)
		a.confirmationOrder = causalorder.New(a.evictionState.State, a.GetOrRegisterBlock, (*Block).IsConfirmed, a.markAsConfirmed, a.acceptanceFailed)
	}, (*Gadget).setup)
}

// IsMarkerAccepted returns whether the given marker is accepted.
func (a *Gadget) IsMarkerAccepted(marker markers.Marker) (accepted bool) {
	a.evictionState.RLock()
	defer a.evictionState.RUnlock()

	return a.isMarkerAccepted(marker)
}

// IsMarkerConfirmed returns whether the given marker is confirmed.
func (a *Gadget) IsMarkerConfirmed(marker markers.Marker) (confirmed bool) {
	a.evictionState.RLock()
	defer a.evictionState.RUnlock()

	return a.isMarkerConfirmed(marker)
}

// IsBlockAccepted returns whether the given block is accepted.
func (a *Gadget) IsBlockAccepted(blockID models.BlockID) (accepted bool) {
	a.evictionState.RLock()
	defer a.evictionState.RUnlock()

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
	a.evictionState.RLock()
	defer a.evictionState.RUnlock()

	return a.block(id)
}

func (a *Gadget) GetOrRegisterBlock(blockID models.BlockID) (block *Block, exists bool) {
	a.evictionState.RLock()
	defer a.evictionState.RUnlock()

	return a.getOrRegisterBlock(blockID)
}

func (a *Gadget) RefreshSequenceAcceptance(sequenceID markers.SequenceID, newMaxSupportedIndex, prevMaxSupportedIndex markers.Index) {
	a.evictionState.RLock()

	var acceptedBlocks, confirmedBlocks []*Block

	totalWeight := a.totalWeightCallback()

	for markerIndex := prevMaxSupportedIndex; markerIndex <= newMaxSupportedIndex; markerIndex++ {
		if markerIndex == 0 {
			continue
		}

		marker := markers.NewMarker(sequenceID, markerIndex)

		markerVoters := a.tangle.VirtualVoting.MarkerVoters(marker)

		if validator.IsThresholdReached(totalWeight, a.tangle.ValidatorSet.TotalWeight(), a.optsConfirmationThreshold) {
			// have enough weight to confirm based on total weight
			if validator.IsThresholdReached(totalWeight, markerVoters.TotalWeight(), a.optsConfirmationThreshold) {
				// need to mark outside of if statement, otherwise only the first part would be executed
				markerAccepted := a.setMarkerAccepted(marker)
				markerConfirmed := a.setMarkerConfirmed(marker)
				if markerAccepted || markerConfirmed {
					blocksToAccept, blocksToConfirm := a.propagateAcceptance(marker, true)
					acceptedBlocks = append(acceptedBlocks, blocksToAccept...)
					confirmedBlocks = append(confirmedBlocks, blocksToConfirm...)
				}
			}
		} else if a.tangle.ValidatorSet.IsThresholdReached(markerVoters.TotalWeight(), a.optsMarkerAcceptanceThreshold) && a.setMarkerAccepted(marker) {
			blocksToAccept, _ := a.propagateAcceptance(marker, false)
			acceptedBlocks = append(acceptedBlocks, blocksToAccept...)

		}
	}
	a.evictionState.RUnlock()
	// EVICTION
	for _, block := range acceptedBlocks {
		a.acceptanceOrder.Queue(block)
	}
	for _, block := range confirmedBlocks {
		a.confirmationOrder.Queue(block)
	}
}

func (a *Gadget) setup() {
	a.tangle.VirtualVoting.Events.SequenceTracker.VotersUpdated.Attach(event.NewClosure(func(evt *sequencetracker.VoterUpdatedEvent) {
		a.RefreshSequenceAcceptance(evt.SequenceID, evt.NewMaxSupportedIndex, evt.PrevMaxSupportedIndex)
	}))

	a.tangle.VirtualVoting.Events.ConflictTracker.VoterAdded.Attach(event.NewClosure(func(evt *conflicttracker.VoterEvent[utxo.TransactionID]) {
		a.RefreshConflictAcceptance(evt.ConflictID)
	}))

	a.tangle.Booker.Events.SequenceEvicted.Attach(event.NewClosure(a.evictSequence))

	a.evictionState.Events.EpochEvicted.Attach(event.NewClosure(a.evictEpoch))
}

func (a *Gadget) block(id models.BlockID) (block *Block, exists bool) {
	if a.evictionState.IsRootBlock(id) {
		return NewRootBlock(id), true
	}

	storage := a.blocks.Get(id.Index(), false)
	if storage == nil {
		return nil, false
	}

	return storage.Get(id)
}

func (a *Gadget) propagateAcceptance(marker markers.Marker, confirmed bool) (acceptedBlocks, confirmedBlocks []*Block) {
	bookerBlock, blockExists := a.tangle.BlockFromMarker(marker)
	if !blockExists {
		return
	}

	block, blockExists := a.GetOrRegisterBlock(bookerBlock.ID())
	if !blockExists || block.IsAccepted() && !confirmed || block.IsConfirmed() && confirmed {
		// this can happen when block was a root block and while processing this method, the root blocks method has already been replaced
		return
	}

	pastConeWalker := walker.New[*Block](false).Push(block)
	for pastConeWalker.HasNext() {
		walkerBlock := pastConeWalker.Next()

		var acceptanceQueued, confirmationQueued bool

		if acceptanceQueued = walkerBlock.SetAcceptanceQueued(); acceptanceQueued {
			acceptedBlocks = append(acceptedBlocks, walkerBlock)
		}

		if confirmed {
			if confirmationQueued = walkerBlock.SetConfirmationQueued(); confirmationQueued {
				confirmedBlocks = append(confirmedBlocks, walkerBlock)
			}
		}

		if !acceptanceQueued && !confirmationQueued {
			continue
		}

		for _, parentBlockID := range walkerBlock.Parents() {
			if !confirmed && a.isBlockAccepted(parentBlockID) || confirmed && a.isBlockConfirmed(parentBlockID) {
				continue
			}

			parentBlock, parentExists := a.getOrRegisterBlock(parentBlockID)
			if parentExists {
				pastConeWalker.Push(parentBlock)
			}
		}
	}

	return
}

func (a *Gadget) markAsAccepted(block *Block) (err error) {
	if a.evictionState.IsTooOld(block.ID()) {
		return errors.Errorf("block with %s belongs to an evicted epoch", block.ID())
	}

	if block.SetAccepted() {
		// If block has been orphaned before acceptance, remove the flag from the block. Otherwise, remove the block from TimedHeap.
		if block.IsExplicitlyOrphaned() {
			a.tangle.SetOrphaned(block.Block.Block.Block, false)
		}

		a.Events.BlockAccepted.Trigger(block)

		// set ConfirmationState of payload (applicable only to transactions)
		if tx, ok := block.Payload().(*devnetvm.Transaction); ok {
			a.tangle.Ledger.SetTransactionInclusionTime(tx.ID(), block.IssuingTime())
		}
	}

	return nil
}

func (a *Gadget) markAsConfirmed(block *Block) (err error) {
	if a.evictionState.IsTooOld(block.ID()) {
		return errors.Errorf("block with %s belongs to an evicted epoch", block.ID())
	}

	if block.SetConfirmed() {
		a.Events.BlockConfirmed.Trigger(block)
	}

	return nil
}

func (a *Gadget) acceptanceFailed(block *Block, err error) {
	a.Events.Error.Trigger(errors.Wrapf(err, "could not mark block %s as accepted", block.ID()))
}

func (a *Gadget) evictEpoch(index epoch.Index) {
	a.acceptanceOrder.EvictEpoch(index)

	a.evictionState.Lock()
	defer a.evictionState.Unlock()

	storage := a.blocks.Get(index, false)
	if storage != nil {
		a.Events.EpochClosed.Trigger(storage)
	}
	a.blocks.EvictEpoch(index)
}

func (a *Gadget) evictSequence(sequenceID markers.SequenceID) {
	a.evictionState.Lock()
	defer a.evictionState.Unlock()

	if !a.lastAcceptedMarker.Delete(sequenceID) {
		a.Events.Error.Trigger(errors.Errorf("could not evict last accepted marker of sequenceID=%s", sequenceID))
	}

	if !a.lastConfirmedMarker.Delete(sequenceID) {
		a.Events.Error.Trigger(errors.Errorf("could not evict last confirmed marker of sequenceID=%s", sequenceID))
	}
}

func (a *Gadget) getOrRegisterBlock(blockID models.BlockID) (block *Block, exists bool) {
	block, exists = a.block(blockID)
	if !exists {
		virtualVotingBlock, virtualVotingBlockExists := a.tangle.VirtualVoting.Block(blockID)
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
	if a.evictionState.IsTooOld(virtualVotingBlock.ID()) {
		return nil, errors.Errorf("block %s belongs to an evicted epoch", virtualVotingBlock.ID())
	}

	blockStorage := a.blocks.Get(virtualVotingBlock.ID().Index(), true)
	block, _ = blockStorage.RetrieveOrCreate(virtualVotingBlock.ID(), func() *Block {
		return NewBlock(virtualVotingBlock)
	})

	return block, nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Conflict Acceptance //////////////////////////////////////////////////////////////////////////////////////////

func (a *Gadget) RefreshConflictAcceptance(conflictID utxo.TransactionID) {
	conflictVoters := a.tangle.VirtualVoting.ConflictVoters(conflictID)
	conflictWeight := conflictVoters.TotalWeight()
	if !a.tangle.ValidatorSet.IsThresholdReached(conflictWeight, a.optsConflictAcceptanceThreshold) {
		return
	}

	markAsAccepted := true
	isOtherConflictAccepted := false
	var otherAcceptedConflict utxo.TransactionID

	a.tangle.Booker.Ledger.ConflictDAG.Utils.ForEachConflictingConflictID(conflictID, func(conflictingConflictID utxo.TransactionID) bool {
		// check if another conflict is accepted, to evaluate reorg condition
		if !isOtherConflictAccepted && a.tangle.Booker.Ledger.ConflictDAG.ConfirmationState(set.NewAdvancedSet(conflictingConflictID)).IsAccepted() {
			isOtherConflictAccepted = true
			otherAcceptedConflict = conflictingConflictID
		}

		conflictingConflictVoters := a.tangle.VirtualVoting.ConflictVoters(conflictingConflictID)

		// if 66% ahead of ALL conflicting conflicts, then set accepted
		if conflictingConflictWeight := conflictingConflictVoters.TotalWeight(); !a.tangle.ValidatorSet.IsThresholdReached(conflictWeight-conflictingConflictWeight, a.optsConflictAcceptanceThreshold) {
			markAsAccepted = false
		}

		return markAsAccepted
	})

	// check if previously accepted conflict is different from the newly accepted one, then trigger the reorg
	if markAsAccepted && isOtherConflictAccepted {
		a.Events.Error.Trigger(errors.Errorf("conflictID %s needs to be reorg-ed, but functionality not implemented yet!", conflictID))
		a.Events.Reorg.Trigger(otherAcceptedConflict)
		return
	}

	if markAsAccepted {
		a.tangle.Booker.Ledger.ConflictDAG.SetConflictAccepted(conflictID)
	}
}

func (a *Gadget) setMarkerAccepted(marker markers.Marker) (wasUpdated bool) {
	a.lastAcceptedMarkerMutex.Lock()
	defer a.lastAcceptedMarkerMutex.Unlock()

	if index, exists := a.lastAcceptedMarker.Get(marker.SequenceID()); !exists || index < marker.Index() {
		a.lastAcceptedMarker.Set(marker.SequenceID(), marker.Index())
		return true
	}
	return false
}
func (a *Gadget) setMarkerConfirmed(marker markers.Marker) (wasUpdated bool) {
	a.lastConfirmedMarkerMutex.Lock()
	defer a.lastConfirmedMarkerMutex.Unlock()

	if index, exists := a.lastConfirmedMarker.Get(marker.SequenceID()); !exists || index < marker.Index() {
		a.lastConfirmedMarker.Set(marker.SequenceID(), marker.Index())
		return true
	}
	return false
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
		gadget.optsConfirmationThreshold = confirmationThreshold
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
