package acceptancegadget

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/generics/walker"

	"github.com/iotaledger/goshimmer/packages/core/causalorder"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/votes"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/protocol/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
)

const (
	defaultMarkerAcceptanceThreshold   = 0.67
	defaultConflictAcceptanceThreshold = 0.67
)

// region AcceptanceGadget /////////////////////////////////////////////////////////////////////////////////////////////

type AcceptanceGadget struct {
	Events                          *Events
	Tangle                          *tangle.Tangle
	EvictionManager                 *eviction.LockableManager[models.BlockID]
	blocks                          *memstorage.EpochStorage[models.BlockID, *Block]
	lastAcceptedMarker              *memstorage.Storage[markers.SequenceID, markers.Index]
	lastAcceptedMarkerMutex         sync.Mutex
	acceptanceOrder                 *causalorder.CausalOrder[models.BlockID, *Block]
	optsMarkerAcceptanceThreshold   float64
	optsConflictAcceptanceThreshold float64
}

func New(tangle *tangle.Tangle, opts ...options.Option[AcceptanceGadget]) *AcceptanceGadget {
	return options.Apply(&AcceptanceGadget{
		Events:                          newEvents(),
		Tangle:                          tangle,
		EvictionManager:                 tangle.EvictionManager.Lockable(),
		lastAcceptedMarker:              memstorage.New[markers.SequenceID, markers.Index](),
		blocks:                          memstorage.NewEpochStorage[models.BlockID, *Block](),
		optsMarkerAcceptanceThreshold:   defaultMarkerAcceptanceThreshold,
		optsConflictAcceptanceThreshold: defaultConflictAcceptanceThreshold,
	}, opts, func(a *AcceptanceGadget) {
		a.acceptanceOrder = causalorder.New(a.EvictionManager.Manager, a.GetOrRegisterBlock, (*Block).Accepted, a.markAsAccepted, a.acceptanceFailed)
	}, (*AcceptanceGadget).setupEvents)
}

// IsMarkerAccepted returns whether the given marker is accepted.
func (a *AcceptanceGadget) IsMarkerAccepted(marker markers.Marker) (accepted bool) {
	a.EvictionManager.RLock()
	defer a.EvictionManager.RUnlock()

	return a.isMarkerAccepted(marker)
}

// IsBlockAccepted returns whether the given block is accepted.
func (a *AcceptanceGadget) IsBlockAccepted(blockID models.BlockID) (accepted bool) {
	a.EvictionManager.RLock()
	defer a.EvictionManager.RUnlock()

	return a.isBlockAccepted(blockID)
}

func (a *AcceptanceGadget) isBlockAccepted(blockID models.BlockID) bool {
	block, exists := a.block(blockID)
	return exists && block.Accepted()
}

func (a *AcceptanceGadget) isMarkerAccepted(marker markers.Marker) bool {
	lastAcceptedIndex, exists := a.lastAcceptedMarker.Get(marker.SequenceID())
	return exists && lastAcceptedIndex >= marker.Index()
}

func (a *AcceptanceGadget) FirstUnacceptedIndex(sequenceID markers.SequenceID) (firstUnacceptedIndex markers.Index) {
	lastAcceptedIndex, exists := a.lastAcceptedMarker.Get(sequenceID)
	if !exists {
		return 0
	}

	return lastAcceptedIndex + 1
}

// Block retrieves a Block with metadata from the in-memory storage of the AcceptanceGadget.
func (a *AcceptanceGadget) Block(id models.BlockID) (block *Block, exists bool) {
	a.EvictionManager.RLock()
	defer a.EvictionManager.RUnlock()

	return a.block(id)
}

func (a *AcceptanceGadget) GetOrRegisterBlock(blockID models.BlockID) (block *Block, exists bool) {
	a.EvictionManager.RLock()
	defer a.EvictionManager.RUnlock()

	return a.getOrRegisterBlock(blockID)
}

func (a *AcceptanceGadget) RefreshSequenceAcceptance(sequenceID markers.SequenceID, newMaxSupportedIndex, prevMaxSupportedIndex markers.Index) {
	a.EvictionManager.RLock()
	defer a.EvictionManager.RUnlock()
	for markerIndex := prevMaxSupportedIndex; markerIndex <= newMaxSupportedIndex; markerIndex++ {
		if markerIndex == 0 {
			continue
		}

		marker := markers.NewMarker(sequenceID, markerIndex)

		markerVoters := a.Tangle.VirtualVoting.MarkerVoters(marker)
		if a.Tangle.ValidatorSet.IsThresholdReached(markerVoters.TotalWeight(), a.optsMarkerAcceptanceThreshold) && a.setMarkerAccepted(marker) {
			a.propagateAcceptance(marker)
		}
	}
}

func (a *AcceptanceGadget) setupEvents() {
	a.Tangle.VirtualVoting.Events.SequenceVoterUpdated.Attach(event.NewClosure[*votes.SequenceVotersUpdatedEvent](func(evt *votes.SequenceVotersUpdatedEvent) {
		a.RefreshSequenceAcceptance(evt.SequenceID, evt.NewMaxSupportedIndex, evt.PrevMaxSupportedIndex)
	}))

	a.Tangle.VirtualVoting.Events.ConflictVoterAdded.Attach(event.NewClosure[*votes.ConflictVoterEvent[utxo.TransactionID]](func(evt *votes.ConflictVoterEvent[utxo.TransactionID]) {
		a.RefreshConflictAcceptance(evt.ConflictID)
	}))

	a.Tangle.Booker.Events.SequenceEvicted.Attach(event.NewClosure(a.evictSequence))
	a.EvictionManager.Events.EpochEvicted.Attach(event.NewClosure(a.evictEpoch))
}

func (a *AcceptanceGadget) block(id models.BlockID) (block *Block, exists bool) {
	if a.EvictionManager.IsRootBlock(id) {
		virtualVotingBlock, _ := a.Tangle.Block(id)

		return NewBlock(virtualVotingBlock, WithAccepted(true)), true
	}

	storage := a.blocks.Get(id.Index(), false)
	if storage == nil {
		return nil, false
	}

	return storage.Get(id)
}

func (a *AcceptanceGadget) propagateAcceptance(marker markers.Marker) {
	bookerBlock, blockExists := a.Tangle.BlockFromMarker(marker)
	if !blockExists {
		return
	}

	block, _ := a.getOrRegisterBlock(bookerBlock.ID())

	pastConeWalker := walker.New[*Block](false).Push(block)
	for pastConeWalker.HasNext() {
		walkerBlock := pastConeWalker.Next()
		a.acceptanceOrder.Queue(walkerBlock)

		for _, parentBlockID := range walkerBlock.Parents() {
			if a.isBlockAccepted(parentBlockID) {
				continue
			}

			parentBlock, _ := a.getOrRegisterBlock(parentBlockID)
			pastConeWalker.Push(parentBlock)
		}
	}
}

func (a *AcceptanceGadget) markAsAccepted(block *Block) (err error) {
	if a.EvictionManager.IsTooOld(block.ID()) {
		return errors.Errorf("block with %s belongs to an evicted epoch", block.ID())
	}
	if block.SetAccepted() {
		a.Events.BlockAccepted.Trigger(block)

		// set ConfirmationState of payload (applicable only to transactions)
		if tx, ok := block.Payload().(*devnetvm.Transaction); ok {
			a.Tangle.Ledger.SetTransactionInclusionTime(tx.ID(), block.IssuingTime())
		}
	}

	return nil
}

func (a *AcceptanceGadget) acceptanceFailed(block *Block, err error) {
	a.Events.Error.Trigger(errors.Wrapf(err, "could not mark block %s as accepted", block.ID()))
}

func (a *AcceptanceGadget) evictEpoch(index epoch.Index) {
	a.acceptanceOrder.EvictEpoch(index)

	a.EvictionManager.Lock()
	defer a.EvictionManager.Unlock()

	a.blocks.EvictEpoch(index)
}

func (a *AcceptanceGadget) evictSequence(sequenceID markers.SequenceID) {
	a.EvictionManager.Lock()
	defer a.EvictionManager.Unlock()

	if !a.lastAcceptedMarker.Delete(sequenceID) {
		a.Events.Error.Trigger(errors.Errorf("could not evict sequenceID=%s", sequenceID))
	}
}

func (a *AcceptanceGadget) getOrRegisterBlock(blockID models.BlockID) (block *Block, exists bool) {
	block, exists = a.block(blockID)
	if !exists {
		virtualVotingBlock, virtualVotingBlockExists := a.Tangle.Block(blockID)
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

func (a *AcceptanceGadget) registerBlock(virtualVotingBlock *virtualvoting.Block) (block *Block, err error) {
	if a.EvictionManager.IsTooOld(virtualVotingBlock.ID()) {
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

func (a *AcceptanceGadget) RefreshConflictAcceptance(conflictID utxo.TransactionID) {
	conflictVoters := a.Tangle.VirtualVoting.ConflictVoters(conflictID)
	conflictWeight := conflictVoters.TotalWeight()
	if !a.Tangle.ValidatorSet.IsThresholdReached(conflictWeight, a.optsConflictAcceptanceThreshold) {
		return
	}

	markAsAccepted := true
	isOtherConflictAccepted := false
	var otherAcceptedConflict utxo.TransactionID

	a.Tangle.Booker.Ledger.ConflictDAG.Utils.ForEachConflictingConflictID(conflictID, func(conflictingConflictID utxo.TransactionID) bool {
		// check if another conflict is accepted, to evaluate reorg condition
		if !isOtherConflictAccepted && a.Tangle.Booker.Ledger.ConflictDAG.ConfirmationState(set.NewAdvancedSet(conflictingConflictID)).IsAccepted() {
			isOtherConflictAccepted = true
			otherAcceptedConflict = conflictingConflictID
		}

		conflictingConflictVoters := a.Tangle.VirtualVoting.ConflictVoters(conflictingConflictID)

		// if 66% ahead of ALL conflicting conflicts, then set accepted
		if conflictingConflictWeight := conflictingConflictVoters.TotalWeight(); !a.Tangle.ValidatorSet.IsThresholdReached(conflictWeight-conflictingConflictWeight, a.optsConflictAcceptanceThreshold) {
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
		a.Tangle.Booker.Ledger.ConflictDAG.SetConflictAccepted(conflictID)
	}
}

func (a *AcceptanceGadget) setMarkerAccepted(marker markers.Marker) (wasUpdated bool) {
	a.lastAcceptedMarkerMutex.Lock()
	defer a.lastAcceptedMarkerMutex.Unlock()

	if index, exists := a.lastAcceptedMarker.Get(marker.SequenceID()); !exists || index < marker.Index() {
		a.lastAcceptedMarker.Set(marker.SequenceID(), marker.Index())
		return true
	}
	return false
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithMarkerAcceptanceThreshold(acceptanceThreshold float64) options.Option[AcceptanceGadget] {
	return func(gadget *AcceptanceGadget) {
		gadget.optsMarkerAcceptanceThreshold = acceptanceThreshold
	}
}

func WithConflictAcceptanceThreshold(acceptanceThreshold float64) options.Option[AcceptanceGadget] {
	return func(gadget *AcceptanceGadget) {
		gadget.optsConflictAcceptanceThreshold = acceptanceThreshold
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
