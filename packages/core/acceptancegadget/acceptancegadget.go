package acceptancegadget

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/generics/walker"

	"github.com/iotaledger/goshimmer/packages/core/causalorder"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/eviction"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/core/markers"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/tangle"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
	"github.com/iotaledger/goshimmer/packages/core/tangle/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/core/votes"
)

const defaultMarkerAcceptanceThreshold = 0.66
const defaultConflictAcceptanceThreshold = 0.66

// region AcceptanceGadget /////////////////////////////////////////////////////////////////////////////////////////////

type AcceptanceGadget struct {
	Events                          *Events
	Tangle                          *tangle.Tangle
	EvictionManager                 *eviction.LockableManager[models.BlockID]
	blocks                          *memstorage.EpochStorage[models.BlockID, *Block]
	lastAcceptedMarker              *memstorage.Storage[markers.SequenceID, markers.Index]
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
		a.acceptanceOrder = causalorder.New(a.EvictionManager.Manager, a.Block, (*Block).Accepted, a.markAsAccepted, a.acceptanceFailed)
	}, (*AcceptanceGadget).setupEvents)
}

func (a *AcceptanceGadget) setupEvents() {
	a.Tangle.VirtualVoting.Events.SequenceVoterAdded.Attach(event.NewClosure[*votes.SequenceVotersUpdatedEvent](func(evt *votes.SequenceVotersUpdatedEvent) {
		a.RefreshSequenceAcceptance(evt.SequenceID, evt.NewMaxSupportedIndex, evt.PrevMaxSupportedIndex)
	}))

	a.Tangle.VirtualVoting.Events.ConflictVoterAdded.Attach(event.NewClosure[*votes.ConflictVoterEvent[utxo.TransactionID]](func(evt *votes.ConflictVoterEvent[utxo.TransactionID]) {
		a.RefreshConflictAcceptance(evt.ConflictID)
	}))

	a.Tangle.Booker.Events.SequenceEvicted.Attach(event.NewClosure(a.evictSequence))
	a.EvictionManager.Events.EpochEvicted.Attach(event.NewClosure(a.evictEpoch))
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
	if exists && block.Accepted() {
		return true
	}
	return false
}

func (a *AcceptanceGadget) isMarkerAccepted(marker markers.Marker) bool {
	bookerBlock, exists := a.Tangle.BlockFromMarker(marker)
	if !exists {
		return false
	}

	return a.isBlockAccepted(bookerBlock.ID())
}

func (a *AcceptanceGadget) FirstUnacceptedIndex(sequenceID markers.SequenceID) (firstUnacceptedIndex markers.Index) {
	firstAcceptedIndex, exists := a.lastAcceptedMarker.Get(sequenceID)
	if !exists {
		return 0
	}

	return firstAcceptedIndex + 1
}

// Block retrieves a Block with metadata from the in-memory storage of the AcceptanceGadget.
func (a *AcceptanceGadget) Block(id models.BlockID) (block *Block, exists bool) {
	a.EvictionManager.RLock()
	defer a.EvictionManager.RUnlock()

	return a.block(id)
}

func (a *AcceptanceGadget) RefreshSequenceAcceptance(sequenceID markers.SequenceID, newMaxSupportedIndex, prevMaxSupportedIndex markers.Index) {
	a.EvictionManager.RLock()
	defer a.EvictionManager.RUnlock()

	for markerIndex := prevMaxSupportedIndex + 1; markerIndex <= newMaxSupportedIndex; markerIndex++ {
		marker := markers.NewMarker(sequenceID, markerIndex)

		markerVoters := a.Tangle.VirtualVoting.MarkerVoters(marker)
		markerWeight := TotalVotersWeight(markerVoters)

		if markerWeight > uint64(float64(a.Tangle.ValidatorSet.TotalWeight())*a.optsMarkerAcceptanceThreshold) && !a.isMarkerAccepted(marker) {
			err := a.propagateAcceptance(marker)
			if err != nil {
				a.Events.Error.Trigger(errors.Wrap(err, "could not mark past cone blocks as accepted"))
			} else {
				a.lastAcceptedMarker.Set(sequenceID, markerIndex)
			}

		}
	}
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

func (a *AcceptanceGadget) propagateAcceptance(marker markers.Marker) (err error) {
	//TODO: should tangle expose BlockFromMarker that returns virtualvoting.Block?
	bookerBlock, _ := a.Tangle.BlockFromMarker(marker)
	virtualVotingBlock, _ := a.Tangle.Block(bookerBlock.ID())

	block, err := a.getOrRegisterBlock(virtualVotingBlock)
	if err != nil {
		return errors.Wrap(err, "could not mark past cone as accepted")
	}

	pastConeWalker := walker.New[*Block](false).Push(block)
	for pastConeWalker.HasNext() {
		walkerBlock := pastConeWalker.Next()

		a.acceptanceOrder.Queue(walkerBlock)

		for _, parentBlockID := range walkerBlock.Parents() {
			if a.isBlockAccepted(parentBlockID) {
				continue
			}

			parentBlockBooker, parentExists := a.Tangle.Block(parentBlockID)
			if !parentExists {
				return errors.Errorf("parent block %s does not exist", parentBlockID)
			}

			parentBlock, parentErr := a.getOrRegisterBlock(parentBlockBooker)
			if parentErr != nil {
				return errors.Wrap(parentErr, "could not mark past cone as accepted")
			}

			pastConeWalker.Push(parentBlock)
		}
	}

	return nil
}

func (a *AcceptanceGadget) getOrRegisterBlock(virtualVotingBlock *virtualvoting.Block) (block *Block, err error) {
	block, exists := a.block(virtualVotingBlock.ID())
	if !exists {
		block, err = a.registerBlock(virtualVotingBlock)
		if err != nil {
			return nil, err
		}
	}
	return block, nil
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
	a.EvictionManager.Lock()
	defer a.EvictionManager.Unlock()

	a.acceptanceOrder.EvictEpoch(index)
	a.blocks.EvictEpoch(index)
}

func (a *AcceptanceGadget) evictSequence(sequenceID markers.SequenceID) {
	a.EvictionManager.Lock()
	defer a.EvictionManager.Unlock()

	if !a.lastAcceptedMarker.Delete(sequenceID) {
		a.Events.Error.Trigger(errors.Errorf("could not evict sequenceID=%s", sequenceID))
	}
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
	conflictWeight := TotalVotersWeight(conflictVoters)

	if conflictWeight <= uint64(float64(a.Tangle.ValidatorSet.TotalWeight())*a.optsConflictAcceptanceThreshold) {
		return
	}

	markAsAccepted := true
	otherConflictAccepted := false

	a.Tangle.ConflictDAG.Utils.ForEachConflictingConflictID(conflictID, func(conflictingConflictID utxo.TransactionID) bool {
		// check if another conflict is accepted, to evaluate reorg condition
		otherConflictAccepted = otherConflictAccepted || a.Tangle.ConflictDAG.ConfirmationState(set.NewAdvancedSet(conflictingConflictID)).IsAccepted()

		conflictingConflictVoters := a.Tangle.VirtualVoting.ConflictVoters(conflictingConflictID)
		conflictingConflictWeight := TotalVotersWeight(conflictingConflictVoters)

		// if 66% ahead of ALL conflicting conflicts, then set accepted
		if conflictWeight-conflictingConflictWeight <= uint64(float64(a.Tangle.ValidatorSet.TotalWeight())*a.optsConflictAcceptanceThreshold) {
			markAsAccepted = false
		}

		return markAsAccepted
	})

	// check if previously accepted conflict is differnet from the newly accepted one, then trigger the reorg
	if markAsAccepted && otherConflictAccepted {
		a.Events.Error.Trigger(errors.Errorf("conflictID %s needs to be reorg-ed, but functionality not implemented yet!", conflictID))
		return
	}

	if markAsAccepted {
		a.Tangle.ConflictDAG.SetConflictAccepted(conflictID)
	}
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

// region Utils ////////////////////////////////////////////////////////////////////////////////////////////////////////

func TotalVotersWeight(voters *set.AdvancedSet[*validator.Validator]) uint64 {
	var weight uint64
	_ = voters.ForEach(func(validator *validator.Validator) error {
		weight += validator.Weight()
		return nil
	})
	return weight
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
