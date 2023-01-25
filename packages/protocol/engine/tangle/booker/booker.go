package booker

import (
	"context"
	"fmt"
	"sync"

	"github.com/iotaledger/hive.go/core/cerrors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/walker"
	"github.com/iotaledger/hive.go/core/syncutils"
	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/causalorder"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markermanager"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// region Booker ///////////////////////////////////////////////////////////////////////////////////////////////////////

type Booker struct {
	// Events contains the Events of Booker.
	Events *Events

	Ledger        *ledger.Ledger
	bookingOrder  *causalorder.CausalOrder[models.BlockID, *Block]
	attachments   *attachments
	blocks        *memstorage.EpochStorage[models.BlockID, *Block]
	markerManager *markermanager.MarkerManager[models.BlockID, *Block]
	bookingMutex  *syncutils.DAGMutex[models.BlockID]
	evictionMutex sync.RWMutex

	optsMarkerManager []options.Option[markermanager.MarkerManager[models.BlockID, *Block]]

	*blockdag.BlockDAG
}

func New(blockDAG *blockdag.BlockDAG, ledger *ledger.Ledger, opts ...options.Option[Booker]) (booker *Booker) {
	return options.Apply(&Booker{
		Events:            NewEvents(),
		attachments:       newAttachments(),
		blocks:            memstorage.NewEpochStorage[models.BlockID, *Block](),
		bookingMutex:      syncutils.NewDAGMutex[models.BlockID](),
		optsMarkerManager: make([]options.Option[markermanager.MarkerManager[models.BlockID, *Block]], 0),
		Ledger:            ledger,
		BlockDAG:          blockDAG,
	}, opts, func(b *Booker) {
		b.markerManager = markermanager.NewMarkerManager(b.optsMarkerManager...)
		b.bookingOrder = causalorder.New(
			b.Block,
			(*Block).IsBooked,
			b.book,
			b.markInvalid,
			causalorder.WithReferenceValidator[models.BlockID](isReferenceValid),
		)

		blockDAG.EvictionState.Events.EpochEvicted.Hook(event.NewClosure(b.evict))

		b.Events.MarkerManager = b.markerManager.Events
	}, (*Booker).setupEvents)
}

// Queue checks if payload is solid and then adds the block to a Booker's CausalOrder.
func (b *Booker) Queue(block *Block) (wasQueued bool, err error) {
	if wasQueued, err = b.queue(block); wasQueued {
		b.bookingOrder.Queue(block)
	}

	return
}

func (b *Booker) queue(block *Block) (wasQueued bool, err error) {
	b.evictionMutex.RLock()
	defer b.evictionMutex.RUnlock()

	if b.EvictionState.InEvictedEpoch(block.ID()) {
		return false, nil
	}

	b.blocks.Get(block.ID().Index(), true).Set(block.ID(), block)

	return b.isPayloadSolid(block)
}

// Block retrieves a Block with metadata from the in-memory storage of the Booker.
func (b *Booker) Block(id models.BlockID) (block *Block, exists bool) {
	b.evictionMutex.RLock()
	defer b.evictionMutex.RUnlock()

	return b.block(id)
}

// BlockConflicts returns the Conflict related details of the given Block.
func (b *Booker) BlockConflicts(block *Block) (blockConflictIDs utxo.TransactionIDs) {
	_, blockConflictIDs = b.BlockBookingDetails(block)
	return
}

// BlockBookingDetails returns the Conflict and Marker related details of the given Block.
func (b *Booker) BlockBookingDetails(block *Block) (pastMarkersConflictIDs, blockConflictIDs utxo.TransactionIDs) {
	b.evictionMutex.RLock()
	defer b.evictionMutex.RUnlock()

	return b.blockBookingDetails(block)
}

// PayloadConflictIDs returns the ConflictIDs of the payload contained in the given Block.
func (b *Booker) PayloadConflictIDs(block *Block) (conflictIDs utxo.TransactionIDs) {
	if b.EvictionState.InEvictedEpoch(block.ID()) {
		return utxo.NewTransactionIDs()
	}

	conflictIDs = utxo.NewTransactionIDs()

	transaction, isTransaction := block.Transaction()
	if !isTransaction {
		return
	}

	b.Ledger.Storage.CachedTransactionMetadata(transaction.ID()).Consume(func(transactionMetadata *ledger.TransactionMetadata) {
		conflictIDs.AddAll(transactionMetadata.ConflictIDs())
	})

	return
}

// Sequence retrieves a Sequence by its ID.
func (b *Booker) Sequence(id markers.SequenceID) (sequence *markers.Sequence, exists bool) {
	b.evictionMutex.RLock()
	defer b.evictionMutex.RUnlock()

	return b.markerManager.SequenceManager.Sequence(id)
}

// BlockFromMarker retrieves the Block of the given Marker.
func (b *Booker) BlockFromMarker(marker markers.Marker) (block *Block, exists bool) {
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

// GetEarliestAttachment returns the earliest attachment for a given transaction ID.
func (b *Booker) GetEarliestAttachment(txID utxo.TransactionID) (attachment *Block) {
	return b.attachments.getEarliestAttachment(txID)
}

// GetLatestAttachment returns the latest attachment for a given transaction ID.
func (b *Booker) GetLatestAttachment(txID utxo.TransactionID) (attachment *Block) {
	return b.attachments.getLatestAttachment(txID)
}

func (b *Booker) GetAllAttachments(txID utxo.TransactionID) (attachments Blocks) {
	return NewBlocks(b.attachments.Get(txID)...)
}

func (b *Booker) evict(epochIndex epoch.Index) {
	b.bookingOrder.EvictUntil(epochIndex)

	b.evictionMutex.Lock()
	defer b.evictionMutex.Unlock()

	b.attachments.Evict(epochIndex)
	b.markerManager.Evict(epochIndex)
	b.blocks.Evict(epochIndex)
}

func (b *Booker) isPayloadSolid(block *Block) (isPayloadSolid bool, err error) {
	tx, isTx := block.Transaction()
	if !isTx {
		return true, nil
	}

	if attachmentBlock, created := b.attachments.Store(tx.ID(), block); created {
		b.Events.AttachmentCreated.Trigger(attachmentBlock)
	}

	if err = b.Ledger.StoreAndProcessTransaction(
		models.BlockIDToContext(context.Background(), block.ID()), tx,
	); errors.Is(err, ledger.ErrTransactionUnsolid) {
		return false, nil
	}

	return err == nil, err
}

// block retrieves the Block with given id from the mem-storage.
func (b *Booker) block(id models.BlockID) (block *Block, exists bool) {
	if b.EvictionState.IsRootBlock(id) {
		return NewRootBlock(id), true
	}

	storage := b.blocks.Get(id.Index(), false)
	if storage == nil {
		return nil, false
	}

	return storage.Get(id)
}

func (b *Booker) book(block *Block) (err error) {
	// TODO: make sure this is actually necessary
	if block.IsInvalid() {
		return errors.Errorf("block with %s was marked as invalid", block.ID())
	}

	tryInheritConflictIDs := func() error {
		b.evictionMutex.RLock()
		defer b.evictionMutex.RUnlock()

		if b.EvictionState.InEvictedEpoch(block.ID()) {
			return errors.Errorf("block with %s belongs to an evicted epoch", block.ID())
		}

		if err := b.inheritConflictIDs(block); err != nil {
			return errors.Wrap(err, "error inheriting conflict IDs")
		}
		return nil
	}

	if err := tryInheritConflictIDs(); err != nil {
		return err
	}

	b.Events.BlockBooked.Trigger(block)

	return nil
}

func (b *Booker) markInvalid(block *Block, reason error) {
	b.SetInvalid(block.Block, reason)
}

func (b *Booker) inheritConflictIDs(block *Block) (err error) {
	b.bookingMutex.RLock(block.Parents()...)
	defer b.bookingMutex.RUnlock(block.Parents()...)
	b.bookingMutex.Lock(block.ID())
	defer b.bookingMutex.Unlock(block.ID())

	parentsStructureDetails, pastMarkersConflictIDs, inheritedConflictIDs, err := b.determineBookingDetails(block)
	if err != nil {
		return errors.Wrap(err, "failed to inherit conflict IDs")
	}

	newStructureDetails := b.markerManager.ProcessBlock(block, parentsStructureDetails, inheritedConflictIDs)
	block.setStructureDetails(newStructureDetails)

	if !newStructureDetails.IsPastMarker() {
		addedConflictIDs := inheritedConflictIDs.Clone()
		addedConflictIDs.DeleteAll(pastMarkersConflictIDs)
		block.AddAllAddedConflictIDs(addedConflictIDs)

		subtractedConflictIDs := pastMarkersConflictIDs.Clone()
		subtractedConflictIDs.DeleteAll(inheritedConflictIDs)
		block.AddAllSubtractedConflictIDs(subtractedConflictIDs)
	}

	block.setBooked()

	return
}

// determineBookingDetails determines the booking details of an unbooked Block.
func (b *Booker) determineBookingDetails(block *Block) (parentsStructureDetails []*markers.StructureDetails, parentsPastMarkersConflictIDs, inheritedConflictIDs utxo.TransactionIDs, err error) {
	inheritedConflictIDs = b.PayloadConflictIDs(block)

	parentsStructureDetails, parentsPastMarkersConflictIDs, strongParentsConflictIDs := b.collectStrongParentsBookingDetails(block)

	weakPayloadConflictIDs := b.collectWeakParentsConflictIDs(block)

	likedConflictIDs, dislikedConflictIDs, shallowLikeErr := b.collectShallowLikedParentsConflictIDs(block)
	if shallowLikeErr != nil {
		return nil, nil, nil, errors.Wrapf(shallowLikeErr, "failed to collect shallow likes of %s", block.ID())
	}

	inheritedConflictIDs.AddAll(strongParentsConflictIDs)
	inheritedConflictIDs.AddAll(weakPayloadConflictIDs)
	inheritedConflictIDs.AddAll(likedConflictIDs)
	inheritedConflictIDs.DeleteAll(b.Ledger.Utils.ConflictIDsInFutureCone(dislikedConflictIDs))

	return parentsStructureDetails, b.Ledger.ConflictDAG.UnconfirmedConflicts(parentsPastMarkersConflictIDs), b.Ledger.ConflictDAG.UnconfirmedConflicts(inheritedConflictIDs), nil
}

// collectStrongParentsBookingDetails returns the booking details of a Block's strong parents.
func (b *Booker) collectStrongParentsBookingDetails(block *Block) (parentsStructureDetails []*markers.StructureDetails, parentsPastMarkersConflictIDs, parentsConflictIDs utxo.TransactionIDs) {
	parentsStructureDetails = make([]*markers.StructureDetails, 0)
	parentsPastMarkersConflictIDs = utxo.NewTransactionIDs()
	parentsConflictIDs = utxo.NewTransactionIDs()

	block.ForEachParentByType(models.StrongParentType, func(parentBlockID models.BlockID) bool {
		if b.EvictionState.IsRootBlock(parentBlockID) {
			return true
		}

		parentBlock, exists := b.block(parentBlockID)
		if !exists {
			// This should never happen.
			panic(fmt.Sprintf("parent %s does not exist", parentBlockID))
		}

		parentPastMarkersConflictIDs, parentConflictIDs := b.blockBookingDetails(parentBlock)
		parentsStructureDetails = append(parentsStructureDetails, parentBlock.StructureDetails())
		parentsPastMarkersConflictIDs.AddAll(parentPastMarkersConflictIDs)
		parentsConflictIDs.AddAll(parentConflictIDs)

		return true
	})

	return
}

// collectShallowDislikedParentsConflictIDs removes the ConflictIDs of the shallow dislike reference and all its conflicts from
// the supplied ArithmeticConflictIDs.
func (b *Booker) collectWeakParentsConflictIDs(block *Block) (payloadConflictIDs utxo.TransactionIDs) {
	payloadConflictIDs = utxo.NewTransactionIDs()

	block.ForEachParentByType(models.WeakParentType, func(parentBlockID models.BlockID) bool {
		parentBlock, exists := b.Block(parentBlockID)
		if !exists {
			panic(fmt.Sprintf("parent %s does not exist", parentBlockID))
		}
		payloadConflictIDs.AddAll(b.PayloadConflictIDs(parentBlock))

		return true
	})

	return payloadConflictIDs
}

// collectShallowLikedParentsConflictIDs adds the ConflictIDs of the shallow like reference and removes all its conflicts from
// the supplied ArithmeticConflictIDs.
func (b *Booker) collectShallowLikedParentsConflictIDs(block *Block) (collectedLikedConflictIDs, collectedDislikedConflictIDs utxo.TransactionIDs, err error) {
	collectedLikedConflictIDs = utxo.NewTransactionIDs()
	collectedDislikedConflictIDs = utxo.NewTransactionIDs()
	block.ForEachParentByType(models.ShallowLikeParentType, func(parentBlockID models.BlockID) bool {
		parentBlock, exists := b.Block(parentBlockID)
		if !exists {
			panic(fmt.Sprintf("parent %s does not exist", parentBlockID))
		}
		transaction, isTransaction := parentBlock.Transaction()
		if !isTransaction {
			err = errors.WithMessagef(cerrors.ErrFatal, "%s referenced by a shallow like of %s does not contain a Transaction", parentBlockID, block.ID())
			return false
		}

		collectedLikedConflictIDs.AddAll(b.PayloadConflictIDs(parentBlock))

		for it := b.Ledger.Utils.ConflictingTransactions(transaction.ID()).Iterator(); it.HasNext(); {
			conflictingTransactionID := it.Next()
			dislikedConflicts, dislikedConflictsErr := b.Ledger.Utils.TransactionConflictIDs(conflictingTransactionID)
			if dislikedConflictsErr != nil {
				err = errors.Wrapf(dislikedConflictsErr, "failed to retrieve disliked ConflictIDs of Transaction with %s contained in %s referenced by a shallow like of %s", conflictingTransactionID, parentBlockID, block.ID())
				return false
			}
			collectedDislikedConflictIDs.AddAll(dislikedConflicts)
		}

		return err == nil
	})

	return collectedLikedConflictIDs, collectedDislikedConflictIDs, err
}

// blockBookingDetails returns the Conflict and Marker related details of the given Block.
func (b *Booker) blockBookingDetails(block *Block) (pastMarkersConflictIDs, blockConflictIDs utxo.TransactionIDs) {
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
	if subtractedConflictIDs := b.Ledger.Utils.ConflictIDsInFutureCone(block.SubtractedConflictIDs()); !subtractedConflictIDs.IsEmpty() {
		blockConflictIDs.DeleteAll(subtractedConflictIDs)
	}

	return pastMarkersConflictIDs, blockConflictIDs
}

func (b *Booker) strongChildren(block *Block) []*Block {
	return lo.Filter(lo.Map(block.StrongChildren(), func(blockDAGChild *blockdag.Block) (bookerChild *Block) {
		bookerChild, exists := b.block(blockDAGChild.ID())
		if !exists {
			return nil
		}
		return bookerChild
	}), func(child *Block) bool {
		return child != nil
	})
}

func (b *Booker) setupEvents() {
	b.BlockDAG.Events.BlockSolid.Hook(event.NewClosure(func(block *blockdag.Block) {
		if _, err := b.Queue(NewBlock(block)); err != nil {
			panic(err)
		}
	}))

	b.BlockDAG.Events.BlockOrphaned.Hook(event.NewClosure(func(orphanedBlock *blockdag.Block) {
		block, exists := b.Block(orphanedBlock.ID())
		if !exists {
			return
		}

		if tx, isTx := block.Transaction(); isTx {
			attachmentBlock, attachmentOrphaned, lastAttachmentOrphaned := b.attachments.OrphanAttachment(tx.ID(), block)

			if attachmentOrphaned {
				fmt.Println("Transaction attachments orphaned!!!!", block.ID(), tx.ID())
				b.Events.AttachmentOrphaned.Trigger(attachmentBlock)
			}

			if lastAttachmentOrphaned {
				fmt.Println("Transaction orphaned!!!!", block.ID(), tx.ID())
				b.Events.Error.Trigger(errors.Errorf("transaction %s orphaned", tx.ID()))
				b.Ledger.PruneTransaction(tx.ID(), true)
			}
		}
	}))

	b.Ledger.Events.TransactionConflictIDUpdated.Hook(event.NewClosure(func(event *ledger.TransactionConflictIDUpdatedEvent) {
		if err := b.PropagateForkedConflict(event.TransactionID, event.AddedConflictID, event.RemovedConflictIDs); err != nil {
			b.Events.Error.Trigger(errors.Wrapf(err, "failed to propagate Conflict update of %s to BlockDAG", event.TransactionID))
		}
	}))

	b.Ledger.Events.TransactionBooked.Attach(event.NewClosure(func(e *ledger.TransactionBookedEvent) {
		contextBlockID := models.BlockIDFromContext(e.Context)

		for _, block := range b.attachments.Get(e.TransactionID) {
			if contextBlockID != block.ID() {
				b.bookingOrder.Queue(block)
			}
		}
	}))
}

// region FORK LOGIC ///////////////////////////////////////////////////////////////////////////////////////////////////

// PropagateForkedConflict propagates the forked ConflictID to the future cone of the attachments of the given Transaction.
func (b *Booker) PropagateForkedConflict(transactionID, addedConflictID utxo.TransactionID, removedConflictIDs utxo.TransactionIDs) (err error) {
	for blockWalker := walker.New[*Block]().PushAll(b.attachments.Get(transactionID)...); blockWalker.HasNext(); {
		block := blockWalker.Next()

		updated, propagateFurther, forkErr := b.propagateForkedConflict(block, addedConflictID, removedConflictIDs)
		if forkErr != nil {
			blockWalker.StopWalk()
			return errors.Wrapf(forkErr, "failed to propagate forked ConflictID %s to future cone of %s", addedConflictID, block.ID())
		}
		if !updated {
			continue
		}

		b.Events.BlockConflictAdded.Trigger(&BlockConflictAddedEvent{
			Block:             block,
			ConflictID:        addedConflictID,
			ParentConflictIDs: removedConflictIDs,
		})

		if propagateFurther {
			blockWalker.PushAll(b.strongChildren(block)...)
		}
	}
	return nil
}

func (b *Booker) propagateForkedConflict(block *Block, addedConflictID utxo.TransactionID, removedConflictIDs utxo.TransactionIDs) (propagated, propagateFurther bool, err error) {
	b.bookingMutex.Lock(block.ID())
	defer b.bookingMutex.Unlock(block.ID())

	if !block.IsBooked() {
		return false, false, nil
	}

	if structureDetails := block.StructureDetails(); structureDetails.IsPastMarker() {
		if err = b.propagateForkedTransactionToMarkerFutureCone(structureDetails.PastMarkers().Marker(), addedConflictID, removedConflictIDs); err != nil {
			return false, false, errors.Wrapf(err, "failed to propagate conflict %s to future cone of %v", addedConflictID, structureDetails.PastMarkers().Marker())
		}
		return true, false, nil
	}

	propagated = b.updateBlockConflicts(block, addedConflictID, removedConflictIDs)
	// We only need to propagate further (in the block's future cone) if the block was updated.
	return propagated, propagated, nil
}

func (b *Booker) updateBlockConflicts(block *Block, addedConflict utxo.TransactionID, parentConflicts utxo.TransactionIDs) (updated bool) {
	if _, conflictIDs := b.blockBookingDetails(block); !conflictIDs.HasAll(parentConflicts) {
		return false
	}

	return block.AddConflictID(addedConflict)
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
	b.Events.MarkerConflictAdded.Trigger(&MarkerConflictAddedEvent{
		Marker:            currentMarker,
		ConflictID:        newConflictID,
		ParentConflictIDs: removedConflictIDs,
	})

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

func (b *Booker) rLockBlockSequences(block *Block) bool {
	return block.StructureDetails().PastMarkers().ForEachSorted(func(sequenceID markers.SequenceID, _ markers.Index) bool {
		b.markerManager.SequenceMutex.RLock(sequenceID)
		return true
	})
}

func (b *Booker) rUnlockBlockSequences(block *Block) bool {
	return block.StructureDetails().PastMarkers().ForEachSorted(func(sequenceID markers.SequenceID, _ markers.Index) bool {
		b.markerManager.SequenceMutex.RUnlock(sequenceID)
		return true
	})
}

// isReferenceValid checks if the reference between the child and its parent is valid.
func isReferenceValid(child *Block, parent *Block) (err error) {
	if parent.IsInvalid() {
		return errors.Errorf("parent %s of child %s is marked as invalid", parent.ID(), child.ID())
	}

	return nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithMarkerManagerOptions(opts ...options.Option[markermanager.MarkerManager[models.BlockID, *Block]]) options.Option[Booker] {
	return func(b *Booker) {
		b.optsMarkerManager = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
