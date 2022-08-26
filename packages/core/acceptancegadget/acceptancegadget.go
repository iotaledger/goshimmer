package acceptancegadget

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/walker"

	"github.com/iotaledger/goshimmer/packages/core/causalorder"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/eviction"
	"github.com/iotaledger/goshimmer/packages/core/markers"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/tangle"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
	"github.com/iotaledger/goshimmer/packages/core/tangle/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/core/votes"
)

const defaultMarkerAcceptanceThreshold = 0.66

// region AcceptanceGadget /////////////////////////////////////////////////////////////////////////////////////////////

type AcceptanceGadget struct {
	Events                        *Events
	Tangle                        *tangle.Tangle
	EvictionManager               *eviction.LockableManager[models.BlockID]
	blocks                        *memstorage.EpochStorage[models.BlockID, *Block]
	lastAcceptedMarker            *memstorage.Storage[markers.SequenceID, markers.Index]
	acceptanceOrder               *causalorder.CausalOrder[models.BlockID, *Block]
	optsMarkerAcceptanceThreshold float64

	// todo remove these maps
	pruningMap *memstorage.Storage[epoch.Index, *markers.Markers]
	votersMap  *memstorage.Storage[markers.Marker, *validator.Set]
}

func New(tangle *tangle.Tangle, opts ...options.Option[AcceptanceGadget]) *AcceptanceGadget {
	return options.Apply(&AcceptanceGadget{
		Events:                        newEvents(),
		Tangle:                        tangle,
		EvictionManager:               tangle.EvictionManager.Lockable(),
		lastAcceptedMarker:            memstorage.New[markers.SequenceID, markers.Index](),
		blocks:                        memstorage.NewEpochStorage[models.BlockID, *Block](),
		optsMarkerAcceptanceThreshold: defaultMarkerAcceptanceThreshold,

		// this is not needed, use tangle instead
		pruningMap: memstorage.New[epoch.Index, *markers.Markers](),
		votersMap:  memstorage.New[markers.Marker, *validator.Set](),
	}, opts, func(a *AcceptanceGadget) {
		a.acceptanceOrder = causalorder.New(a.EvictionManager.Manager, a.Block, (*Block).Accepted, a.markAsAccepted, a.acceptanceFailed)
	}, (*AcceptanceGadget).setupEvents)
}

func (a *AcceptanceGadget) setupEvents() {
	a.Tangle.VirtualVoting.Events.SequenceVoterAdded.Attach(event.NewClosure[*votes.SequenceVoterEvent](func(evt *votes.SequenceVoterEvent) {
		a.update(evt.Voter, evt.NewMaxSupportedMarker, evt.PrevMaxSupportedMarker)
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

func (a *AcceptanceGadget) update(voter *validator.Validator, newMaxSupportedMarker, prevMaxSupportedMarker markers.Marker) {
	a.EvictionManager.RLock()
	defer a.EvictionManager.RUnlock()

	sequenceID := newMaxSupportedMarker.SequenceID()

	// TODO: check what is received when sequence is not supported yet
	for markerIndex := prevMaxSupportedMarker.Index() + 1; markerIndex <= newMaxSupportedMarker.Index(); markerIndex++ {
		markerVoters, isEvicted := a.getMarkerVoters(sequenceID, markerIndex)
		if isEvicted {
			a.Events.Error.Trigger(errors.Errorf("failed to update acceptance for %s with vote %s", markers.NewMarker(sequenceID, markerIndex), voter))
			return
		}

		markerVoters.Add(voter)
		if markerVoters.TotalWeight() > uint64(float64(a.Tangle.ValidatorSet.TotalWeight())*a.optsMarkerAcceptanceThreshold) && !a.isMarkerAccepted(markers.NewMarker(sequenceID, markerIndex)) {
			err := a.propagateAcceptance(sequenceID, markerIndex)
			if err != nil {
				a.Events.Error.Trigger(errors.Wrap(err, "could not mark past cone blocks as accepted"))
			} else {
				a.lastAcceptedMarker.Set(sequenceID, markerIndex)
			}

		}
	}
}

func (a *AcceptanceGadget) getMarkerVoters(sequenceID markers.SequenceID, markerIndex markers.Index) (markerVoters *validator.Set, isTooOld bool) {
	block, mappingExists := a.Tangle.BlockFromMarker(markers.NewMarker(sequenceID, markerIndex))
	if !mappingExists {
		panic(errors.Errorf("marker %s is not mapped to a block", markers.NewMarker(sequenceID, markerIndex)))
	}

	if a.EvictionManager.IsTooOld(block.ID()) {
		return nil, true
	}

	markerVoters, created := a.votersMap.RetrieveOrCreate(markers.NewMarker(sequenceID, markerIndex), func() *validator.Set {
		return validator.NewSet()
	})

	if created {
		epochMarkers, _ := a.pruningMap.RetrieveOrCreate(block.ID().Index(), func() *markers.Markers {
			return markers.NewMarkers()
		})
		epochMarkers.Set(sequenceID, markerIndex)

	}
	return markerVoters, false
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

func (a *AcceptanceGadget) propagateAcceptance(sequenceID markers.SequenceID, index markers.Index) (err error) {
	//TODO: should tangle expose BlockFromMarker that returns virtualvoting.Block?
	bookerBlock, _ := a.Tangle.BlockFromMarker(markers.NewMarker(sequenceID, index))
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
	block.SetAccepted()

	a.Events.BlockAccepted.Trigger(block)

	return nil
}

func (a *AcceptanceGadget) acceptanceFailed(block *Block, err error) {
	a.Events.Error.Trigger(errors.Wrapf(err, "could not mark block %s as accepted", block.ID()))
}

func (a *AcceptanceGadget) evictEpoch(index epoch.Index) {
	a.EvictionManager.Lock()
	defer a.EvictionManager.Unlock()

	// TODO: implement me
}

func (a *AcceptanceGadget) evictSequence(sequenceID markers.SequenceID) {
	a.EvictionManager.Lock()
	defer a.EvictionManager.Unlock()

	// TODO: implement me
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

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithMarkerAcceptanceThreshold(acceptanceThreshold float64) options.Option[AcceptanceGadget] {
	return func(gadget *AcceptanceGadget) {
		gadget.optsMarkerAcceptanceThreshold = acceptanceThreshold
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
