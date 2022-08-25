package acceptancegadget

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/walker"
	"github.com/iotaledger/hive.go/core/syncutils"

	"github.com/iotaledger/goshimmer/packages/core/causalorder"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/eviction"
	"github.com/iotaledger/goshimmer/packages/core/markers"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
	"github.com/iotaledger/goshimmer/packages/core/tangle/otv"
	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/core/votes"
)

type AcceptanceGadget struct {
	Events *Events

	blocks     *memstorage.EpochStorage[models.BlockID, *Block]
	pruningMap *memstorage.Storage[epoch.Index, *markers.Markers]
	votersMap  *memstorage.Storage[markers.Marker, *validator.Set]

	lastAcceptedMarker *memstorage.Storage[markers.SequenceID, markers.Index]

	validatorSet           *validator.Set
	markerBlockMappingFunc func(marker markers.Marker) (block *booker.Block, exists bool)

	acceptanceOrder *causalorder.CausalOrder[models.BlockID, *Block]

	otv             otv.OnTangleVoting
	evictionManager *eviction.LockableManager[models.BlockID]
	sequenceMutex   *syncutils.DAGMutex[markers.SequenceID]

	optsMarkerAcceptanceThreshold float64
}

func New(validatorSet *validator.Set, evictionManager *eviction.Manager[models.BlockID], otv otv.OnTangleVoting, markerBlockMappingFunc func(marker markers.Marker) (block *booker.Block, exists bool), opts ...options.Option[AcceptanceGadget]) *AcceptanceGadget {
	gadget := options.Apply(&AcceptanceGadget{
		Events: newEvents(),

		validatorSet:           validatorSet,
		markerBlockMappingFunc: markerBlockMappingFunc,

		pruningMap:         memstorage.New[epoch.Index, *markers.Markers](),
		votersMap:          memstorage.New[markers.Marker, *validator.Set](),
		lastAcceptedMarker: memstorage.New[markers.SequenceID, markers.Index](),
		otv:                otv,

		evictionManager: evictionManager.Lockable(),
	}, opts)

	gadget.acceptanceOrder = causalorder.New(evictionManager, gadget.Block, (*Block).Accepted, gadget.markAsAccepted, gadget.acceptanceFailed)

	otv.Events.SequenceVoterAdded.Attach(event.NewClosure[*votes.SequenceVoterEvent](func(evt *votes.SequenceVoterEvent) {
		gadget.update(evt.Voter, evt.NewMaxSupportedMarker, evt.PrevMaxSupportedMarker)
	}))
	gadget.evictionManager.Events.EpochEvicted.Attach(event.NewClosure(gadget.evictEpoch))

	return gadget
}

// IsMarkerAccepted returns whether the given marker is accepted.
func (a *AcceptanceGadget) IsMarkerAccepted(marker markers.Marker) (accepted bool) {
	a.evictionManager.RLock()
	defer a.evictionManager.RUnlock()

	return a.isMarkerAccepted(marker)
}

// IsBlockAccepted returns whether the given block is accepted.
func (a *AcceptanceGadget) IsBlockAccepted(blockID models.BlockID) (accepted bool) {
	a.evictionManager.RLock()
	defer a.evictionManager.RUnlock()

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
	bookerBlock, exists := a.markerBlockMappingFunc(marker)
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
	a.evictionManager.RLock()
	defer a.evictionManager.RUnlock()

	return a.block(id)
}

func (a *AcceptanceGadget) update(voter *validator.Validator, newMaxSupportedMarker, prevMaxSupportedMarker markers.Marker) {
	a.evictionManager.RLock()
	defer a.evictionManager.RUnlock()

	sequenceID := newMaxSupportedMarker.SequenceID()

	a.sequenceMutex.Lock(sequenceID)
	defer a.sequenceMutex.Unlock(sequenceID)

	// TODO: check what is received when sequence is not supported yet
	for markerIndex := prevMaxSupportedMarker.Index() + 1; markerIndex <= newMaxSupportedMarker.Index(); markerIndex++ {
		markerVoters, isEvicted := a.getMarkerVoters(sequenceID, markerIndex)
		if isEvicted {
			a.Events.Error.Trigger(errors.Errorf("failed to update acceptance for %s with vote %s", markers.NewMarker(sequenceID, markerIndex), voter))
			continue
		}

		markerVoters.Add(voter)

		if markerVoters.TotalWeight() > uint64(float64(a.validatorSet.TotalWeight())*a.optsMarkerAcceptanceThreshold) && !a.isMarkerAccepted(markers.NewMarker(sequenceID, markerIndex)) {
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
	block, mappingExists := a.markerBlockMappingFunc(markers.NewMarker(sequenceID, markerIndex))
	if !mappingExists {
		panic(errors.Errorf("marker %s is not mapped to a block", markers.NewMarker(sequenceID, markerIndex)))
	}

	if a.evictionManager.IsTooOld(block.ID()) {
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
	if a.evictionManager.IsRootBlock(id) {
		otvBlock, _ := a.otv.Block(id)

		return NewBlock(otvBlock), true
	}

	storage := a.blocks.Get(id.Index(), false)
	if storage == nil {
		return nil, false
	}

	return storage.Get(id)
}

func (a *AcceptanceGadget) propagateAcceptance(sequenceID markers.SequenceID, index markers.Index) (err error) {
	bookerBlock, _ := a.markerBlockMappingFunc(markers.NewMarker(sequenceID, index))

	block, err := a.getOrRegisterBlock(bookerBlock)
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

			parentBlockBooker, parentExists := a.otv.Booker.Block(parentBlockID)
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

func (a *AcceptanceGadget) getOrRegisterBlock(bookerBlock *booker.Block) (block *Block, err error) {
	block, exists := a.block(bookerBlock.ID())
	if !exists {
		block, err = a.registerBlock(bookerBlock)
		if err != nil {
			return nil, err
		}
	}
	return block, nil
}

func (a *AcceptanceGadget) markAsAccepted(block *Block) (err error) {
	if a.evictionManager.IsTooOld(block.ID()) {
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
	// TODO: implement me
}

func (a *AcceptanceGadget) registerBlock(bookerBlock *booker.Block) (block *Block, err error) {
	if a.evictionManager.IsTooOld(bookerBlock.ID()) {
		return nil, errors.Errorf("block %s belongs to an evicted epoch", bookerBlock.ID())
	}
	blockStorage := a.blocks.Get(bookerBlock.ID().Index(), true)

	block, _ = blockStorage.RetrieveOrCreate(bookerBlock.ID(), func() *Block {
		return NewBlock(bookerBlock)
	})

	return block, nil
}
