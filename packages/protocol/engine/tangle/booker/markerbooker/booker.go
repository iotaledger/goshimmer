package markerbooker

import (
	"context"
	"fmt"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/votes/sequencetracker"
	"github.com/iotaledger/goshimmer/packages/core/votes/slottracker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markerbooker/markermanager"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markerbooker/markervirtualvoting"
	"github.com/iotaledger/goshimmer/packages/protocol/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/causalorder"
	"github.com/iotaledger/hive.go/core/memstorage"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/ds/walker"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/module"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/syncutils"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

// region Booker ///////////////////////////////////////////////////////////////////////////////////////////////////////

type Booker struct {
	// Events contains the Events of Booker.
	events *booker.Events

	MemPool         mempool.MemPool
	blockDAG        blockdag.BlockDAG
	evictionState   *eviction.State
	validators      *sybilprotection.WeightedSet
	sequenceTracker *sequencetracker.SequenceTracker[booker.BlockVotePower]
	slotTracker     *slottracker.SlotTracker
	virtualVoting   *markervirtualvoting.VirtualVoting

	bookingOrder          *causalorder.CausalOrder[models.BlockID, *booker.Block]
	attachments           *attachments
	blocks                *memstorage.SlotStorage[models.BlockID, *booker.Block]
	markerManager         *markermanager.MarkerManager[models.BlockID, *booker.Block]
	bookingMutex          *syncutils.DAGMutex[models.BlockID]
	sequenceMutex         *syncutils.DAGMutex[markers.SequenceID]
	evictionMutex         sync.RWMutex
	sequenceEvictionMutex *syncutils.StarvingMutex

	optsMarkerManager []options.Option[markermanager.MarkerManager[models.BlockID, *booker.Block]]

	optsSequenceCutoffCallback func(markers.SequenceID) markers.Index
	optsSlotCutoffCallback     func() slot.Index

	workers              *workerpool.Group
	slotTimeProviderFunc func() *slot.TimeProvider

	module.Module
}

func NewProvider(opts ...options.Option[Booker]) module.Provider[*engine.Engine, booker.Booker] {
	return module.Provide(func(e *engine.Engine) booker.Booker {
		b := New(e.Workers.CreateGroup("Booker"), e.EvictionState, e.Ledger.MemPool(), e.SybilProtection.Validators(), e.SlotTimeProvider, opts...)

		e.Events.Consensus.SlotGadget.SlotConfirmed.Hook(func(index slot.Index) {
			err := e.Storage.Permanent.Settings.SetLatestConfirmedSlot(index)
			if err != nil {
				panic(err)
			}

			b.EvictSlotTracker(index)
		}, event.WithWorkerPool(e.Workers.CreatePool("Eviction", 1))) // Using just 1 worker to avoid contention

		e.HookConstructed(func() {
			b.Initialize(e.Tangle.BlockDAG())
		})

		return b
	})
}

func New(workers *workerpool.Group, evictionState *eviction.State, memPool mempool.MemPool, validators *sybilprotection.WeightedSet, slotTimeProviderFunc func() *slot.TimeProvider, opts ...options.Option[Booker]) *Booker {
	return options.Apply(&Booker{
		events:                booker.NewEvents(),
		attachments:           newAttachments(),
		blocks:                memstorage.NewSlotStorage[models.BlockID, *booker.Block](),
		bookingMutex:          syncutils.NewDAGMutex[models.BlockID](),
		sequenceMutex:         syncutils.NewDAGMutex[markers.SequenceID](),
		sequenceEvictionMutex: syncutils.NewStarvingMutex(),
		validators:            validators,
		optsMarkerManager:     make([]options.Option[markermanager.MarkerManager[models.BlockID, *booker.Block]], 0),
		optsSequenceCutoffCallback: func(sequenceID markers.SequenceID) markers.Index {
			return 1
		},

		optsSlotCutoffCallback: func() slot.Index {
			return 0
		},
		MemPool:              memPool,
		evictionState:        evictionState,
		workers:              workers,
		slotTimeProviderFunc: slotTimeProviderFunc,
	}, opts, func(b *Booker) {
		b.markerManager = markermanager.NewMarkerManager(b.optsMarkerManager...)
		b.sequenceTracker = sequencetracker.NewSequenceTracker[booker.BlockVotePower](validators, b.markerManager.SequenceManager.Sequence, b.optsSequenceCutoffCallback)
		b.slotTracker = slottracker.NewSlotTracker(b.optsSlotCutoffCallback)
		b.virtualVoting = markervirtualvoting.New(workers.CreateGroup("VirtualVoting"), memPool.ConflictDAG(), b.markerManager.SequenceManager, validators)
		b.bookingOrder = causalorder.New(
			workers.CreatePool("BookingOrder", 2),
			b.Block,
			(*booker.Block).IsBooked,
			b.book,
			b.markInvalid,
			(*booker.Block).Parents,
			causalorder.WithReferenceValidator[models.BlockID](isReferenceValid),
		)

		b.evictionState.Events.SlotEvicted.Hook(b.evict)

		b.events.VirtualVoting.LinkTo(b.virtualVoting.Events())
		b.events.SequenceEvicted.LinkTo(b.markerManager.Events.SequenceEvicted)
		b.events.SequenceTracker.LinkTo(b.sequenceTracker.Events)
		b.events.SlotTracker.LinkTo(b.slotTracker.Events)
	}, (*Booker).TriggerConstructed)
}

func (b *Booker) Initialize(blockDAG blockdag.BlockDAG) {
	b.blockDAG = blockDAG

	b.blockDAG.Events().BlockSolid.Hook(func(block *blockdag.Block) {
		if _, err := b.Queue(booker.NewBlock(block)); err != nil {
			panic(err)
		}
	})
	b.blockDAG.Events().BlockOrphaned.Hook(func(orphanedBlock *blockdag.Block) {
		block, exists := b.Block(orphanedBlock.ID())
		if !exists {
			return
		}

		b.OrphanAttachment(block)
	})
	b.MemPool.Events().TransactionConflictIDUpdated.Hook(func(event *mempool.TransactionConflictIDUpdatedEvent) {
		if err := b.PropagateForkedConflict(event.TransactionID, event.AddedConflictID, event.RemovedConflictIDs); err != nil {
			b.events.Error.Trigger(errors.Wrapf(err, "failed to propagate Conflict update of %s to BlockDAG", event.TransactionID))
		}
	})
	b.MemPool.Events().TransactionBooked.Hook(func(e *mempool.TransactionBookedEvent) {
		contextBlockID := models.BlockIDFromContext(e.Context)

		for _, block := range b.attachments.Get(e.TransactionID) {
			if contextBlockID != block.ID() {
				b.bookingOrder.Queue(block)
			}
		}
	}, event.WithWorkerPool(b.workers.CreatePool("Booker", 2)))

	b.events.SequenceEvicted.Hook(func(sequenceID markers.SequenceID) {
		b.EvictSequence(sequenceID)
	}, event.WithWorkerPool(b.workers.CreatePool("VirtualVoting Sequence Eviction", 1)))

	b.TriggerInitialized()
}

var _ booker.Booker = new(Booker)

func (b *Booker) Events() *booker.Events {
	return b.events
}

func (b *Booker) VirtualVoting() booker.VirtualVoting {
	return b.virtualVoting
}

func (b *Booker) SequenceTracker() *sequencetracker.SequenceTracker[booker.BlockVotePower] {
	return b.sequenceTracker
}

func (b *Booker) SequenceManager() *markers.SequenceManager {
	return b.markerManager.SequenceManager
}

// Queue checks if payload is solid and then adds the block to a Booker's CausalOrder.
func (b *Booker) Queue(block *booker.Block) (wasQueued bool, err error) {
	if wasQueued, err = b.queue(block); wasQueued {
		b.bookingOrder.Queue(block)
	}

	return
}

func (b *Booker) queue(block *booker.Block) (wasQueued bool, err error) {
	b.evictionMutex.RLock()
	defer b.evictionMutex.RUnlock()

	if b.evictionState.InEvictedSlot(block.ID()) {
		return false, nil
	}

	b.blocks.Get(block.ID().Index(), true).Set(block.ID(), block)

	return b.isPayloadSolid(block)
}

// Block retrieves a Block with metadata from the in-memory storage of the Booker.
func (b *Booker) Block(id models.BlockID) (block *booker.Block, exists bool) {
	b.evictionMutex.RLock()
	defer b.evictionMutex.RUnlock()

	return b.block(id)
}

// BlockConflicts returns the Conflict related details of the given Block.
func (b *Booker) BlockConflicts(block *booker.Block) (blockConflictIDs utxo.TransactionIDs) {
	_, blockConflictIDs = b.BlockBookingDetails(block)
	return
}

// BlockBookingDetails returns the Conflict and Marker related details of the given Block.
func (b *Booker) BlockBookingDetails(block *booker.Block) (pastMarkersConflictIDs, blockConflictIDs utxo.TransactionIDs) {
	b.evictionMutex.RLock()
	defer b.evictionMutex.RUnlock()

	return b.blockBookingDetails(block)
}

// TransactionConflictIDs returns the ConflictIDs of the Transaction contained in the given Block including conflicts from the UTXO past cone.
func (b *Booker) TransactionConflictIDs(block *booker.Block) (conflictIDs utxo.TransactionIDs) {
	if b.evictionState.InEvictedSlot(block.ID()) {
		return utxo.NewTransactionIDs()
	}

	conflictIDs = utxo.NewTransactionIDs()

	transaction, isTransaction := block.Transaction()
	if !isTransaction {
		return
	}

	b.MemPool.Storage().CachedTransactionMetadata(transaction.ID()).Consume(func(transactionMetadata *mempool.TransactionMetadata) {
		conflictIDs.AddAll(transactionMetadata.ConflictIDs())
	})

	return
}

// PayloadConflictID returns the ConflictID of the conflicting payload contained in the given Block without conflicts from the UTXO past cone.
func (b *Booker) PayloadConflictID(block *booker.Block) (conflictID utxo.TransactionID, conflictingConflictIDs utxo.TransactionIDs, isTransaction bool) {
	conflictingConflictIDs = utxo.NewTransactionIDs()

	if b.evictionState.InEvictedSlot(block.ID()) {
		return conflictID, conflictingConflictIDs, false
	}

	transaction, isTransaction := block.Transaction()
	if !isTransaction {
		return conflictID, conflictingConflictIDs, false
	}

	conflict, exists := b.MemPool.ConflictDAG().Conflict(transaction.ID())
	if !exists {
		return utxo.EmptyTransactionID, conflictingConflictIDs, true
	}

	conflict.ForEachConflictingConflict(func(conflictingConflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) bool {
		conflictingConflictIDs.Add(conflictingConflict.ID())
		return true
	})

	return transaction.ID(), conflictingConflictIDs, true
}

// Sequence retrieves a Sequence by its ID.
func (b *Booker) Sequence(id markers.SequenceID) (sequence *markers.Sequence, exists bool) {
	b.evictionMutex.RLock()
	defer b.evictionMutex.RUnlock()

	return b.markerManager.SequenceManager.Sequence(id)
}

// BlockFromMarker retrieves the Block of the given Marker.
func (b *Booker) BlockFromMarker(marker markers.Marker) (block *booker.Block, exists bool) {
	b.evictionMutex.RLock()
	defer b.evictionMutex.RUnlock()
	if marker.Index() == 0 {
		panic(fmt.Sprintf("cannot retrieve block for Marker with Index(0) - %#v", marker))
	}

	return b.markerManager.BlockFromMarker(marker)
}

// BlockCeiling returns the smallest Index that is >= the given Marker and a boolean value indicating if it exists.
func (b *Booker) BlockCeiling(marker markers.Marker) (ceilingMarker markers.Marker, exists bool) {
	b.evictionMutex.RLock()
	defer b.evictionMutex.RUnlock()

	return b.markerManager.BlockCeiling(marker)
}

// BlockFloor returns the largest Index that is <= the given Marker and a boolean value indicating if it exists.
func (b *Booker) BlockFloor(marker markers.Marker) (floorMarker markers.Marker, exists bool) {
	b.evictionMutex.RLock()
	defer b.evictionMutex.RUnlock()

	return b.markerManager.BlockFloor(marker)
}

// MarkerVotersTotalWeight retrieves Validators supporting a given marker.
func (b *Booker) MarkerVotersTotalWeight(marker markers.Marker) (totalWeight int64) {
	b.sequenceEvictionMutex.RLock()
	defer b.sequenceEvictionMutex.RUnlock()

	_ = b.sequenceTracker.Voters(marker).ForEach(func(id identity.ID) error {
		if weight, exists := b.validators.Get(id); exists {
			totalWeight += weight.Value
		}

		return nil
	})

	return totalWeight
}

// SlotVotersTotalWeight retrieves the total weight of the Validators voting for a given slot.
func (b *Booker) SlotVotersTotalWeight(slotIndex slot.Index) (totalWeight int64) {
	b.sequenceEvictionMutex.RLock()
	defer b.sequenceEvictionMutex.RUnlock()

	_ = b.slotTracker.Voters(slotIndex).ForEach(func(id identity.ID) error {
		if weight, exists := b.validators.Get(id); exists {
			totalWeight += weight.Value
		}

		return nil
	})

	return totalWeight
}

// GetEarliestAttachment returns the earliest attachment for a given transaction ID.
// returnOrphaned parameter specifies whether the returned attachment may be orphaned.
func (b *Booker) GetEarliestAttachment(txID utxo.TransactionID) (attachment *booker.Block) {
	return b.attachments.getEarliestAttachment(txID)
}

// GetLatestAttachment returns the latest attachment for a given transaction ID.
// returnOrphaned parameter specifies whether the returned attachment may be orphaned.
func (b *Booker) GetLatestAttachment(txID utxo.TransactionID) (attachment *booker.Block) {
	return b.attachments.getLatestAttachment(txID)
}

func (b *Booker) GetAllAttachments(txID utxo.TransactionID) (attachments *advancedset.AdvancedSet[*booker.Block]) {
	return b.attachments.GetAttachmentBlocks(txID)
}

func (b *Booker) ProcessForkedMarker(marker markers.Marker, forkedConflictID utxo.TransactionID, parentConflictIDs utxo.TransactionIDs) {
	b.sequenceEvictionMutex.RLock()
	defer b.sequenceEvictionMutex.RUnlock()

	// take everything in future cone because it was not conflicting before and move to new conflict.
	for voterID, votePower := range b.sequenceTracker.VotersWithPower(marker) {
		b.virtualVoting.ConflictTracker().AddSupportToForkedConflict(forkedConflictID, parentConflictIDs, voterID, votePower)
	}
}

func (b *Booker) EvictSequence(sequenceID markers.SequenceID) {
	b.evictionMutex.Lock()
	defer b.evictionMutex.Unlock()

	b.sequenceTracker.EvictSequence(sequenceID)
}

func (b *Booker) EvictSlotTracker(slotIndex slot.Index) {
	b.evictionMutex.Lock()
	defer b.evictionMutex.Unlock()

	b.slotTracker.EvictSlot(slotIndex)
}

func (b *Booker) evict(slotIndex slot.Index) {
	b.bookingOrder.EvictUntil(slotIndex)

	b.evictionMutex.Lock()
	defer b.evictionMutex.Unlock()

	b.attachments.Evict(slotIndex)
	b.markerManager.Evict(slotIndex)
	b.blocks.Evict(slotIndex)
}

func (b *Booker) isPayloadSolid(block *booker.Block) (isPayloadSolid bool, err error) {
	tx, isTx := block.Transaction()
	if !isTx {
		return true, nil
	}

	if b.attachments.Store(tx.ID(), block) {
		b.events.AttachmentCreated.Trigger(block)
	}

	if err = b.MemPool.StoreAndProcessTransaction(
		models.BlockIDToContext(context.Background(), block.ID()), tx,
	); errors.Is(err, mempool.ErrTransactionUnsolid) {
		return false, nil
	}

	return err == nil, err
}

// block retrieves the Block with given id from the mem-storage.
func (b *Booker) block(id models.BlockID) (block *booker.Block, exists bool) {
	if b.evictionState.IsRootBlock(id) {
		return booker.NewRootBlock(id, b.slotTimeProviderFunc()), true
	}

	storage := b.blocks.Get(id.Index(), false)
	if storage == nil {
		return nil, false
	}

	return storage.Get(id)
}

func (b *Booker) book(block *booker.Block) (inheritingErr error) {
	// Need to mutually exclude a fork on this block.
	// VirtualVoting.Track is performed within the context on this lock to make those two steps atomic.
	b.bookingMutex.Lock(block.ID())
	defer b.bookingMutex.Unlock(block.ID())

	// TODO: make sure this is actually necessary
	if block.IsInvalid() {
		return errors.Errorf("block with %s was marked as invalid", block.ID())
	}

	tryInheritConflictIDs := func() (inheritedConflictIDs utxo.TransactionIDs, err error) {
		b.evictionMutex.RLock()
		defer b.evictionMutex.RUnlock()

		if b.evictionState.InEvictedSlot(block.ID()) {
			return nil, errors.Errorf("block with %s belongs to an evicted slot", block.ID())
		}

		if inheritedConflictIDs, err = b.inheritConflictIDs(block); err != nil {
			return nil, errors.Wrap(err, "error inheriting conflict IDs")
		}

		return
	}

	inheritedConflitIDs, inheritingErr := tryInheritConflictIDs()
	if inheritingErr != nil {
		return inheritingErr
	}

	b.events.BlockBooked.Trigger(&booker.BlockBookedEvent{
		Block:       block,
		ConflictIDs: inheritedConflitIDs,
	})

	votePower := booker.NewBlockVotePower(block.ID(), block.IssuingTime())
	if invalid := b.virtualVoting.Track(block, inheritedConflitIDs, votePower); !invalid {
		b.sequenceTracker.TrackVotes(block.StructureDetails().PastMarkers(), block.IssuerID(), votePower)
		b.slotTracker.TrackVotes(block.Commitment().Index(), block.IssuerID(), slottracker.SlotVotePower{Index: block.ID().Index()})
	}

	b.events.BlockTracked.Trigger(block)

	return nil
}

func (b *Booker) markInvalid(block *booker.Block, reason error) {
	b.blockDAG.SetInvalid(block.Block, errors.Wrap(reason, "block marked as invalid in Booker"))
}

func (b *Booker) inheritConflictIDs(block *booker.Block) (inheritedConflictIDs utxo.TransactionIDs, err error) {
	b.bookingMutex.RLock(block.Parents()...)
	defer b.bookingMutex.RUnlock(block.Parents()...)

	allParentsInPastSlots := true
	for parentID := range block.ParentsByType(models.StrongParentType) {
		if parentID.Index() >= block.ID().Index() {
			allParentsInPastSlots = false
			break
		}
	}

	strongParentsStructureDetails := b.determineStrongParentsStructureDetails(block)
	newStructureDetails, newSequenceCreated := b.markerManager.SequenceManager.InheritStructureDetails(strongParentsStructureDetails, allParentsInPastSlots)

	pastMarkersConflictIDs, err := func() (pastMarkersConflictIDs *advancedset.AdvancedSet[utxo.TransactionID], err error) {
		// Prevent race-condition by write-locking the sequence we are writing conflicts mapping to and,
		// read-locking sequences we are sourcing such mappings from.
		if newStructureDetails.IsPastMarker() {
			b.sequenceMutex.Lock(newStructureDetails.PastMarkers().Marker().SequenceID())
			defer b.sequenceMutex.Unlock(newStructureDetails.PastMarkers().Marker().SequenceID())
		} else {
			sequenceIDs := make([]markers.SequenceID, 0)
			newStructureDetails.PastMarkers().ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
				sequenceIDs = append(sequenceIDs, sequenceID)
				return true
			})
			b.sequenceMutex.RLock(sequenceIDs...)
			defer b.sequenceMutex.RUnlock(sequenceIDs...)
		}

		pastMarkersConflictIDs, inheritedConflictIDs, err = b.determineBookingConflictIDs(block)
		if err != nil {
			return nil, errors.Wrap(err, "failed to determine booking conflict IDs")
		}
		b.markerManager.ProcessBlock(block, newSequenceCreated, inheritedConflictIDs, newStructureDetails)

		return pastMarkersConflictIDs, nil
	}()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to inherit conflict IDs of block with %s", block.ID())
	}

	block.SetStructureDetails(newStructureDetails)

	if !newStructureDetails.IsPastMarker() {
		newStructureDetailsConflictIDs := b.markerManager.ConflictIDsFromStructureDetails(newStructureDetails)

		addedConflictIDs := inheritedConflictIDs.Clone()
		addedConflictIDs.DeleteAll(newStructureDetailsConflictIDs)
		block.AddAllAddedConflictIDs(addedConflictIDs)

		subtractedConflictIDs := pastMarkersConflictIDs.Clone()
		subtractedConflictIDs.DeleteAll(inheritedConflictIDs)
		block.AddAllSubtractedConflictIDs(subtractedConflictIDs)
	}

	block.SetBooked()

	return
}

// determineBookingDetails determines the booking details of an unbooked Block.
func (b *Booker) determineBookingConflictIDs(block *booker.Block) (parentsPastMarkersConflictIDs, inheritedConflictIDs utxo.TransactionIDs, err error) {
	inheritedConflictIDs = b.TransactionConflictIDs(block)

	parentsPastMarkersConflictIDs, strongParentsConflictIDs := b.collectStrongParentsConflictIDs(block)

	weakPayloadConflictIDs := b.collectWeakParentsConflictIDs(block)

	likedConflictIDs, dislikedConflictIDs, shallowLikeErr := b.collectShallowLikedParentsConflictIDs(block)
	if shallowLikeErr != nil {
		return nil, nil, errors.Wrapf(shallowLikeErr, "failed to collect shallow likes of %s", block.ID())
	}

	inheritedConflictIDs.AddAll(strongParentsConflictIDs)
	inheritedConflictIDs.AddAll(weakPayloadConflictIDs)
	inheritedConflictIDs.AddAll(likedConflictIDs)

	inheritedConflictIDs.DeleteAll(b.MemPool.Utils().ConflictIDsInFutureCone(dislikedConflictIDs))

	// block always sets Like reference its own conflict, if its payload is a transaction, and it's conflicting
	if selfConflictID, selfDislikedConflictIDs, isTransaction := b.PayloadConflictID(block); isTransaction && !selfConflictID.IsEmpty() {
		inheritedConflictIDs.Add(selfConflictID)
		// if a payload is a conflicting transaction, then remove any conflicting conflicts from supported conflicts
		inheritedConflictIDs.DeleteAll(b.MemPool.Utils().ConflictIDsInFutureCone(selfDislikedConflictIDs))
	}

	unconfirmedParentsPast := b.MemPool.ConflictDAG().UnconfirmedConflicts(parentsPastMarkersConflictIDs)
	unconfirmedInherited := b.MemPool.ConflictDAG().UnconfirmedConflicts(inheritedConflictIDs)

	return unconfirmedParentsPast, unconfirmedInherited, nil
}

func (b *Booker) determineStrongParentsStructureDetails(block *booker.Block) (strongParentsStructureDetails []*markers.StructureDetails) {
	strongParentsStructureDetails = make([]*markers.StructureDetails, 0)

	block.ForEachParentByType(models.StrongParentType, func(parentBlockID models.BlockID) bool {
		if b.evictionState.IsRootBlock(parentBlockID) {
			return true
		}

		parentBlock, exists := b.block(parentBlockID)
		if !exists {
			// This should never happen.
			panic(fmt.Sprintf("parent %s does not exist", parentBlockID))
		}

		strongParentsStructureDetails = append(strongParentsStructureDetails, parentBlock.StructureDetails())

		return true
	})

	return
}

// collectStrongParentsBookingDetails returns the booking details of a Block's strong parents.
func (b *Booker) collectStrongParentsConflictIDs(block *booker.Block) (parentsPastMarkersConflictIDs, parentsConflictIDs utxo.TransactionIDs) {
	parentsPastMarkersConflictIDs = utxo.NewTransactionIDs()
	parentsConflictIDs = utxo.NewTransactionIDs()

	block.ForEachParentByType(models.StrongParentType, func(parentBlockID models.BlockID) bool {
		if b.evictionState.IsRootBlock(parentBlockID) {
			return true
		}

		parentBlock, exists := b.block(parentBlockID)
		if !exists {
			// This should never happen.
			panic(fmt.Sprintf("parent %s does not exist", parentBlockID))
		}

		parentPastMarkersConflictIDs, parentConflictIDs := b.blockBookingDetails(parentBlock)
		parentsPastMarkersConflictIDs.AddAll(parentPastMarkersConflictIDs)
		parentsConflictIDs.AddAll(parentConflictIDs)

		return true
	})

	return
}

// collectShallowDislikedParentsConflictIDs removes the ConflictIDs of the shallow dislike reference and all its conflicts from
// the supplied ArithmeticConflictIDs.
func (b *Booker) collectWeakParentsConflictIDs(block *booker.Block) (transactionConflictIDs utxo.TransactionIDs) {
	transactionConflictIDs = utxo.NewTransactionIDs()

	block.ForEachParentByType(models.WeakParentType, func(parentBlockID models.BlockID) bool {
		parentBlock, exists := b.Block(parentBlockID)
		if !exists {
			panic(fmt.Sprintf("parent %s does not exist", parentBlockID))
		}
		transactionConflictIDs.AddAll(b.TransactionConflictIDs(parentBlock))

		return true
	})

	return transactionConflictIDs
}

// collectShallowLikedParentsConflictIDs adds the ConflictIDs of the shallow like reference and removes all its conflicts from
// the supplied ArithmeticConflictIDs.
func (b *Booker) collectShallowLikedParentsConflictIDs(block *booker.Block) (collectedLikedConflictIDs, collectedDislikedConflictIDs utxo.TransactionIDs, err error) {
	collectedLikedConflictIDs = utxo.NewTransactionIDs()
	collectedDislikedConflictIDs = utxo.NewTransactionIDs()

	block.ForEachParentByType(models.ShallowLikeParentType, func(parentBlockID models.BlockID) bool {
		parentBlock, exists := b.Block(parentBlockID)
		if !exists {
			panic(fmt.Sprintf("parent %s does not exist", parentBlockID))
		}

		conflictID, conflictingConflictIDs, isTransaction := b.PayloadConflictID(parentBlock)
		if !isTransaction {
			err = errors.Errorf("%s (isRootBlock %t) referenced by a shallow like of %s does not contain a Transaction", parentBlockID, b.evictionState.IsRootBlock(parentBlockID), block.ID())
			return false
		}

		// if Payload is a transaction but is not conflicting (yet, possibly) do not discard the whole block, but ignore the Like reference
		// if the Payload will be forked in the future, then forking logic will use that Like reference during propagation
		if conflictID.IsEmpty() {
			return true
		}

		collectedLikedConflictIDs.Add(conflictID)
		collectedDislikedConflictIDs.AddAll(conflictingConflictIDs)

		return err == nil
	})

	return collectedLikedConflictIDs, collectedDislikedConflictIDs, err
}

// blockBookingDetails returns the Conflict and Marker related details of the given Block.
func (b *Booker) blockBookingDetails(block *booker.Block) (pastMarkersConflictIDs, blockConflictIDs utxo.TransactionIDs) {
	b.rLockBlockSequences(block)
	defer b.rUnlockBlockSequences(block)

	pastMarkersConflictIDs = b.markerManager.ConflictIDsFromStructureDetails(block.StructureDetails())

	blockConflictIDs = utxo.NewTransactionIDs()
	blockConflictIDs.AddAll(pastMarkersConflictIDs)

	if addedConflictIDs := block.AddedConflictIDs(); !addedConflictIDs.IsEmpty() {
		blockConflictIDs.AddAll(addedConflictIDs)
	}

	// We always need to subtract all conflicts in the future cone of the SubtractedConflictIDs due to the fact that
	// conflicts in the future cone can be propagated later. Specifically, through changing a marker mapping, the base
	// of the block's conflicts changes, and thus it might implicitly "inherit" conflicts that were previously removed.
	if subtractedConflictIDs := b.MemPool.Utils().ConflictIDsInFutureCone(block.SubtractedConflictIDs()); !subtractedConflictIDs.IsEmpty() {
		blockConflictIDs.DeleteAll(subtractedConflictIDs)
	}

	return pastMarkersConflictIDs, blockConflictIDs
}

func (b *Booker) blocksFromBlockDAGBlocks(blocks []*blockdag.Block) []*booker.Block {
	return lo.Filter(lo.Map(blocks, func(blockDAGChild *blockdag.Block) (bookerChild *booker.Block) {
		bookerChild, exists := b.block(blockDAGChild.ID())
		if !exists {
			return nil
		}
		return bookerChild
	}), func(child *booker.Block) bool {
		return child != nil
	})
}

func (b *Booker) OrphanAttachment(block *booker.Block) {
	if tx, isTx := block.Transaction(); isTx {
		attachmentBlock, attachmentOrphaned, lastAttachmentOrphaned := b.attachments.AttachmentOrphaned(tx.ID(), block)

		if attachmentOrphaned {
			b.events.AttachmentOrphaned.Trigger(attachmentBlock)
		}

		if lastAttachmentOrphaned {
			// TODO: attach this event somewhere to the engine
			b.events.Error.Trigger(errors.Errorf("transaction %s orphaned", tx.ID()))
			b.MemPool.PruneTransaction(tx.ID(), true)
		}
	}
}

// region FORK LOGIC ///////////////////////////////////////////////////////////////////////////////////////////////////

// PropagateForkedConflict propagates the forked ConflictID to the future cone of the attachments of the given Transaction.
func (b *Booker) PropagateForkedConflict(transactionID, addedConflictID utxo.TransactionID, removedConflictIDs utxo.TransactionIDs) (err error) {
	blockWalker := walker.New[*booker.Block]()

	for it := b.GetAllAttachments(transactionID).Iterator(); it.HasNext(); {
		attachment := it.Next()
		blockWalker.Push(attachment)

		// weak and like reference implies a vote only on the parent, so fork propagation should only go through direct weak/like children.
		blockWalker.PushAll(b.blocksFromBlockDAGBlocks(attachment.WeakChildren())...)
		blockWalker.PushAll(b.blocksFromBlockDAGBlocks(attachment.LikedInsteadChildren())...)
	}

	for blockWalker.HasNext() {
		block := blockWalker.Next()
		propagateFurther, propagationErr := b.propagateToBlock(block, addedConflictID, removedConflictIDs)
		if propagationErr != nil {
			blockWalker.StopWalk()
			return errors.Wrapf(propagationErr, "failed to propagate forked ConflictID %s to future cone of %s", addedConflictID, block.ID())
		}
		if propagateFurther {
			blockWalker.PushAll(b.blocksFromBlockDAGBlocks(block.StrongChildren())...)
		}
	}

	return nil
}

func (b *Booker) propagateToBlock(block *booker.Block, addedConflictID utxo.TransactionID, removedConflictIDs utxo.TransactionIDs) (propagateFurther bool, err error) {
	// Need to mutually exclude a booking on this block.
	// VirtualVoting.Track is performed within the context on this lock to make those two steps atomic.
	// TODO: possibly need to also lock this mutex when propagating through markers.
	b.bookingMutex.Lock(block.ID())
	defer b.bookingMutex.Unlock(block.ID())

	updated, propagateFurther, forkErr := b.propagateForkedConflict(block, addedConflictID, removedConflictIDs)
	if forkErr != nil {
		return false, errors.Wrapf(forkErr, "failed to propagate forked ConflictID %s to future cone of %s", addedConflictID, block.ID())
	}
	if !updated {
		return false, nil
	}

	b.events.BlockConflictAdded.Trigger(&booker.BlockConflictAddedEvent{
		Block:             block,
		ConflictID:        addedConflictID,
		ParentConflictIDs: removedConflictIDs,
	})

	b.virtualVoting.ProcessForkedBlock(block, addedConflictID, removedConflictIDs)

	return true, nil
}

func (b *Booker) propagateForkedConflict(block *booker.Block, addedConflictID utxo.TransactionID, removedConflictIDs utxo.TransactionIDs) (propagated, propagateFurther bool, err error) {
	if !block.IsBooked() {
		return false, false, nil
	}

	// if structureDetails := block.StructureDetails(); structureDetails.IsPastMarker() {
	// 	fmt.Println(">> propagating forked conflict to marker future cone of block", addedConflictID, block.ID(), structureDetails.PastMarkers().Marker())
	// 	if err = b.propagateForkedTransactionToMarkerFutureCone(structureDetails.PastMarkers().Marker(), addedConflictID, removedConflictIDs); err != nil {
	// 		err = errors.Wrapf(err, "failed to propagate conflict %s to future cone of %v", addedConflictID, structureDetails.PastMarkers().Marker())
	// 		fmt.Println(err)
	// 		return false, false, err
	// 	}
	// 	return true, false, nil
	// }

	propagated = b.updateBlockConflicts(block, addedConflictID, removedConflictIDs)
	// We only need to propagate further (in the block's future cone) if the block was updated.
	return propagated, propagated, nil
}

func (b *Booker) updateBlockConflicts(block *booker.Block, addedConflict utxo.TransactionID, parentConflicts utxo.TransactionIDs) (updated bool) {
	_, conflictIDs := b.blockBookingDetails(block)

	// if a block does not already support all parent conflicts of a conflict A, then it cannot vote for a more specialize conflict of A
	if !conflictIDs.HasAll(parentConflicts) {
		return false
	}

	updated = block.AddConflictID(addedConflict)

	return updated
}

// propagateForkedTransactionToMarkerFutureCone propagates a newly created ConflictID into the future cone of the given Marker.
func (b *Booker) propagateForkedTransactionToMarkerFutureCone(marker markers.Marker, conflictID utxo.TransactionID, removedConflictIDs utxo.TransactionIDs) (err error) {
	markerWalker := walker.New[markers.Marker](false)
	markerWalker.Push(marker)

	for markerWalker.HasNext() {
		currentMarker := markerWalker.Next()

		if err = b.forkSingleMarker(currentMarker, conflictID, removedConflictIDs, markerWalker); err != nil {
			return errors.Wrapf(err, "failed to propagate Conflict %s to Blocks approving %v", conflictID, currentMarker)
		}
	}

	return
}

// forkSingleMarker propagates a newly created ConflictID to a single marker and queues the next elements that need to be
// visited.
func (b *Booker) forkSingleMarker(currentMarker markers.Marker, newConflictID utxo.TransactionID, removedConflictIDs utxo.TransactionIDs, markerWalker *walker.Walker[markers.Marker]) (err error) {
	b.markerManager.SequenceMutex.Lock(currentMarker.SequenceID())
	defer b.markerManager.SequenceMutex.Unlock(currentMarker.SequenceID())

	// update ConflictID mapping
	newConflictIDs := b.markerManager.ConflictIDs(currentMarker)
	if !newConflictIDs.HasAll(removedConflictIDs) {
		return nil
	}

	if !newConflictIDs.Add(newConflictID) {
		return nil
	}

	if !b.markerManager.SetConflictIDs(currentMarker, newConflictIDs) {
		return nil
	}

	// trigger event
	b.events.MarkerConflictAdded.Trigger(&booker.MarkerConflictAddedEvent{
		Marker:            currentMarker,
		ConflictID:        newConflictID,
		ParentConflictIDs: removedConflictIDs,
	})

	b.ProcessForkedMarker(currentMarker, newConflictID, removedConflictIDs)

	// propagate updates to later ConflictID mappings of the same sequence.
	b.markerManager.ForEachConflictIDMapping(currentMarker.SequenceID(), currentMarker.Index(), func(mappedMarker markers.Marker, _ utxo.TransactionIDs) {
		markerWalker.Push(mappedMarker)
	})

	// propagate updates to referencing markers of later sequences ...
	b.markerManager.ForEachMarkerReferencingMarker(currentMarker, func(referencingMarker markers.Marker) {
		markerWalker.Push(referencingMarker)
	})

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Utils //////////////////////////////////////////////////////////////////////////////////////////////////////

func (b *Booker) rLockBlockSequences(block *booker.Block) bool {
	return block.StructureDetails().PastMarkers().ForEachSorted(func(sequenceID markers.SequenceID, _ markers.Index) bool {
		b.markerManager.SequenceMutex.RLock(sequenceID)
		return true
	})
}

func (b *Booker) rUnlockBlockSequences(block *booker.Block) bool {
	return block.StructureDetails().PastMarkers().ForEachSorted(func(sequenceID markers.SequenceID, _ markers.Index) bool {
		b.markerManager.SequenceMutex.RUnlock(sequenceID)
		return true
	})
}

// isReferenceValid checks if the reference between the child and its parent is valid.
func isReferenceValid(child *booker.Block, parent *booker.Block) (err error) {
	if parent.IsInvalid() {
		return errors.Errorf("parent %s of child %s is marked as invalid", parent.ID(), child.ID())
	}

	return nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithMarkerManagerOptions(opts ...options.Option[markermanager.MarkerManager[models.BlockID, *booker.Block]]) options.Option[Booker] {
	return func(b *Booker) {
		b.optsMarkerManager = append(b.optsMarkerManager, opts...)
	}
}

func WithSlotCutoffCallback(slotCutoffCallback func() slot.Index) options.Option[Booker] {
	return func(b *Booker) {
		b.optsSlotCutoffCallback = slotCutoffCallback
	}
}

func WithSequenceCutoffCallback(sequenceCutoffCallback func(id markers.SequenceID) markers.Index) options.Option[Booker] {
	return func(b *Booker) {
		b.optsSequenceCutoffCallback = sequenceCutoffCallback
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
