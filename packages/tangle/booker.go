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

// Booker is a Tangle component that takes care of booking Messages and Transactions by assigning them to the
// corresponding Branch of the ledger state.
type Booker struct {
	// Events is a dictionary for the Booker related Events.
	Events *BookerEvents

	MarkersManager *BranchMarkersMapper

	tangle              *Tangle
	payloadBookingMutex *syncutils.DAGMutex[MessageID]
	bookingMutex        *syncutils.DAGMutex[MessageID]
	sequenceMutex       *syncutils.DAGMutex[markers.SequenceID]
}

// NewBooker is the constructor of a Booker.
func NewBooker(tangle *Tangle) (messageBooker *Booker) {
	messageBooker = &Booker{
		Events:              NewBookerEvents(),
		tangle:              tangle,
		MarkersManager:      NewBranchMarkersMapper(tangle),
		payloadBookingMutex: syncutils.NewDAGMutex[MessageID](),
		bookingMutex:        syncutils.NewDAGMutex[MessageID](),
		sequenceMutex:       syncutils.NewDAGMutex[markers.SequenceID](),
	}

	return
}

// Setup sets up the behavior of the component by making it attach to the relevant events of other components.
func (b *Booker) Setup() {
	b.tangle.Solidifier.Events.MessageSolid.Hook(event.NewClosure(func(event *MessageSolidEvent) {
		b.book(event.Message)
	}))

	b.tangle.Ledger.Events.TransactionBranchIDUpdated.Hook(event.NewClosure(func(event *ledger.TransactionBranchIDUpdatedEvent) {
		if err := b.PropagateForkedBranch(event.TransactionID, event.AddedBranchID, event.RemovedBranchIDs); err != nil {
			b.Events.Error.Trigger(errors.Errorf("failed to propagate Branch update of %s to tangle: %w", event.TransactionID, err))
		}
	}))

	b.tangle.Ledger.Events.TransactionBooked.Attach(event.NewClosure(func(event *ledger.TransactionBookedEvent) {
		b.processBookedTransaction(event.TransactionID, MessageIDFromContext(event.Context))
	}))

	b.tangle.Booker.Events.MessageBooked.Attach(event.NewClosure(func(event *MessageBookedEvent) {
		b.propagateBooking(event.MessageID)
	}))
}

// MessageBranchIDs returns the BranchIDs of the given Message.
func (b *Booker) MessageBranchIDs(messageID MessageID) (branchIDs *set.AdvancedSet[utxo.TransactionID], err error) {
	if messageID == EmptyMessageID {
		return set.NewAdvancedSet[utxo.TransactionID](), nil
	}

	if _, _, branchIDs, err = b.messageBookingDetails(messageID); err != nil {
		err = errors.Errorf("failed to retrieve booking details of Message with %s: %w", messageID, err)
	}

	return
}

// PayloadBranchIDs returns the BranchIDs of the payload contained in the given Message.
func (b *Booker) PayloadBranchIDs(messageID MessageID) (branchIDs *set.AdvancedSet[utxo.TransactionID], err error) {
	branchIDs = set.NewAdvancedSet[utxo.TransactionID]()

	b.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		transaction, isTransaction := message.Payload().(utxo.Transaction)
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

// book books the Payload of a Message.
func (b *Booker) book(message *Message) {
	b.payloadBookingMutex.Lock(message.ID())
	defer b.payloadBookingMutex.Unlock(message.ID())

	b.tangle.Storage.MessageMetadata(message.ID()).Consume(func(messageMetadata *MessageMetadata) {
		if !b.readyForBooking(message, messageMetadata) {
			return
		}

		if tx, isTx := message.Payload().(utxo.Transaction); isTx {
			if !b.bookTransaction(messageMetadata.ID(), tx) {
				return
			}
		}

		if err := b.BookMessage(message, messageMetadata); err != nil {
			b.Events.Error.Trigger(errors.Errorf("failed to book message with %s after booking its payload: %w", message.ID(), err))
		}
	})
}

func (b *Booker) bookTransaction(messageID MessageID, tx utxo.Transaction) (success bool) {
	if cachedAttachment, stored := b.tangle.Storage.StoreAttachment(tx.ID(), messageID); stored {
		cachedAttachment.Release()
	}

	if err := b.tangle.Ledger.StoreAndProcessTransaction(MessageIDToContext(context.Background(), messageID), tx); err != nil {
		if !errors.Is(err, ledger.ErrTransactionUnsolid) {
			// TODO: handle invalid transactions (possibly need to attach to invalid event though)
			//  delete attachments of transaction
			b.Events.Error.Trigger(errors.Errorf("failed to book transaction with %s: %w", tx.ID(), err))
		}

		return false
	}

	return true
}

func (b *Booker) processBookedTransaction(id utxo.TransactionID, messageIDToIgnore MessageID) {
	b.tangle.Storage.Attachments(id).Consume(func(attachment *Attachment) {
		event.Loop.Submit(func() {
			b.tangle.Storage.Message(attachment.MessageID()).Consume(func(message *Message) {
				b.tangle.Storage.MessageMetadata(attachment.MessageID()).Consume(func(messageMetadata *MessageMetadata) {
					if attachment.MessageID() == messageIDToIgnore {
						return
					}

					b.payloadBookingMutex.RLock(message.Parents()...)
					b.payloadBookingMutex.Lock(attachment.MessageID())
					defer b.payloadBookingMutex.Unlock(attachment.MessageID())
					defer b.payloadBookingMutex.RUnlock(message.Parents()...)

					if !b.readyForBooking(message, messageMetadata) {
						return
					}

					if err := b.BookMessage(message, messageMetadata); err != nil {
						b.Events.Error.Trigger(errors.Errorf("failed to book message with %s when processing booked transaction %s: %w", attachment.MessageID(), id, err))
					}
				})
			})
		})
	})
}

func (b *Booker) propagateBooking(messageID MessageID) {
	b.tangle.Storage.Approvers(messageID).Consume(func(approver *Approver) {
		event.Loop.Submit(func() {
			b.tangle.Storage.Message(approver.ApproverMessageID()).Consume(func(approvingMessage *Message) {
				b.book(approvingMessage)
			})
		})
	})
}

func (b *Booker) readyForBooking(message *Message, messageMetadata *MessageMetadata) (ready bool) {
	if !messageMetadata.IsSolid() || messageMetadata.IsBooked() {
		return false
	}

	ready = true
	message.ForEachParent(func(parent Parent) {
		b.tangle.Storage.MessageMetadata(parent.ID).Consume(func(messageMetadata *MessageMetadata) {
			ready = ready && messageMetadata.IsBooked()
		})
	})
	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BOOK LOGIC ///////////////////////////////////////////////////////////////////////////////////////////////////

// BookMessage tries to book the given Message (and potentially its contained Transaction) into the ledger and the Tangle.
// It fires a MessageBooked event if it succeeds. If the Message is invalid it fires a MessageInvalid event.
// Booking a message essentially means that parents are examined, the branch of the message determined based on the
// branch inheritance rules of the like switch and markers are inherited. If everything is valid, the message is marked
// as booked. Following, the message branch is set, and it can continue in the dataflow to add support to the determined
// branches and markers.
func (b *Booker) BookMessage(message *Message, messageMetadata *MessageMetadata) (err error) {
	b.bookingMutex.RLock(message.Parents()...)
	defer b.bookingMutex.RUnlock(message.Parents()...)
	b.bookingMutex.Lock(message.ID())
	defer b.bookingMutex.Unlock(message.ID())

	if err = b.inheritBranchIDs(message, messageMetadata); err != nil {
		err = errors.Errorf("failed to inherit BranchIDs of Message with %s: %w", messageMetadata.ID(), err)
		return
	}

	messageMetadata.SetBooked(true)

	b.Events.MessageBooked.Trigger(&MessageBookedEvent{message.ID()})

	return
}

func (b *Booker) inheritBranchIDs(message *Message, messageMetadata *MessageMetadata) (err error) {
	structureDetails, _, inheritedBranchIDs, bookingDetailsErr := b.determineBookingDetails(message)
	if bookingDetailsErr != nil {
		return errors.Errorf("failed to determine booking details of Message with %s: %w", message.ID(), bookingDetailsErr)
	}

	inheritedStructureDetails, newSequenceCreated := b.MarkersManager.InheritStructureDetails(message, structureDetails)
	messageMetadata.SetStructureDetails(inheritedStructureDetails)

	if newSequenceCreated {
		b.MarkersManager.SetBranchIDs(inheritedStructureDetails.PastMarkers().Marker(), inheritedBranchIDs)
		return nil
	}

	// TODO: do not retrieve markers branches once again, determineBookingDetails already does it
	pastMarkersBranchIDs, inheritedStructureDetailsBranchIDsErr := b.branchIDsFromStructureDetails(inheritedStructureDetails)
	if inheritedStructureDetailsBranchIDsErr != nil {
		return errors.Errorf("failed to determine BranchIDs of inherited StructureDetails of Message with %s: %w", message.ID(), inheritedStructureDetailsBranchIDsErr)
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
		messageMetadata.SetAddedBranchIDs(addedBranchIDs)
	}

	if subtractedBranchIDs.Size() != 0 {
		messageMetadata.SetSubtractedBranchIDs(subtractedBranchIDs)
	}

	return nil
}

// determineBookingDetails determines the booking details of an unbooked Message.
func (b *Booker) determineBookingDetails(message *Message) (parentsStructureDetails []*markers.StructureDetails, parentsPastMarkersBranchIDs, inheritedBranchIDs *set.AdvancedSet[utxo.TransactionID], err error) {
	inheritedBranchIDs, err = b.PayloadBranchIDs(message.ID())
	if err != nil {
		return
	}

	parentsStructureDetails, parentsPastMarkersBranchIDs, strongParentsBranchIDs, bookingDetailsErr := b.collectStrongParentsBookingDetails(message)
	if bookingDetailsErr != nil {
		err = errors.Errorf("failed to retrieve booking details of parents of Message with %s: %w", message.ID(), bookingDetailsErr)
		return
	}
	inheritedBranchIDs.AddAll(strongParentsBranchIDs)

	weakPayloadBranchIDs, weakParentsErr := b.collectWeakParentsBranchIDs(message)
	if weakParentsErr != nil {
		return nil, nil, nil, errors.Errorf("failed to collect weak parents of %s: %w", message.ID(), weakParentsErr)
	}
	inheritedBranchIDs.AddAll(weakPayloadBranchIDs)

	dislikedBranchIDs := set.NewAdvancedSet[utxo.TransactionID]()
	likedBranchIDs, dislikedBranchesFromShallowLike, shallowLikeErr := b.collectShallowLikedParentsBranchIDs(message)
	if shallowLikeErr != nil {
		return nil, nil, nil, errors.Errorf("failed to collect shallow likes of %s: %w", message.ID(), shallowLikeErr)
	}
	inheritedBranchIDs.AddAll(likedBranchIDs)
	dislikedBranchIDs.AddAll(dislikedBranchesFromShallowLike)

	dislikedBranchesFromShallowDislike, shallowDislikeErr := b.collectShallowDislikedParentsBranchIDs(message)
	if shallowDislikeErr != nil {
		return nil, nil, nil, errors.Errorf("failed to collect shallow dislikes of %s: %w", message.ID(), shallowDislikeErr)
	}
	dislikedBranchIDs.AddAll(dislikedBranchesFromShallowDislike)

	inheritedBranchIDs.DeleteAll(b.tangle.Ledger.Utils.BranchIDsInFutureCone(dislikedBranchIDs))

	return parentsStructureDetails, parentsPastMarkersBranchIDs, b.tangle.Ledger.ConflictDAG.UnconfirmedConflicts(inheritedBranchIDs), nil
}

// messageBookingDetails returns the Branch and Marker related details of the given Message.
func (b *Booker) messageBookingDetails(messageID MessageID) (structureDetails *markers.StructureDetails, pastMarkersBranchIDs, messageBranchIDs *set.AdvancedSet[utxo.TransactionID], err error) {
	pastMarkersBranchIDs = set.NewAdvancedSet[utxo.TransactionID]()
	messageBranchIDs = set.NewAdvancedSet[utxo.TransactionID]()

	if !b.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		structureDetails = messageMetadata.StructureDetails()
		if pastMarkersBranchIDs, err = b.branchIDsFromStructureDetails(structureDetails); err != nil {
			err = errors.Errorf("failed to retrieve BranchIDs from Structure Details %s of message with %s: %w", structureDetails, messageMetadata.ID(), err)
			return
		}
		messageBranchIDs.AddAll(pastMarkersBranchIDs)

		if addedBranchIDs := messageMetadata.AddedBranchIDs(); !addedBranchIDs.IsEmpty() {
			messageBranchIDs.AddAll(addedBranchIDs)
		}

		if subtractedBranchIDs := messageMetadata.SubtractedBranchIDs(); !subtractedBranchIDs.IsEmpty() {
			messageBranchIDs.DeleteAll(subtractedBranchIDs)
		}
	}) {
		err = errors.Errorf("failed to retrieve MessageMetadata with %s: %w", messageID, cerrors.ErrFatal)
	}

	return structureDetails, pastMarkersBranchIDs, messageBranchIDs, err
}

// branchIDsFromStructureDetails returns the BranchIDs from StructureDetails.
func (b *Booker) branchIDsFromStructureDetails(structureDetails *markers.StructureDetails) (structureDetailsBranchIDs *set.AdvancedSet[utxo.TransactionID], err error) {
	if structureDetails == nil {
		return nil, errors.Errorf("StructureDetails is empty: %w", cerrors.ErrFatal)
	}

	structureDetailsBranchIDs = set.NewAdvancedSet[utxo.TransactionID]()
	// obtain all the Markers
	structureDetails.PastMarkers().ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
		branchIDs := b.MarkersManager.PendingBranchIDs(markers.NewMarker(sequenceID, index))
		structureDetailsBranchIDs.AddAll(branchIDs)
		return true
	})

	return
}

// collectStrongParentsBookingDetails returns the booking details of a Message's strong parents.
func (b *Booker) collectStrongParentsBookingDetails(message *Message) (parentsStructureDetails []*markers.StructureDetails, parentsPastMarkersBranchIDs, parentsBranchIDs *set.AdvancedSet[utxo.TransactionID], err error) {
	parentsStructureDetails = make([]*markers.StructureDetails, 0)
	parentsPastMarkersBranchIDs = set.NewAdvancedSet[utxo.TransactionID]()
	parentsBranchIDs = set.NewAdvancedSet[utxo.TransactionID]()

	message.ForEachParentByType(StrongParentType, func(parentMessageID MessageID) bool {
		parentStructureDetails, parentPastMarkersBranchIDs, parentBranchIDs, parentErr := b.messageBookingDetails(parentMessageID)
		if parentErr != nil {
			err = errors.Errorf("failed to retrieve booking details of Message with %s: %w", parentMessageID, parentErr)
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
func (b *Booker) collectShallowLikedParentsBranchIDs(message *Message) (collectedLikedBranchIDs, collectedDislikedBranchIDs *set.AdvancedSet[utxo.TransactionID], err error) {
	collectedLikedBranchIDs = set.NewAdvancedSet[utxo.TransactionID]()
	collectedDislikedBranchIDs = set.NewAdvancedSet[utxo.TransactionID]()
	message.ForEachParentByType(ShallowLikeParentType, func(parentMessageID MessageID) bool {
		if !b.tangle.Storage.Message(parentMessageID).Consume(func(message *Message) {
			transaction, isTransaction := message.Payload().(utxo.Transaction)
			if !isTransaction {
				err = errors.Errorf("%s referenced by a shallow like of %s does not contain a Transaction: %w", parentMessageID, message.ID(), cerrors.ErrFatal)
				return
			}

			likedBranchIDs, likedBranchesErr := b.tangle.Ledger.Utils.TransactionBranchIDs(transaction.ID())
			if likedBranchesErr != nil {
				err = errors.Errorf("failed to retrieve liked BranchIDs of Transaction with %s contained in %s referenced by a shallow like of %s: %w", transaction.ID(), parentMessageID, message.ID(), likedBranchesErr)
				return
			}
			collectedLikedBranchIDs.AddAll(likedBranchIDs)

			for it := b.tangle.Ledger.Utils.ConflictingTransactions(transaction.ID()).Iterator(); it.HasNext(); {
				conflictingTransactionID := it.Next()
				dislikedBranches, dislikedBranchesErr := b.tangle.Ledger.Utils.TransactionBranchIDs(conflictingTransactionID)
				if dislikedBranchesErr != nil {
					err = errors.Errorf("failed to retrieve disliked BranchIDs of Transaction with %s contained in %s referenced by a shallow like of %s: %w", conflictingTransactionID, parentMessageID, message.ID(), dislikedBranchesErr)
					return
				}
				collectedDislikedBranchIDs.AddAll(dislikedBranches)
			}
		}) {
			err = errors.Errorf("failed to load MessageMetadata of shallow like with %s: %w", parentMessageID, cerrors.ErrFatal)
		}

		return err == nil
	})

	return collectedLikedBranchIDs, collectedDislikedBranchIDs, err
}

// collectShallowDislikedParentsBranchIDs removes the BranchIDs of the shallow dislike reference and all its conflicts from
// the supplied ArithmeticBranchIDs.
func (b *Booker) collectShallowDislikedParentsBranchIDs(message *Message) (collectedDislikedBranchIDs *set.AdvancedSet[utxo.TransactionID], err error) {
	collectedDislikedBranchIDs = set.NewAdvancedSet[utxo.TransactionID]()
	message.ForEachParentByType(DislikeParentType, func(parentMessageID MessageID) bool {
		if !b.tangle.Storage.Message(parentMessageID).Consume(func(message *Message) {
			transaction, isTransaction := message.Payload().(utxo.Transaction)
			if !isTransaction {
				err = errors.Errorf("%s referenced by a shallow like of %s does not contain a Transaction: %w", parentMessageID, message.ID(), cerrors.ErrFatal)
				return
			}

			referenceDislikedBranchIDs, referenceDislikedBranchIDsErr := b.tangle.Ledger.Utils.TransactionBranchIDs(transaction.ID())
			if referenceDislikedBranchIDsErr != nil {
				err = errors.Errorf("failed to retrieve liked BranchIDs of Transaction with %s contained in %s referenced by a shallow like of %s: %w", transaction.ID(), parentMessageID, message.ID(), referenceDislikedBranchIDsErr)
				return
			}
			collectedDislikedBranchIDs.AddAll(referenceDislikedBranchIDs)
		}) {
			err = errors.Errorf("failed to load MessageMetadata of shallow like with %s: %w", parentMessageID, cerrors.ErrFatal)
		}

		return err == nil
	})

	return collectedDislikedBranchIDs, err
}

// collectShallowDislikedParentsBranchIDs removes the BranchIDs of the shallow dislike reference and all its conflicts from
// the supplied ArithmeticBranchIDs.
func (b *Booker) collectWeakParentsBranchIDs(message *Message) (payloadBranchIDs *set.AdvancedSet[utxo.TransactionID], err error) {
	payloadBranchIDs = set.NewAdvancedSet[utxo.TransactionID]()
	message.ForEachParentByType(WeakParentType, func(parentMessageID MessageID) bool {
		if !b.tangle.Storage.Message(parentMessageID).Consume(func(message *Message) {
			transaction, isTransaction := message.Payload().(utxo.Transaction)
			// Payloads other than Transactions are MasterBranch
			if !isTransaction {
				return
			}

			weakReferencePayloadBranch, weakReferenceErr := b.tangle.Ledger.Utils.TransactionBranchIDs(transaction.ID())
			if weakReferenceErr != nil {
				err = errors.Errorf("failed to retrieve BranchIDs of Transaction with %s contained in %s weakly referenced by %s: %w", transaction.ID(), parentMessageID, message.ID(), weakReferenceErr)
				return
			}

			payloadBranchIDs.AddAll(weakReferencePayloadBranch)
		}) {
			err = errors.Errorf("failed to load MessageMetadata of %s weakly referenced by %s: %w", parentMessageID, message.ID(), cerrors.ErrFatal)
		}

		return err == nil
	})

	return payloadBranchIDs, err
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region FORK LOGIC ///////////////////////////////////////////////////////////////////////////////////////////////////

// PropagateForkedBranch propagates the forked BranchID to the future cone of the attachments of the given Transaction.
func (b *Booker) PropagateForkedBranch(transactionID utxo.TransactionID, addedBranchID utxo.TransactionID, removedBranchIDs *set.AdvancedSet[utxo.TransactionID]) (err error) {
	b.tangle.Utils.WalkMessageMetadata(func(messageMetadata *MessageMetadata, messageWalker *walker.Walker[MessageID]) {

		updated, forkErr := b.propagateForkedBranch(messageMetadata, addedBranchID, removedBranchIDs)
		if forkErr != nil {
			messageWalker.StopWalk()
			err = errors.Errorf("failed to propagate forked BranchID %s to future cone of %s: %w", addedBranchID, messageMetadata.ID(), forkErr)
			return
		}
		if !updated {
			return
		}

		b.Events.MessageBranchUpdated.Trigger(&MessageBranchUpdatedEvent{
			MessageID: messageMetadata.ID(),
			BranchID:  addedBranchID,
		})

		for approvingMessageID := range b.tangle.Utils.ApprovingMessageIDs(messageMetadata.ID(), StrongApprover) {
			messageWalker.Push(approvingMessageID)
		}
	}, b.tangle.Storage.AttachmentMessageIDs(transactionID), false)

	return
}

func (b *Booker) propagateForkedBranch(messageMetadata *MessageMetadata, addedBranchID utxo.TransactionID, removedBranchIDs *set.AdvancedSet[utxo.TransactionID]) (propagated bool, err error) {
	b.bookingMutex.Lock(messageMetadata.ID())
	defer b.bookingMutex.Unlock(messageMetadata.ID())

	if !messageMetadata.IsBooked() {
		return false, nil
	}

	if structureDetails := messageMetadata.StructureDetails(); structureDetails.IsPastMarker() {
		if err = b.propagateForkedTransactionToMarkerFutureCone(structureDetails.PastMarkers().Marker(), addedBranchID, removedBranchIDs); err != nil {
			return false, errors.Errorf("failed to propagate conflict%s to future cone of %s: %w", addedBranchID, structureDetails.PastMarkers().Marker(), err)
		}
		return true, nil
	}

	return b.updateMessageBranches(messageMetadata, addedBranchID, removedBranchIDs)
}

func (b *Booker) updateMessageBranches(messageMetadata *MessageMetadata, addedBranch utxo.TransactionID, removedBranches *set.AdvancedSet[utxo.TransactionID]) (updated bool, err error) {
	addedBranchIDs := messageMetadata.AddedBranchIDs()
	if !addedBranchIDs.Add(addedBranch) {
		return false, nil
	}

	pastMarkerBranchIDs, err := b.branchIDsFromStructureDetails(messageMetadata.StructureDetails())
	if err != nil {
		return false, errors.Errorf("failed to retrieve BranchIDs from Structure Details of %s: %w", messageMetadata.ID(), err)
	}
	if pastMarkerBranchIDs.Has(addedBranch) {
		return false, nil
	}

	removedBranchIDsFromPastMarkers := pastMarkerBranchIDs.DeleteAll(removedBranches)
	removedBranchIDsFromAddedBranchIDs := addedBranchIDs.DeleteAll(removedBranches)

	allRemovedBranchIDs := set.NewAdvancedSet[utxo.TransactionID]()
	allRemovedBranchIDs.AddAll(removedBranchIDsFromPastMarkers)
	allRemovedBranchIDs.AddAll(removedBranchIDsFromAddedBranchIDs)
	if !allRemovedBranchIDs.Equal(removedBranches) {
		return false, nil
	}

	subtractedBranchIDs := messageMetadata.SubtractedBranchIDs()
	subtractedBranchIDs.AddAll(removedBranchIDsFromPastMarkers)

	updated = messageMetadata.SetAddedBranchIDs(addedBranchIDs)
	updated = messageMetadata.SetSubtractedBranchIDs(subtractedBranchIDs) || updated

	return updated, nil
}

// propagateForkedTransactionToMarkerFutureCone propagates a newly created BranchID into the future cone of the given Marker.
func (b *Booker) propagateForkedTransactionToMarkerFutureCone(marker markers.Marker, branchID utxo.TransactionID, removedBranchIDs *set.AdvancedSet[utxo.TransactionID]) (err error) {
	markerWalker := walker.New[markers.Marker](false)
	markerWalker.Push(marker)

	for markerWalker.HasNext() {
		currentMarker := markerWalker.Next()

		if err = b.forkSingleMarker(currentMarker, branchID, removedBranchIDs, markerWalker); err != nil {
			err = errors.Errorf("failed to propagate Conflict%s to Messages approving %s: %w", branchID, currentMarker, err)
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

	if !b.MarkersManager.SetBranchIDs(currentMarker, b.tangle.Ledger.ConflictDAG.UnconfirmedConflicts(newBranchIDs)) {
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
