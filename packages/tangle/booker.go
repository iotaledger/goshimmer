package tangle

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/generics/walker"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/markers"
)

// region booker ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Booker is a Tangle component that takes care of booking Blocks and Transactions by assigning them to the
// corresponding Branch of the ledger state.
type Booker struct {
	// Events is a dictionary for the Booker related Events.
	Events *BookerEvents

	MarkersManager *BranchMarkersMapper

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
		MarkersManager:      NewBranchMarkersMapper(tangle),
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

	b.tangle.Ledger.Events.TransactionBranchIDUpdated.Hook(event.NewClosure(func(event *ledger.TransactionBranchIDUpdatedEvent) {
		if err := b.PropagateForkedBranch(event.TransactionID, event.AddedBranchID, event.RemovedBranchIDs); err != nil {
			b.Events.Error.Trigger(errors.Errorf("failed to propagate Branch update of %s to tangle: %w", event.TransactionID, err))
		}
	}))

	b.tangle.Ledger.Events.TransactionBooked.Attach(event.NewClosure(func(event *ledger.TransactionBookedEvent) {
		b.processBookedTransaction(event.TransactionID, BlockIDFromContext(event.Context))
	}))

	b.tangle.Booker.Events.BlockBooked.Attach(event.NewClosure(func(event *BlockBookedEvent) {
		b.propagateBooking(event.BlockID)
	}))
}

// BlockBranchIDs returns the BranchIDs of the given Block.
func (b *Booker) BlockBranchIDs(blockID BlockID) (branchIDs *set.AdvancedSet[utxo.TransactionID], err error) {
	if blockID == EmptyBlockID {
		return set.NewAdvancedSet[utxo.TransactionID](), nil
	}

	if _, _, branchIDs, err = b.blockBookingDetails(blockID); err != nil {
		err = errors.Errorf("failed to retrieve booking details of Block with %s: %w", blockID, err)
	}

	return
}

// PayloadBranchIDs returns the BranchIDs of the payload contained in the given Block.
func (b *Booker) PayloadBranchIDs(blockID BlockID) (branchIDs *set.AdvancedSet[utxo.TransactionID], err error) {
	branchIDs = set.NewAdvancedSet[utxo.TransactionID]()

	b.tangle.Storage.Block(blockID).Consume(func(block *Block) {
		transaction, isTransaction := block.Payload().(utxo.Transaction)
		if !isTransaction {
			return
		}

		b.tangle.Ledger.Storage.CachedTransactionMetadata(transaction.ID()).Consume(func(transactionMetadata *ledger.TransactionMetadata) {
			resolvedBranchIDs := b.tangle.Ledger.ConflictDAG.UnconfirmedConflicts(transactionMetadata.BranchIDs())
			branchIDs.AddAll(resolvedBranchIDs)
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
	b.tangle.Storage.Childs(blockID).Consume(func(child *Child) {
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
// Booking a block essentially means that parents are examined, the branch of the block determined based on the
// branch inheritance rules of the like switch and markers are inherited. If everything is valid, the block is marked
// as booked. Following, the block branch is set, and it can continue in the dataflow to add support to the determined
// branches and markers.
func (b *Booker) BookBlock(block *Block, blockMetadata *BlockMetadata) (err error) {
	b.bookingMutex.RLock(block.Parents()...)
	defer b.bookingMutex.RUnlock(block.Parents()...)
	b.bookingMutex.Lock(block.ID())
	defer b.bookingMutex.Unlock(block.ID())

	if err = b.inheritBranchIDs(block, blockMetadata); err != nil {
		err = errors.Errorf("failed to inherit BranchIDs of Block with %s: %w", blockMetadata.ID(), err)
		return
	}

	blockMetadata.SetBooked(true)

	b.Events.BlockBooked.Trigger(&BlockBookedEvent{block.ID()})

	return
}

func (b *Booker) inheritBranchIDs(block *Block, blockMetadata *BlockMetadata) (err error) {
	structureDetails, _, inheritedBranchIDs, bookingDetailsErr := b.determineBookingDetails(block)
	if bookingDetailsErr != nil {
		return errors.Errorf("failed to determine booking details of Block with %s: %w", block.ID(), bookingDetailsErr)
	}

	inheritedStructureDetails, newSequenceCreated := b.MarkersManager.InheritStructureDetails(block, structureDetails)
	blockMetadata.SetStructureDetails(inheritedStructureDetails)

	if newSequenceCreated {
		b.MarkersManager.SetBranchIDs(inheritedStructureDetails.PastMarkers().Marker(), inheritedBranchIDs)
		return nil
	}

	// TODO: do not retrieve markers branches once again, determineBookingDetails already does it
	pastMarkersBranchIDs, inheritedStructureDetailsBranchIDsErr := b.branchIDsFromStructureDetails(inheritedStructureDetails)
	if inheritedStructureDetailsBranchIDsErr != nil {
		return errors.Errorf("failed to determine BranchIDs of inherited StructureDetails of Block with %s: %w", block.ID(), inheritedStructureDetailsBranchIDsErr)
	}

	addedBranchIDs := inheritedBranchIDs.Clone()
	addedBranchIDs.DeleteAll(pastMarkersBranchIDs)

	subtractedBranchIDs := pastMarkersBranchIDs.Clone()
	subtractedBranchIDs.DeleteAll(inheritedBranchIDs)

	if addedBranchIDs.Size()+subtractedBranchIDs.Size() == 0 {
		return nil
	}

	if inheritedStructureDetails.IsPastMarker() {
		b.MarkersManager.SetBranchIDs(inheritedStructureDetails.PastMarkers().Marker(), inheritedBranchIDs)
		return nil
	}

	if addedBranchIDs.Size() != 0 {
		blockMetadata.SetAddedBranchIDs(addedBranchIDs)
	}

	if subtractedBranchIDs.Size() != 0 {
		blockMetadata.SetSubtractedBranchIDs(subtractedBranchIDs)
	}

	return nil
}

// determineBookingDetails determines the booking details of an unbooked Block.
func (b *Booker) determineBookingDetails(block *Block) (parentsStructureDetails []*markers.StructureDetails, parentsPastMarkersBranchIDs, inheritedBranchIDs *set.AdvancedSet[utxo.TransactionID], err error) {
	inheritedBranchIDs, err = b.PayloadBranchIDs(block.ID())
	if err != nil {
		return
	}

	parentsStructureDetails, parentsPastMarkersBranchIDs, strongParentsBranchIDs, bookingDetailsErr := b.collectStrongParentsBookingDetails(block)
	if bookingDetailsErr != nil {
		err = errors.Errorf("failed to retrieve booking details of parents of Block with %s: %w", block.ID(), bookingDetailsErr)
		return
	}

	weakPayloadBranchIDs, weakParentsErr := b.collectWeakParentsBranchIDs(block)
	if weakParentsErr != nil {
		return nil, nil, nil, errors.Errorf("failed to collect weak parents of %s: %w", block.ID(), weakParentsErr)
	}

	likedBranchIDs, dislikedBranchIDs, shallowLikeErr := b.collectShallowLikedParentsBranchIDs(block)
	if shallowLikeErr != nil {
		return nil, nil, nil, errors.Errorf("failed to collect shallow likes of %s: %w", block.ID(), shallowLikeErr)
	}

	inheritedBranchIDs.AddAll(strongParentsBranchIDs)
	inheritedBranchIDs.AddAll(weakPayloadBranchIDs)
	inheritedBranchIDs.AddAll(likedBranchIDs)
	inheritedBranchIDs.DeleteAll(b.tangle.Ledger.Utils.BranchIDsInFutureCone(dislikedBranchIDs))

	return parentsStructureDetails, parentsPastMarkersBranchIDs, b.tangle.Ledger.ConflictDAG.UnconfirmedConflicts(inheritedBranchIDs), nil
}

// blockBookingDetails returns the Branch and Marker related details of the given Block.
func (b *Booker) blockBookingDetails(blockID BlockID) (structureDetails *markers.StructureDetails, pastMarkersBranchIDs, blockBranchIDs *set.AdvancedSet[utxo.TransactionID], err error) {
	pastMarkersBranchIDs = set.NewAdvancedSet[utxo.TransactionID]()
	blockBranchIDs = set.NewAdvancedSet[utxo.TransactionID]()

	if !b.tangle.Storage.BlockMetadata(blockID).Consume(func(blockMetadata *BlockMetadata) {
		structureDetails = blockMetadata.StructureDetails()
		if pastMarkersBranchIDs, err = b.branchIDsFromStructureDetails(structureDetails); err != nil {
			err = errors.Errorf("failed to retrieve BranchIDs from Structure Details %s of block with %s: %w", structureDetails, blockMetadata.ID(), err)
			return
		}
		blockBranchIDs.AddAll(pastMarkersBranchIDs)

		if addedBranchIDs := blockMetadata.AddedBranchIDs(); !addedBranchIDs.IsEmpty() {
			blockBranchIDs.AddAll(addedBranchIDs)
		}

		if subtractedBranchIDs := blockMetadata.SubtractedBranchIDs(); !subtractedBranchIDs.IsEmpty() {
			blockBranchIDs.DeleteAll(subtractedBranchIDs)
		}
	}) {
		err = errors.Errorf("failed to retrieve BlockMetadata with %s: %w", blockID, cerrors.ErrFatal)
	}

	return structureDetails, pastMarkersBranchIDs, blockBranchIDs, err
}

func (b *Booker) branchIDsFromMetadata(meta *BlockMetadata) (branchIDs utxo.TransactionIDs, err error) {
	if branchIDs, err = b.branchIDsFromStructureDetails(meta.StructureDetails()); err != nil {
		return nil, errors.Errorf("failed to retrieve BranchIDs from Structure Details %s of block with %s: %w", meta.StructureDetails(), meta.ID(), err)
	}

	if addedBranchIDs := meta.AddedBranchIDs(); !addedBranchIDs.IsEmpty() {
		branchIDs.AddAll(addedBranchIDs)
	}

	if subtractedBranchIDs := meta.SubtractedBranchIDs(); !subtractedBranchIDs.IsEmpty() {
		branchIDs.DeleteAll(subtractedBranchIDs)
	}

	return branchIDs, nil
}

// branchIDsFromStructureDetails returns the BranchIDs from StructureDetails.
func (b *Booker) branchIDsFromStructureDetails(structureDetails *markers.StructureDetails) (structureDetailsBranchIDs utxo.TransactionIDs, err error) {
	if structureDetails == nil {
		return nil, errors.Errorf("StructureDetails is empty: %w", cerrors.ErrFatal)
	}

	structureDetailsBranchIDs = utxo.NewTransactionIDs()
	// obtain all the Markers
	structureDetails.PastMarkers().ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
		branchIDs := b.MarkersManager.BranchIDs(markers.NewMarker(sequenceID, index))
		structureDetailsBranchIDs.AddAll(branchIDs)
		return true
	})

	return
}

// collectStrongParentsBookingDetails returns the booking details of a Block's strong parents.
func (b *Booker) collectStrongParentsBookingDetails(block *Block) (parentsStructureDetails []*markers.StructureDetails, parentsPastMarkersBranchIDs, parentsBranchIDs *set.AdvancedSet[utxo.TransactionID], err error) {
	parentsStructureDetails = make([]*markers.StructureDetails, 0)
	parentsPastMarkersBranchIDs = set.NewAdvancedSet[utxo.TransactionID]()
	parentsBranchIDs = set.NewAdvancedSet[utxo.TransactionID]()

	block.ForEachParentByType(StrongParentType, func(parentBlockID BlockID) bool {
		parentStructureDetails, parentPastMarkersBranchIDs, parentBranchIDs, parentErr := b.blockBookingDetails(parentBlockID)
		if parentErr != nil {
			err = errors.Errorf("failed to retrieve booking details of Block with %s: %w", parentBlockID, parentErr)
			return false
		}

		parentsStructureDetails = append(parentsStructureDetails, parentStructureDetails)
		parentsPastMarkersBranchIDs.AddAll(parentPastMarkersBranchIDs)
		parentsBranchIDs.AddAll(parentBranchIDs)

		return true
	})

	return
}

// collectShallowLikedParentsBranchIDs adds the BranchIDs of the shallow like reference and removes all its conflicts from
// the supplied ArithmeticBranchIDs.
func (b *Booker) collectShallowLikedParentsBranchIDs(block *Block) (collectedLikedBranchIDs, collectedDislikedBranchIDs *set.AdvancedSet[utxo.TransactionID], err error) {
	collectedLikedBranchIDs = set.NewAdvancedSet[utxo.TransactionID]()
	collectedDislikedBranchIDs = set.NewAdvancedSet[utxo.TransactionID]()
	block.ForEachParentByType(ShallowLikeParentType, func(parentBlockID BlockID) bool {
		if !b.tangle.Storage.Block(parentBlockID).Consume(func(block *Block) {
			transaction, isTransaction := block.Payload().(utxo.Transaction)
			if !isTransaction {
				err = errors.Errorf("%s referenced by a shallow like of %s does not contain a Transaction: %w", parentBlockID, block.ID(), cerrors.ErrFatal)
				return
			}

			likedBranchIDs, likedBranchesErr := b.tangle.Ledger.Utils.TransactionBranchIDs(transaction.ID())
			if likedBranchesErr != nil {
				err = errors.Errorf("failed to retrieve liked BranchIDs of Transaction with %s contained in %s referenced by a shallow like of %s: %w", transaction.ID(), parentBlockID, block.ID(), likedBranchesErr)
				return
			}
			collectedLikedBranchIDs.AddAll(likedBranchIDs)

			for it := b.tangle.Ledger.Utils.ConflictingTransactions(transaction.ID()).Iterator(); it.HasNext(); {
				conflictingTransactionID := it.Next()
				dislikedBranches, dislikedBranchesErr := b.tangle.Ledger.Utils.TransactionBranchIDs(conflictingTransactionID)
				if dislikedBranchesErr != nil {
					err = errors.Errorf("failed to retrieve disliked BranchIDs of Transaction with %s contained in %s referenced by a shallow like of %s: %w", conflictingTransactionID, parentBlockID, block.ID(), dislikedBranchesErr)
					return
				}
				collectedDislikedBranchIDs.AddAll(dislikedBranches)
			}
		}) {
			err = errors.Errorf("failed to load BlockMetadata of shallow like with %s: %w", parentBlockID, cerrors.ErrFatal)
		}

		return err == nil
	})

	return collectedLikedBranchIDs, collectedDislikedBranchIDs, err
}

// collectShallowDislikedParentsBranchIDs removes the BranchIDs of the shallow dislike reference and all its conflicts from
// the supplied ArithmeticBranchIDs.
func (b *Booker) collectWeakParentsBranchIDs(block *Block) (payloadBranchIDs *set.AdvancedSet[utxo.TransactionID], err error) {
	payloadBranchIDs = set.NewAdvancedSet[utxo.TransactionID]()
	block.ForEachParentByType(WeakParentType, func(parentBlockID BlockID) bool {
		if !b.tangle.Storage.Block(parentBlockID).Consume(func(block *Block) {
			transaction, isTransaction := block.Payload().(utxo.Transaction)
			// Payloads other than Transactions are MasterBranch
			if !isTransaction {
				return
			}

			weakReferencePayloadBranch, weakReferenceErr := b.tangle.Ledger.Utils.TransactionBranchIDs(transaction.ID())
			if weakReferenceErr != nil {
				err = errors.Errorf("failed to retrieve BranchIDs of Transaction with %s contained in %s weakly referenced by %s: %w", transaction.ID(), parentBlockID, block.ID(), weakReferenceErr)
				return
			}

			payloadBranchIDs.AddAll(weakReferencePayloadBranch)
		}) {
			err = errors.Errorf("failed to load BlockMetadata of %s weakly referenced by %s: %w", parentBlockID, block.ID(), cerrors.ErrFatal)
		}

		return err == nil
	})

	return payloadBranchIDs, err
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region FORK LOGIC ///////////////////////////////////////////////////////////////////////////////////////////////////

// PropagateForkedBranch propagates the forked BranchID to the future cone of the attachments of the given Transaction.
func (b *Booker) PropagateForkedBranch(transactionID utxo.TransactionID, addedBranchID utxo.TransactionID, removedBranchIDs *set.AdvancedSet[utxo.TransactionID]) (err error) {
	b.tangle.Utils.WalkBlockMetadata(func(blockMetadata *BlockMetadata, blockWalker *walker.Walker[BlockID]) {
		updated, forkErr := b.propagateForkedBranch(blockMetadata, addedBranchID, removedBranchIDs)
		if forkErr != nil {
			blockWalker.StopWalk()
			err = errors.Errorf("failed to propagate forked BranchID %s to future cone of %s: %w", addedBranchID, blockMetadata.ID(), forkErr)
			return
		}
		if !updated {
			return
		}

		b.Events.BlockBranchUpdated.Trigger(&BlockBranchUpdatedEvent{
			BlockID:  blockMetadata.ID(),
			BranchID: addedBranchID,
		})

		for approvingBlockID := range b.tangle.Utils.ApprovingBlockIDs(blockMetadata.ID(), StrongChild) {
			blockWalker.Push(approvingBlockID)
		}
	}, b.tangle.Storage.AttachmentBlockIDs(transactionID), false)

	return
}

func (b *Booker) propagateForkedBranch(blockMetadata *BlockMetadata, addedBranchID utxo.TransactionID, removedBranchIDs *set.AdvancedSet[utxo.TransactionID]) (propagated bool, err error) {
	b.bookingMutex.Lock(blockMetadata.ID())
	defer b.bookingMutex.Unlock(blockMetadata.ID())

	if !blockMetadata.IsBooked() {
		return false, nil
	}

	if structureDetails := blockMetadata.StructureDetails(); structureDetails.IsPastMarker() {
		if err = b.propagateForkedTransactionToMarkerFutureCone(structureDetails.PastMarkers().Marker(), addedBranchID, removedBranchIDs); err != nil {
			return false, errors.Errorf("failed to propagate conflict%s to future cone of %s: %w", addedBranchID, structureDetails.PastMarkers().Marker(), err)
		}
		return true, nil
	}

	return b.updateBlockBranches(blockMetadata, addedBranchID, removedBranchIDs)
}

func (b *Booker) updateBlockBranches(blockMetadata *BlockMetadata, addedBranch utxo.TransactionID, parentConflicts utxo.TransactionIDs) (updated bool, err error) {
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

	branchIDs, err := b.branchIDsFromMetadata(blockMetadata)
	if err != nil {
		return false, errors.Errorf("failed to retrieve BranchIDs of %s: %w", blockMetadata.ID(), err)
	}

	if !branchIDs.HasAll(parentConflicts) {
		return false, nil
	}

	if !blockMetadata.AddBranchID(addedBranch) {
		return false, nil
	}

	return true, nil
}

// propagateForkedTransactionToMarkerFutureCone propagates a newly created BranchID into the future cone of the given Marker.
func (b *Booker) propagateForkedTransactionToMarkerFutureCone(marker markers.Marker, branchID utxo.TransactionID, removedBranchIDs *set.AdvancedSet[utxo.TransactionID]) (err error) {
	markerWalker := walker.New[markers.Marker](false)
	markerWalker.Push(marker)

	for markerWalker.HasNext() {
		currentMarker := markerWalker.Next()

		if err = b.forkSingleMarker(currentMarker, branchID, removedBranchIDs, markerWalker); err != nil {
			err = errors.Errorf("failed to propagate Conflict%s to Blocks approving %s: %w", branchID, currentMarker, err)
			return
		}
	}

	return
}

// forkSingleMarker propagates a newly created BranchID to a single marker and queues the next elements that need to be
// visited.
func (b *Booker) forkSingleMarker(currentMarker markers.Marker, newBranchID utxo.TransactionID, removedBranchIDs *set.AdvancedSet[utxo.TransactionID], markerWalker *walker.Walker[markers.Marker]) (err error) {
	b.sequenceMutex.Lock(currentMarker.SequenceID())
	defer b.sequenceMutex.Unlock(currentMarker.SequenceID())

	// update BranchID mapping
	newBranchIDs := b.MarkersManager.BranchIDs(currentMarker)
	if !newBranchIDs.HasAll(removedBranchIDs) {
		return nil
	}

	if !newBranchIDs.Add(newBranchID) {
		return nil
	}

	if !b.MarkersManager.SetBranchIDs(currentMarker, newBranchIDs) {
		return nil
	}

	// trigger event
	b.Events.MarkerBranchAdded.Trigger(&MarkerBranchAddedEvent{
		Marker:      currentMarker,
		NewBranchID: newBranchID,
	})

	// propagate updates to later BranchID mappings of the same sequence.
	b.MarkersManager.ForEachBranchIDMapping(currentMarker.SequenceID(), currentMarker.Index(), func(mappedMarker markers.Marker, _ *set.AdvancedSet[utxo.TransactionID]) {
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
