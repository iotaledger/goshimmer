package booker

import (
	"context"
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/cerrors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/generics/walker"
	"github.com/iotaledger/hive.go/core/syncutils"

	"github.com/iotaledger/goshimmer/packages/core/causalorder"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/markers"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/tangle"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
)

type Booker struct {
	// Events contains the Events of Tangle.
	Events *Events

	ledger        *ledger.Ledger
	bookingOrder  *causalorder.CausalOrder[models.BlockID, *Block]
	attachments   *attachments
	blocks        *memstorage.EpochStorage[models.BlockID, *Block]
	markerManager *MarkerManager
	bookingMutex  *syncutils.DAGMutex[models.BlockID]
	sequenceMutex *syncutils.DAGMutex[markers.SequenceID]

	// rootBlockProvider contains a function that is used to retrieve the root Blocks of the Tangle.
	rootBlockProvider func(models.BlockID) *Block

	// TODO: finish shutdown implementation, maybe replace with component state
	// maxDroppedEpoch contains the highest epoch.Index that has been dropped from the Tangle.
	maxDroppedEpoch epoch.Index

	// TODO: finish shutdown implementation, maybe replace with component state
	// isShutdown contains a flag that indicates whether the Booker was shut down.
	isShutdown bool

	*tangle.Tangle
}

func New(tangleInstance *tangle.Tangle, ledgerInstance *ledger.Ledger, rootBlockProvider func(models.BlockID) *Block, opts ...options.Option[Booker]) (booker *Booker) {
	booker = options.Apply(&Booker{
		ledger:        ledgerInstance,
		Tangle:        tangleInstance,
		Events:        newEvents(),
		attachments:   newAttachments(),
		blocks:        memstorage.NewEpochStorage[models.BlockID, *Block](),
		markerManager: NewMarkerManager(),
		bookingMutex:  syncutils.NewDAGMutex[models.BlockID](),
		sequenceMutex: syncutils.NewDAGMutex[markers.SequenceID](),

		rootBlockProvider: rootBlockProvider,
	}, opts)
	booker.bookingOrder = causalorder.New(booker.Block, (*Block).IsBooked, (*Block).setBooked, causalorder.WithReferenceValidator[models.BlockID](isReferenceValid))

	// TODO: is Hook making the booker single threaded? should we have some other method that would re-check a block
	//b.tangle.Booker.Events.BlockBooked.Attach(event.NewClosure(func(event *BlockBookedEvent) {
	//	b.propagateBooking(event.BlockID)
	//}))

	booker.bookingOrder.Events.Emit.Hook(event.NewClosure(booker.book))
	booker.bookingOrder.Events.Drop.Attach(event.NewClosure(func(block *Block) { booker.SetInvalid(block.Block) }))

	tangleInstance.Events.BlockSolid.Hook(event.NewClosure(func(block *tangle.Block) {
		if _, err := booker.Queue(NewBlock(block)); err != nil {
			panic(err)
		}
	}))

	ledgerInstance.Events.TransactionConflictIDUpdated.Hook(event.NewClosure(func(event *ledger.TransactionConflictIDUpdatedEvent) {
		if err := booker.PropagateForkedConflict(event.TransactionID, event.AddedConflictID, event.RemovedConflictIDs); err != nil {
			fmt.Println(errors.Errorf("failed to propagate Conflict update of %s to tangle: %w", event.TransactionID, err))
		}
	}))

	booker.ledger.Events.TransactionBooked.Attach(event.NewClosure(func(e *ledger.TransactionBookedEvent) {
		contextBlockID := models.BlockIDFromContext(e.Context)

		for _, block := range booker.attachments.Get(e.TransactionID) {
			if contextBlockID != block.ID() {
				booker.bookingOrder.Queue(block)
			}
		}
	}))

	return booker
}

func (b *Booker) Queue(block *Block) (wasQueued bool, err error) {
	// TODO: check whether too old?
	b.blocks.Get(block.ID().EpochIndex, true).Set(block.ID(), block)

	if wasQueued, err = b.isPayloadSolid(block); wasQueued {
		fmt.Println("Payload solid, queuing", block.ID())
		b.bookingOrder.Queue(block)
	}

	return
}

// Block retrieves a Block with metadata from the in-memory storage of the Tangle.
func (b *Booker) Block(id models.BlockID) (block *Block, exists bool) {
	b.RLock()
	defer b.RUnlock()

	if b.isShutdown {
		return nil, false
	}

	return b.block(id)
}

func (b *Booker) isPayloadSolid(block *Block) (isPayloadSolid bool, err error) {
	tx, isTx := block.Payload().(utxo.Transaction)
	if !isTx {
		return true, nil
	}

	b.attachments.Store(tx.ID(), block)

	if err = b.ledger.StoreAndProcessTransaction(
		models.BlockIDToContext(context.Background(), block.ID()), tx,
	); errors.Is(err, ledger.ErrTransactionUnsolid) {
		fmt.Println("Payload not solid")
		return false, nil
	}

	return err == nil, err
}

// block retrieves the Block with given id from the mem-storage.
func (b *Booker) block(id models.BlockID) (block *Block, exists bool) {
	if block = b.rootBlockProvider(id); block != nil {
		return block, true
	}

	if id.EpochIndex <= b.maxDroppedEpoch {
		return nil, false
	}

	return b.blocks.Get(id.EpochIndex, true).Get(id)
}

// isReferenceValid checks if the reference between the child and its parent is valid.
func isReferenceValid(child *Block, parent *Block) (isValid bool) {
	return !parent.IsInvalid()
}

func (b *Booker) book(block *Block) {
	b.bookingMutex.RLock(block.Parents()...)
	defer b.bookingMutex.RUnlock(block.Parents()...)
	b.bookingMutex.Lock(block.ID())
	defer b.bookingMutex.Unlock(block.ID())

	fmt.Printf("booking %s, %p \n", block.ID(), block)

	err := b.inheritConflictIDs(block)
	if err != nil {
		// TODO: print more elegantly using Error event
		fmt.Println("error inheriting conflict IDs:", err)
		b.SetInvalid(block.Block)
		return
	}

	block.setBooked()

	b.Events.BlockBooked.Trigger(block)
}

func (b *Booker) inheritConflictIDs(block *Block) (err error) {
	parentsStructureDetails, pastMarkersConflictIDs, inheritedConflictIDs, err := b.determineBookingDetails(block)
	if err != nil {
		return errors.Errorf("failed to inherit conflict IDs", err)
	}

	newStructureDetails := b.markerManager.ProcessBlock(block, parentsStructureDetails, inheritedConflictIDs)
	block.setStructureDetails(newStructureDetails)

	if newStructureDetails.IsPastMarker() {
		return
	}

	// TODO: maybe only update b.MarkersManager.SetConflictIDs when the conflicts have changed

	addedConflictIDs := inheritedConflictIDs.Clone()
	addedConflictIDs.DeleteAll(pastMarkersConflictIDs)
	block.AddAllAddedConflictIDs(addedConflictIDs)

	subtractedConflictIDs := pastMarkersConflictIDs.Clone()
	subtractedConflictIDs.DeleteAll(inheritedConflictIDs)
	block.AddAllSubtractedConflictIDs(subtractedConflictIDs)

	return
}

// determineBookingDetails determines the booking details of an unbooked Block.
func (b *Booker) determineBookingDetails(block *Block) (parentsStructureDetails []*markers.StructureDetails, parentsPastMarkersConflictIDs, inheritedConflictIDs utxo.TransactionIDs, err error) {
	inheritedConflictIDs = b.PayloadConflictIDs(block)

	parentsStructureDetails, parentsPastMarkersConflictIDs, strongParentsConflictIDs := b.collectStrongParentsBookingDetails(block)

	weakPayloadConflictIDs := b.collectWeakParentsConflictIDs(block)

	likedConflictIDs, dislikedConflictIDs, shallowLikeErr := b.collectShallowLikedParentsConflictIDs(block)
	if shallowLikeErr != nil {
		return nil, nil, nil, errors.Errorf("failed to collect shallow likes of %s: %w", block.ID(), shallowLikeErr)
	}

	inheritedConflictIDs.AddAll(strongParentsConflictIDs)
	inheritedConflictIDs.AddAll(weakPayloadConflictIDs)
	inheritedConflictIDs.AddAll(likedConflictIDs)
	inheritedConflictIDs.DeleteAll(b.ledger.Utils.ConflictIDsInFutureCone(dislikedConflictIDs))

	return parentsStructureDetails, b.ledger.ConflictDAG.UnconfirmedConflicts(parentsPastMarkersConflictIDs), b.ledger.ConflictDAG.UnconfirmedConflicts(inheritedConflictIDs), nil
}

// collectStrongParentsBookingDetails returns the booking details of a Block's strong parents.
func (b *Booker) collectStrongParentsBookingDetails(block *Block) (parentsStructureDetails []*markers.StructureDetails, parentsPastMarkersConflictIDs, parentsConflictIDs utxo.TransactionIDs) {
	parentsStructureDetails = make([]*markers.StructureDetails, 0)
	parentsPastMarkersConflictIDs = utxo.NewTransactionIDs()
	parentsConflictIDs = utxo.NewTransactionIDs()

	block.ForEachParentByType(models.StrongParentType, func(parentBlockID models.BlockID) bool {
		parentBlock, exists := b.Block(parentBlockID)
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
		payloadConflictIDs.AddAll(b.PayloadConflictIDs(block))

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
		transaction, isTransaction := block.Transaction()
		if !isTransaction {
			err = errors.Errorf("%s referenced by a shallow like of %s does not contain a Transaction: %w", parentBlockID, block.ID(), cerrors.ErrFatal)
			return false
		}

		collectedLikedConflictIDs.AddAll(b.PayloadConflictIDs(block))

		for it := b.ledger.Utils.ConflictingTransactions(transaction.ID()).Iterator(); it.HasNext(); {
			conflictingTransactionID := it.Next()
			dislikedConflicts, dislikedConflictsErr := b.ledger.Utils.TransactionConflictIDs(conflictingTransactionID)
			if dislikedConflictsErr != nil {
				err = errors.Errorf("failed to retrieve disliked ConflictIDs of Transaction with %s contained in %s referenced by a shallow like of %s: %w", conflictingTransactionID, parentBlockID, block.ID(), dislikedConflictsErr)
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
	pastMarkersConflictIDs = b.markerManager.ConflictIDsFromStructureDetails(block.StructureDetails())

	blockConflictIDs = utxo.NewTransactionIDs()
	blockConflictIDs.AddAll(pastMarkersConflictIDs)

	if addedConflictIDs := block.AddedConflictIDs(); !addedConflictIDs.IsEmpty() {
		blockConflictIDs.AddAll(addedConflictIDs)
	}

	if subtractedConflictIDs := block.SubtractedConflictIDs(); !subtractedConflictIDs.IsEmpty() {
		blockConflictIDs.DeleteAll(subtractedConflictIDs)
	}

	return pastMarkersConflictIDs, blockConflictIDs
}

// PayloadConflictIDs returns the ConflictIDs of the payload contained in the given Block.
func (b *Booker) PayloadConflictIDs(block *Block) (conflictIDs utxo.TransactionIDs) {
	conflictIDs = utxo.NewTransactionIDs()

	transaction, isTransaction := block.Transaction()
	if !isTransaction {
		return
	}

	b.ledger.Storage.CachedTransactionMetadata(transaction.ID()).Consume(func(transactionMetadata *ledger.TransactionMetadata) {
		conflictIDs.AddAll(transactionMetadata.ConflictIDs())
	})

	return
}

// region FORK LOGIC ///////////////////////////////////////////////////////////////////////////////////////////////////

// PropagateForkedConflict propagates the forked ConflictID to the future cone of the attachments of the given Transaction.
func (b *Booker) PropagateForkedConflict(transactionID utxo.TransactionID, addedConflictID utxo.TransactionID, removedConflictIDs *set.AdvancedSet[utxo.TransactionID]) (err error) {
	for blockWalker := walker.New[*Block]().PushAll(b.attachments.Get(transactionID)...); blockWalker.HasNext(); {
		block := blockWalker.Next()

		updated, forkErr := b.propagateForkedConflict(block, addedConflictID, removedConflictIDs)
		if forkErr != nil {
			blockWalker.StopWalk()
			return errors.Errorf("failed to propagate forked ConflictID %s to future cone of %s: %w", addedConflictID, block.ID(), forkErr)
		}
		if !updated {
			continue
		}

		b.Events.BlockConflictUpdated.Trigger(&BlockConflictUpdatedEvent{
			Block:      block,
			ConflictID: addedConflictID,
		})

		blockWalker.PushAll(b.StrongChildren(block)...)
	}
	return nil
}

func (b *Booker) StrongChildren(block *Block) []*Block {
	return lo.Filter(lo.Map(block.StrongChildren(), func(tangleChild *tangle.Block) (bookerChild *Block) {
		bookerChild, exists := b.Block(tangleChild.ID())
		if !exists {
			return nil
		}
		return bookerChild
	}), func(child *Block) bool {
		return child != nil
	})
}

func (b *Booker) propagateForkedConflict(block *Block, addedConflictID utxo.TransactionID, removedConflictIDs *set.AdvancedSet[utxo.TransactionID]) (propagated bool, err error) {
	b.bookingMutex.Lock(block.ID())
	defer b.bookingMutex.Unlock(block.ID())

	if !block.IsBooked() {
		return false, nil
	}

	if structureDetails := block.StructureDetails(); structureDetails.IsPastMarker() {
		if err = b.propagateForkedTransactionToMarkerFutureCone(structureDetails.PastMarkers().Marker(), addedConflictID, removedConflictIDs); err != nil {
			return false, errors.Errorf("failed to propagate conflict %s to future cone of %s: %w", addedConflictID, structureDetails.PastMarkers().Marker(), err)
		}
		return true, nil
	}

	return b.updateBlockConflicts(block, addedConflictID, removedConflictIDs), nil
}

func (b *Booker) updateBlockConflicts(block *Block, addedConflict utxo.TransactionID, parentConflicts utxo.TransactionIDs) (updated bool) {
	block.StructureDetails().PastMarkers().ForEachSorted(func(sequenceID markers.SequenceID, _ markers.Index) bool {
		b.sequenceMutex.RLock(sequenceID)
		return true
	})

	defer func() {
		block.StructureDetails().PastMarkers().ForEachSorted(func(sequenceID markers.SequenceID, _ markers.Index) bool {
			b.sequenceMutex.RUnlock(sequenceID)
			return true
		})
	}()

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
			return errors.Errorf("failed to propagate Conflict%s to Blocks approving %s: %w", conflictID, currentMarker, err)
		}
	}

	return
}

// forkSingleMarker propagates a newly created ConflictID to a single marker and queues the next elements that need to be
// visited.
func (b *Booker) forkSingleMarker(currentMarker markers.Marker, newConflictID utxo.TransactionID, removedConflictIDs utxo.TransactionIDs, markerWalker *walker.Walker[markers.Marker]) (err error) {
	b.sequenceMutex.Lock(currentMarker.SequenceID())
	defer b.sequenceMutex.Unlock(currentMarker.SequenceID())

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
		Marker:        currentMarker,
		NewConflictID: newConflictID,
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
