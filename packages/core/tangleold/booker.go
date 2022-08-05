package tangleold

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/cerrors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/generics/walker"
	"github.com/iotaledger/hive.go/core/syncutils"

	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/markers"
)

// region booker ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Booker is a Tangle component that takes care of booking Blocks and Transactions by assigning them to the
// corresponding Conflict of the ledger state.
type Booker struct {
	// Events is a dictionary for the Booker related Events.
	Events *BookerEvents

	MarkersManager *ConflictMarkersMapper

	tangle              *Tangle
	payloadBookingMutex *syncutils.DAGMutex[BlockID]
	bookingMutex        *syncutils.DAGMutex[BlockID]
	sequenceMutex       *syncutils.DAGMutex[markers.SequenceID]
}

// NewBooker is the constructor of a Booker.
func NewBooker(tangle *Tangle) (blockBooker *Booker) {
	blockBooker = &Booker{
		Events:              NewBookerEvents(),
		tangle:              tangle,
		MarkersManager:      NewConflictMarkersMapper(tangle),
		payloadBookingMutex: syncutils.NewDAGMutex[BlockID](),
		bookingMutex:        syncutils.NewDAGMutex[BlockID](),
		sequenceMutex:       syncutils.NewDAGMutex[markers.SequenceID](),
	}

	return
}

// Setup sets up the behavior of the component by making it attach to the relevant events of other components.
func (b *Booker) Setup() {
	b.tangle.Solidifier.Events.BlockSolid.Hook(event.NewClosure(func(event *BlockSolidEvent) {
		b.book(event.Block)
	}))

	b.tangle.Ledger.Events.TransactionConflictIDUpdated.Hook(event.NewClosure(func(event *ledger.TransactionConflictIDUpdatedEvent) {
		if err := b.PropagateForkedConflict(event.TransactionID, event.AddedConflictID, event.RemovedConflictIDs); err != nil {
			b.Events.Error.Trigger(errors.Errorf("failed to propagate Conflict update of %s to tangle: %w", event.TransactionID, err))
		}
	}))

	b.tangle.Ledger.Events.TransactionBooked.Attach(event.NewClosure(func(event *ledger.TransactionBookedEvent) {
		b.processBookedTransaction(event.TransactionID, BlockIDFromContext(event.Context))
	}))

	b.tangle.Booker.Events.BlockBooked.Attach(event.NewClosure(func(event *BlockBookedEvent) {
		b.propagateBooking(event.BlockID)
	}))
}

// BlockConflictIDs returns the ConflictIDs of the given Block.
func (b *Booker) BlockConflictIDs(blockID BlockID) (conflictIDs *set.AdvancedSet[utxo.TransactionID], err error) {
	if blockID == EmptyBlockID {
		return set.NewAdvancedSet[utxo.TransactionID](), nil
	}

	if _, _, conflictIDs, err = b.blockBookingDetails(blockID); err != nil {
		err = errors.Errorf("failed to retrieve booking details of Block with %s: %w", blockID, err)
	}

	return
}

// PayloadConflictIDs returns the ConflictIDs of the payload contained in the given Block.
func (b *Booker) PayloadConflictIDs(blockID BlockID) (conflictIDs *set.AdvancedSet[utxo.TransactionID], err error) {
	conflictIDs = set.NewAdvancedSet[utxo.TransactionID]()

	b.tangle.Storage.Block(blockID).Consume(func(block *Block) {
		transaction, isTransaction := block.Payload().(utxo.Transaction)
		if !isTransaction {
			return
		}

		b.tangle.Ledger.Storage.CachedTransactionMetadata(transaction.ID()).Consume(func(transactionMetadata *ledger.TransactionMetadata) {
			resolvedConflictIDs := b.tangle.Ledger.ConflictDAG.UnconfirmedConflicts(transactionMetadata.ConflictIDs())
			conflictIDs.AddAll(resolvedConflictIDs)
		})
	})

	return
}

// Shutdown shuts down the Booker and persists its state.
func (b *Booker) Shutdown() {
	b.MarkersManager.Shutdown()
}

// region BOOK PAYLOAD LOGIC ///////////////////////////////////////////////////////////////////////////////////////////

// book books the Payload of a Block.
func (b *Booker) book(block *Block) {
	b.payloadBookingMutex.Lock(block.ID())
	defer b.payloadBookingMutex.Unlock(block.ID())

	b.tangle.Storage.BlockMetadata(block.ID()).Consume(func(blockMetadata *BlockMetadata) {
		if !b.readyForBooking(block, blockMetadata) {
			return
		}

		if tx, isTx := block.Payload().(utxo.Transaction); isTx {
			if !b.bookTransaction(blockMetadata.ID(), tx) {
				return
			}
		}

		if err := b.BookBlock(block, blockMetadata); err != nil {
			b.Events.Error.Trigger(errors.Errorf("failed to book block with %s after booking its payload: %w", block.ID(), err))
		}
	})
}

func (b *Booker) bookTransaction(blockID BlockID, tx utxo.Transaction) (success bool) {
	if cachedAttachment, stored := b.tangle.Storage.StoreAttachment(tx.ID(), blockID); stored {
		cachedAttachment.Release()
	}

	if err := b.tangle.Ledger.StoreAndProcessTransaction(BlockIDToContext(context.Background(), blockID), tx); err != nil {
		if !errors.Is(err, ledger.ErrTransactionUnsolid) {
			// TODO: handle invalid transactions (possibly need to attach to invalid event though)
			//  delete attachments of transaction
			b.Events.Error.Trigger(errors.Errorf("failed to book transaction with %s: %w", tx.ID(), err))
		}

		return false
	}

	return true
}

func (b *Booker) processBookedTransaction(id utxo.TransactionID, blockIDToIgnore BlockID) {
	b.tangle.Storage.Attachments(id).Consume(func(attachment *Attachment) {
		event.Loop.Submit(func() {
			b.tangle.Storage.Block(attachment.BlockID()).Consume(func(block *Block) {
				b.tangle.Storage.BlockMetadata(attachment.BlockID()).Consume(func(blockMetadata *BlockMetadata) {
					if attachment.BlockID() == blockIDToIgnore {
						return
					}

					b.payloadBookingMutex.RLock(block.Parents()...)
					b.payloadBookingMutex.Lock(attachment.BlockID())
					defer b.payloadBookingMutex.Unlock(attachment.BlockID())
					defer b.payloadBookingMutex.RUnlock(block.Parents()...)

					if !b.readyForBooking(block, blockMetadata) {
						return
					}

					if err := b.BookBlock(block, blockMetadata); err != nil {
						b.Events.Error.Trigger(errors.Errorf("failed to book block with %s when processing booked transaction %s: %w", attachment.BlockID(), id, err))
					}
				})
			})
		})
	})
}

func (b *Booker) propagateBooking(blockID BlockID) {
	b.tangle.Storage.Children(blockID).Consume(func(child *Child) {
		event.Loop.Submit(func() {
			b.tangle.Storage.Block(child.ChildBlockID()).Consume(func(approvingBlock *Block) {
				b.book(approvingBlock)
			})
		})
	})
}

func (b *Booker) readyForBooking(block *Block, blockMetadata *BlockMetadata) (ready bool) {
	if !blockMetadata.IsSolid() || blockMetadata.IsBooked() {
		return false
	}

	ready = true
	block.ForEachParent(func(parent Parent) {
		b.tangle.Storage.BlockMetadata(parent.ID).Consume(func(blockMetadata *BlockMetadata) {
			ready = ready && blockMetadata.IsBooked()
		})
	})
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BOOK LOGIC ///////////////////////////////////////////////////////////////////////////////////////////////////

// BookBlock tries to book the given Block (and potentially its contained Transaction) into the ledger and the Tangle.
// It fires a BlockBooked event if it succeeds. If the Block is invalid it fires a BlockInvalid event.
// Booking a block essentially means that parents are examined, the conflict of the block determined based on the
// conflict inheritance rules of the like switch and markers are inherited. If everything is valid, the block is marked
// as booked. Following, the block conflict is set, and it can continue in the dataflow to add support to the determined
// conflicts and markers.
func (b *Booker) BookBlock(block *Block, blockMetadata *BlockMetadata) (err error) {
	b.bookingMutex.RLock(block.Parents()...)
	defer b.bookingMutex.RUnlock(block.Parents()...)
	b.bookingMutex.Lock(block.ID())
	defer b.bookingMutex.Unlock(block.ID())

	if err = b.inheritConflictIDs(block, blockMetadata); err != nil {
		err = errors.Errorf("failed to inherit ConflictIDs of Block with %s: %w", blockMetadata.ID(), err)
		return
	}

	blockMetadata.SetBooked(true)

	b.Events.BlockBooked.Trigger(&BlockBookedEvent{block.ID()})

	return
}

func (b *Booker) inheritConflictIDs(block *Block, blockMetadata *BlockMetadata) (err error) {
	structureDetails, _, inheritedConflictIDs, bookingDetailsErr := b.determineBookingDetails(block)
	if bookingDetailsErr != nil {
		return errors.Errorf("failed to determine booking details of Block with %s: %w", block.ID(), bookingDetailsErr)
	}

	inheritedStructureDetails, newSequenceCreated := b.MarkersManager.InheritStructureDetails(block, structureDetails)
	blockMetadata.SetStructureDetails(inheritedStructureDetails)

	if newSequenceCreated {
		b.MarkersManager.SetConflictIDs(inheritedStructureDetails.PastMarkers().Marker(), inheritedConflictIDs)
		return nil
	}

	// TODO: do not retrieve markers conflicts once again, determineBookingDetails already does it
	pastMarkersConflictIDs, inheritedStructureDetailsConflictIDsErr := b.conflictIDsFromStructureDetails(inheritedStructureDetails)
	if inheritedStructureDetailsConflictIDsErr != nil {
		return errors.Errorf("failed to determine ConflictIDs of inherited StructureDetails of Block with %s: %w", block.ID(), inheritedStructureDetailsConflictIDsErr)
	}

	addedConflictIDs := inheritedConflictIDs.Clone()
	addedConflictIDs.DeleteAll(pastMarkersConflictIDs)

	subtractedConflictIDs := pastMarkersConflictIDs.Clone()
	subtractedConflictIDs.DeleteAll(inheritedConflictIDs)

	if addedConflictIDs.Size()+subtractedConflictIDs.Size() == 0 {
		return nil
	}

	if inheritedStructureDetails.IsPastMarker() {
		b.MarkersManager.SetConflictIDs(inheritedStructureDetails.PastMarkers().Marker(), inheritedConflictIDs)
		return nil
	}

	if addedConflictIDs.Size() != 0 {
		blockMetadata.SetAddedConflictIDs(addedConflictIDs)
	}

	if subtractedConflictIDs.Size() != 0 {
		blockMetadata.SetSubtractedConflictIDs(subtractedConflictIDs)
	}

	return nil
}

// determineBookingDetails determines the booking details of an unbooked Block.
func (b *Booker) determineBookingDetails(block *Block) (parentsStructureDetails []*markers.StructureDetails, parentsPastMarkersConflictIDs, inheritedConflictIDs *set.AdvancedSet[utxo.TransactionID], err error) {
	inheritedConflictIDs, err = b.PayloadConflictIDs(block.ID())
	if err != nil {
		return
	}

	parentsStructureDetails, parentsPastMarkersConflictIDs, strongParentsConflictIDs, bookingDetailsErr := b.collectStrongParentsBookingDetails(block)
	if bookingDetailsErr != nil {
		err = errors.Errorf("failed to retrieve booking details of parents of Block with %s: %w", block.ID(), bookingDetailsErr)
		return
	}

	weakPayloadConflictIDs, weakParentsErr := b.collectWeakParentsConflictIDs(block)
	if weakParentsErr != nil {
		return nil, nil, nil, errors.Errorf("failed to collect weak parents of %s: %w", block.ID(), weakParentsErr)
	}

	likedConflictIDs, dislikedConflictIDs, shallowLikeErr := b.collectShallowLikedParentsConflictIDs(block)
	if shallowLikeErr != nil {
		return nil, nil, nil, errors.Errorf("failed to collect shallow likes of %s: %w", block.ID(), shallowLikeErr)
	}

	inheritedConflictIDs.AddAll(strongParentsConflictIDs)
	inheritedConflictIDs.AddAll(weakPayloadConflictIDs)
	inheritedConflictIDs.AddAll(likedConflictIDs)
	inheritedConflictIDs.DeleteAll(b.tangle.Ledger.Utils.ConflictIDsInFutureCone(dislikedConflictIDs))

	return parentsStructureDetails, parentsPastMarkersConflictIDs, b.tangle.Ledger.ConflictDAG.UnconfirmedConflicts(inheritedConflictIDs), nil
}

// blockBookingDetails returns the Conflict and Marker related details of the given Block.
func (b *Booker) blockBookingDetails(blockID BlockID) (structureDetails *markers.StructureDetails, pastMarkersConflictIDs, blockConflictIDs *set.AdvancedSet[utxo.TransactionID], err error) {
	pastMarkersConflictIDs = set.NewAdvancedSet[utxo.TransactionID]()
	blockConflictIDs = set.NewAdvancedSet[utxo.TransactionID]()

	if !b.tangle.Storage.BlockMetadata(blockID).Consume(func(blockMetadata *BlockMetadata) {
		structureDetails = blockMetadata.StructureDetails()
		if pastMarkersConflictIDs, err = b.conflictIDsFromStructureDetails(structureDetails); err != nil {
			err = errors.Errorf("failed to retrieve ConflictIDs from Structure Details %s of block with %s: %w", structureDetails, blockMetadata.ID(), err)
			return
		}
		blockConflictIDs.AddAll(pastMarkersConflictIDs)

		if addedConflictIDs := blockMetadata.AddedConflictIDs(); !addedConflictIDs.IsEmpty() {
			blockConflictIDs.AddAll(addedConflictIDs)
		}

		if subtractedConflictIDs := blockMetadata.SubtractedConflictIDs(); !subtractedConflictIDs.IsEmpty() {
			blockConflictIDs.DeleteAll(subtractedConflictIDs)
		}
	}) {
		err = errors.Errorf("failed to retrieve BlockMetadata with %s: %w", blockID, cerrors.ErrFatal)
	}

	return structureDetails, pastMarkersConflictIDs, blockConflictIDs, err
}

func (b *Booker) conflictIDsFromMetadata(meta *BlockMetadata) (conflictIDs utxo.TransactionIDs, err error) {
	if conflictIDs, err = b.conflictIDsFromStructureDetails(meta.StructureDetails()); err != nil {
		return nil, errors.Errorf("failed to retrieve ConflictIDs from Structure Details %s of block with %s: %w", meta.StructureDetails(), meta.ID(), err)
	}

	if addedConflictIDs := meta.AddedConflictIDs(); !addedConflictIDs.IsEmpty() {
		conflictIDs.AddAll(addedConflictIDs)
	}

	if subtractedConflictIDs := meta.SubtractedConflictIDs(); !subtractedConflictIDs.IsEmpty() {
		conflictIDs.DeleteAll(subtractedConflictIDs)
	}

	return conflictIDs, nil
}

// conflictIDsFromStructureDetails returns the ConflictIDs from StructureDetails.
func (b *Booker) conflictIDsFromStructureDetails(structureDetails *markers.StructureDetails) (structureDetailsConflictIDs utxo.TransactionIDs, err error) {
	if structureDetails == nil {
		return nil, errors.Errorf("StructureDetails is empty: %w", cerrors.ErrFatal)
	}

	structureDetailsConflictIDs = utxo.NewTransactionIDs()
	// obtain all the Markers
	structureDetails.PastMarkers().ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
		conflictIDs := b.MarkersManager.ConflictIDs(markers.NewMarker(sequenceID, index))
		structureDetailsConflictIDs.AddAll(conflictIDs)
		return true
	})

	return
}

// collectStrongParentsBookingDetails returns the booking details of a Block's strong parents.
func (b *Booker) collectStrongParentsBookingDetails(block *Block) (parentsStructureDetails []*markers.StructureDetails, parentsPastMarkersConflictIDs, parentsConflictIDs *set.AdvancedSet[utxo.TransactionID], err error) {
	parentsStructureDetails = make([]*markers.StructureDetails, 0)
	parentsPastMarkersConflictIDs = set.NewAdvancedSet[utxo.TransactionID]()
	parentsConflictIDs = set.NewAdvancedSet[utxo.TransactionID]()

	block.ForEachParentByType(StrongParentType, func(parentBlockID BlockID) bool {
		parentStructureDetails, parentPastMarkersConflictIDs, parentConflictIDs, parentErr := b.blockBookingDetails(parentBlockID)
		if parentErr != nil {
			err = errors.Errorf("failed to retrieve booking details of Block with %s: %w", parentBlockID, parentErr)
			return false
		}

		parentsStructureDetails = append(parentsStructureDetails, parentStructureDetails)
		parentsPastMarkersConflictIDs.AddAll(parentPastMarkersConflictIDs)
		parentsConflictIDs.AddAll(parentConflictIDs)

		return true
	})

	return
}

// collectShallowLikedParentsConflictIDs adds the ConflictIDs of the shallow like reference and removes all its conflicts from
// the supplied ArithmeticConflictIDs.
func (b *Booker) collectShallowLikedParentsConflictIDs(block *Block) (collectedLikedConflictIDs, collectedDislikedConflictIDs *set.AdvancedSet[utxo.TransactionID], err error) {
	collectedLikedConflictIDs = set.NewAdvancedSet[utxo.TransactionID]()
	collectedDislikedConflictIDs = set.NewAdvancedSet[utxo.TransactionID]()
	block.ForEachParentByType(ShallowLikeParentType, func(parentBlockID BlockID) bool {
		if !b.tangle.Storage.Block(parentBlockID).Consume(func(block *Block) {
			transaction, isTransaction := block.Payload().(utxo.Transaction)
			if !isTransaction {
				err = errors.Errorf("%s referenced by a shallow like of %s does not contain a Transaction: %w", parentBlockID, block.ID(), cerrors.ErrFatal)
				return
			}

			likedConflictIDs, likedConflictsErr := b.tangle.Ledger.Utils.TransactionConflictIDs(transaction.ID())
			if likedConflictsErr != nil {
				err = errors.Errorf("failed to retrieve liked ConflictIDs of Transaction with %s contained in %s referenced by a shallow like of %s: %w", transaction.ID(), parentBlockID, block.ID(), likedConflictsErr)
				return
			}
			collectedLikedConflictIDs.AddAll(likedConflictIDs)

			for it := b.tangle.Ledger.Utils.ConflictingTransactions(transaction.ID()).Iterator(); it.HasNext(); {
				conflictingTransactionID := it.Next()
				dislikedConflicts, dislikedConflictsErr := b.tangle.Ledger.Utils.TransactionConflictIDs(conflictingTransactionID)
				if dislikedConflictsErr != nil {
					err = errors.Errorf("failed to retrieve disliked ConflictIDs of Transaction with %s contained in %s referenced by a shallow like of %s: %w", conflictingTransactionID, parentBlockID, block.ID(), dislikedConflictsErr)
					return
				}
				collectedDislikedConflictIDs.AddAll(dislikedConflicts)
			}
		}) {
			err = errors.Errorf("failed to load BlockMetadata of shallow like with %s: %w", parentBlockID, cerrors.ErrFatal)
		}

		return err == nil
	})

	return collectedLikedConflictIDs, collectedDislikedConflictIDs, err
}

// collectShallowDislikedParentsConflictIDs removes the ConflictIDs of the shallow dislike reference and all its conflicts from
// the supplied ArithmeticConflictIDs.
func (b *Booker) collectWeakParentsConflictIDs(block *Block) (payloadConflictIDs *set.AdvancedSet[utxo.TransactionID], err error) {
	payloadConflictIDs = set.NewAdvancedSet[utxo.TransactionID]()
	block.ForEachParentByType(WeakParentType, func(parentBlockID BlockID) bool {
		if !b.tangle.Storage.Block(parentBlockID).Consume(func(block *Block) {
			transaction, isTransaction := block.Payload().(utxo.Transaction)
			// Payloads other than Transactions are MasterConflict
			if !isTransaction {
				return
			}

			weakReferencePayloadConflict, weakReferenceErr := b.tangle.Ledger.Utils.TransactionConflictIDs(transaction.ID())
			if weakReferenceErr != nil {
				err = errors.Errorf("failed to retrieve ConflictIDs of Transaction with %s contained in %s weakly referenced by %s: %w", transaction.ID(), parentBlockID, block.ID(), weakReferenceErr)
				return
			}

			payloadConflictIDs.AddAll(weakReferencePayloadConflict)
		}) {
			err = errors.Errorf("failed to load BlockMetadata of %s weakly referenced by %s: %w", parentBlockID, block.ID(), cerrors.ErrFatal)
		}

		return err == nil
	})

	return payloadConflictIDs, err
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region FORK LOGIC ///////////////////////////////////////////////////////////////////////////////////////////////////

// PropagateForkedConflict propagates the forked ConflictID to the future cone of the attachments of the given Transaction.
func (b *Booker) PropagateForkedConflict(transactionID utxo.TransactionID, addedConflictID utxo.TransactionID, removedConflictIDs *set.AdvancedSet[utxo.TransactionID]) (err error) {
	b.tangle.Utils.WalkBlockMetadata(func(blockMetadata *BlockMetadata, blockWalker *walker.Walker[BlockID]) {
		updated, forkErr := b.propagateForkedConflict(blockMetadata, addedConflictID, removedConflictIDs)
		if forkErr != nil {
			blockWalker.StopWalk()
			err = errors.Errorf("failed to propagate forked ConflictID %s to future cone of %s: %w", addedConflictID, blockMetadata.ID(), forkErr)
			return
		}
		if !updated {
			return
		}

		b.Events.BlockConflictUpdated.Trigger(&BlockConflictUpdatedEvent{
			BlockID:    blockMetadata.ID(),
			ConflictID: addedConflictID,
		})

		for approvingBlockID := range b.tangle.Utils.ApprovingBlockIDs(blockMetadata.ID(), StrongChild) {
			blockWalker.Push(approvingBlockID)
		}
	}, b.tangle.Storage.AttachmentBlockIDs(transactionID), false)

	return
}

func (b *Booker) propagateForkedConflict(blockMetadata *BlockMetadata, addedConflictID utxo.TransactionID, removedConflictIDs *set.AdvancedSet[utxo.TransactionID]) (propagated bool, err error) {
	b.bookingMutex.Lock(blockMetadata.ID())
	defer b.bookingMutex.Unlock(blockMetadata.ID())

	if !blockMetadata.IsBooked() {
		return false, nil
	}

	if structureDetails := blockMetadata.StructureDetails(); structureDetails.IsPastMarker() {
		if err = b.propagateForkedTransactionToMarkerFutureCone(structureDetails.PastMarkers().Marker(), addedConflictID, removedConflictIDs); err != nil {
			return false, errors.Errorf("failed to propagate conflict%s to future cone of %s: %w", addedConflictID, structureDetails.PastMarkers().Marker(), err)
		}
		return true, nil
	}

	return b.updateBlockConflicts(blockMetadata, addedConflictID, removedConflictIDs)
}

func (b *Booker) updateBlockConflicts(blockMetadata *BlockMetadata, addedConflict utxo.TransactionID, parentConflicts utxo.TransactionIDs) (updated bool, err error) {
	blockMetadata.StructureDetails().PastMarkers().ForEachSorted(func(sequenceID markers.SequenceID, _ markers.Index) bool {
		b.sequenceMutex.RLock(sequenceID)
		return true
	})

	defer func() {
		blockMetadata.StructureDetails().PastMarkers().ForEachSorted(func(sequenceID markers.SequenceID, _ markers.Index) bool {
			b.sequenceMutex.RUnlock(sequenceID)
			return true
		})
	}()

	conflictIDs, err := b.conflictIDsFromMetadata(blockMetadata)
	if err != nil {
		return false, errors.Errorf("failed to retrieve ConflictIDs of %s: %w", blockMetadata.ID(), err)
	}

	if !conflictIDs.HasAll(parentConflicts) {
		return false, nil
	}

	if !blockMetadata.AddConflictID(addedConflict) {
		return false, nil
	}

	return true, nil
}

// propagateForkedTransactionToMarkerFutureCone propagates a newly created ConflictID into the future cone of the given Marker.
func (b *Booker) propagateForkedTransactionToMarkerFutureCone(marker markers.Marker, conflictID utxo.TransactionID, removedConflictIDs *set.AdvancedSet[utxo.TransactionID]) (err error) {
	markerWalker := walker.New[markers.Marker](false)
	markerWalker.Push(marker)

	for markerWalker.HasNext() {
		currentMarker := markerWalker.Next()

		if err = b.forkSingleMarker(currentMarker, conflictID, removedConflictIDs, markerWalker); err != nil {
			err = errors.Errorf("failed to propagate Conflict%s to Blocks approving %s: %w", conflictID, currentMarker, err)
			return
		}
	}

	return
}

// forkSingleMarker propagates a newly created ConflictID to a single marker and queues the next elements that need to be
// visited.
func (b *Booker) forkSingleMarker(currentMarker markers.Marker, newConflictID utxo.TransactionID, removedConflictIDs *set.AdvancedSet[utxo.TransactionID], markerWalker *walker.Walker[markers.Marker]) (err error) {
	b.sequenceMutex.Lock(currentMarker.SequenceID())
	defer b.sequenceMutex.Unlock(currentMarker.SequenceID())

	// update ConflictID mapping
	newConflictIDs := b.MarkersManager.ConflictIDs(currentMarker)
	if !newConflictIDs.HasAll(removedConflictIDs) {
		return nil
	}

	if !newConflictIDs.Add(newConflictID) {
		return nil
	}

	if !b.MarkersManager.SetConflictIDs(currentMarker, newConflictIDs) {
		return nil
	}

	// trigger event
	b.Events.MarkerConflictAdded.Trigger(&MarkerConflictAddedEvent{
		Marker:        currentMarker,
		NewConflictID: newConflictID,
	})

	// propagate updates to later ConflictID mappings of the same sequence.
	b.MarkersManager.ForEachConflictIDMapping(currentMarker.SequenceID(), currentMarker.Index(), func(mappedMarker markers.Marker, _ *set.AdvancedSet[utxo.TransactionID]) {
		markerWalker.Push(mappedMarker)
	})

	// propagate updates to referencing markers of later sequences ...
	b.MarkersManager.ForEachMarkerReferencingMarker(currentMarker, func(referencingMarker markers.Marker) {
		markerWalker.Push(referencingMarker)
	})

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
