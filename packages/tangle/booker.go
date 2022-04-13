package tangle

import (
	"context"
	"fmt"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/walker"
	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"

	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/goshimmer/packages/ledger/branchdag"
	"github.com/iotaledger/goshimmer/packages/markers"
)

// region booker ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Booker is a Tangle component that takes care of booking Messages and Transactions by assigning them to the
// corresponding Branch of the ledger state.
type Booker struct {
	// Events is a dictionary for the Booker related Events.
	Events *BookerEvents

	tangle         *Tangle
	MarkersManager *BranchMarkersMapper
}

// NewBooker is the constructor of a Booker.
func NewBooker(tangle *Tangle) (messageBooker *Booker) {
	messageBooker = &Booker{
		Events:         NewBookerEvents(),
		tangle:         tangle,
		MarkersManager: NewBranchMarkersMapper(tangle),
	}

	return
}

// Setup sets up the behavior of the component by making it attach to the relevant events of other components.
func (b *Booker) Setup() {
	b.tangle.Solidifier.Events.MessageSolid.Attach(events.NewClosure(b.bookPayload))
	b.tangle.Ledger.Events.TransactionBooked.Attach(event.NewClosure(func(event *ledger.TransactionBookedEvent) {
		b.processBookedTransaction(event.TransactionID, MessageIDFromContext(event.Context))
	}))
	b.tangle.Booker.Events.MessageBooked.Attach(event.NewClosure(func(event *MessageBookedEvent) {
		b.propagateBooking(event.MessageID)
	}))

	b.tangle.Scheduler.Events.MessageDiscarded.Attach(event.NewClosure(func(event *MessageDiscardedEvent) {
		b.tangle.Storage.Message(event.MessageID).Consume(func(message *Message) {
			nodeID := identity.NewID(message.IssuerPublicKey())
			b.MarkersManager.discardedNodes[nodeID] = time.Now()
		})
	}))

	// TODO: hook to event TransactionForked
	b.tangle.Ledger.Events.TransactionBranchIDUpdated.Attach(event.NewClosure[*ledger.TransactionBranchIDUpdatedEvent](func(event *ledger.TransactionBranchIDUpdatedEvent) {
		if err := b.PropagateForkedBranch(event.TransactionID, event.AddedBranchID); err != nil {
			b.Events.Error.Trigger(errors.Errorf("failed to propagate Branch update of %s to tangle: %w", event.TransactionID, err))
		}
	}))
}

// MessageBranchIDs returns the BranchIDs of the given Message.
func (b *Booker) MessageBranchIDs(messageID MessageID) (branchIDs branchdag.BranchIDs, err error) {
	if messageID == EmptyMessageID {
		return branchdag.NewBranchIDs(branchdag.MasterBranchID), nil
	}

	if _, _, branchIDs, err = b.messageBookingDetails(messageID); err != nil {
		err = errors.Errorf("failed to retrieve booking details of Message with %s: %w", messageID, err)
	}

	if branchIDs.IsEmpty() {
		return branchdag.NewBranchIDs(branchdag.MasterBranchID), nil
	}

	if !branchIDs.IsEmpty() {
		branchIDs.Delete(branchdag.MasterBranchID)
	}

	return
}

// PayloadBranchIDs returns the BranchIDs of the payload contained in the given Message.
func (b *Booker) PayloadBranchIDs(messageID MessageID) (branchIDs branchdag.BranchIDs, err error) {
	branchIDs = branchdag.NewBranchIDs()

	b.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		transaction, isTransaction := message.Payload().(utxo.Transaction)
		if !isTransaction {
			branchIDs.Add(branchdag.MasterBranchID)
			return
		}

		b.tangle.Ledger.Storage.CachedTransactionMetadata(transaction.ID()).Consume(func(transactionMetadata *ledger.TransactionMetadata) {
			resolvedBranchIDs := b.tangle.Ledger.BranchDAG.FilterPendingBranches(transactionMetadata.BranchIDs())
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

// bookPayload books the Payload of a Message.
func (b *Booker) bookPayload(messageID MessageID) {
	b.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		b.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
			b.tangle.dagMutex.RLock(message.Parents()...)
			b.tangle.dagMutex.Lock(messageID)
			defer b.tangle.dagMutex.Unlock(messageID)
			defer b.tangle.dagMutex.RUnlock(message.Parents()...)

			if !b.readyForBooking(message, messageMetadata) {
				return
			}

			payload := message.Payload()
			tx, ok := payload.(utxo.Transaction)
			if !ok {
				// Data payloads are considered as booked immediately.
				if err := b.BookMessage(message, messageMetadata); err != nil {
					b.Events.Error.Trigger(errors.Errorf("failed to book message with %s after booking its payload: %w", messageID, err))
				}
				return
			}

			b.tangle.Storage.StoreAttachment(tx.ID(), messageID)
			err := b.tangle.Ledger.StoreAndProcessTransaction(MessageIDToContext(context.Background(), messageID), tx)
			if err != nil {
				// TODO: handle invalid transactions (possibly need to attach to invalid event though)
				//  delete attachments of transaction
				fmt.Println(err)
			}

			// TODO: if transaction successfully booked
			if err := b.BookMessage(message, messageMetadata); err != nil {
				b.Events.Error.Trigger(errors.Errorf("failed to book message with %s after booking its payload: %w", messageID, err))
			}
		})
	})
}

func (b *Booker) processBookedTransaction(id utxo.TransactionID, messageIDToIgnore MessageID) {
	b.tangle.Storage.Attachments(id).Consume(func(attachment *Attachment) {
		// TODO: this is now executed sequentially. Is this a problem or should we fire one event for each of the attachments to make it async?
		b.tangle.Storage.Message(attachment.MessageID()).Consume(func(message *Message) {
			b.tangle.Storage.MessageMetadata(attachment.MessageID()).Consume(func(messageMetadata *MessageMetadata) {
				if attachment.MessageID() == messageIDToIgnore {
					return
				}

				b.tangle.dagMutex.RLock(message.Parents()...)
				b.tangle.dagMutex.Lock(attachment.MessageID())
				defer b.tangle.dagMutex.Unlock(attachment.MessageID())
				defer b.tangle.dagMutex.RUnlock(message.Parents()...)

				if !b.readyForBooking(message, messageMetadata) {
					return
				}

				if err := b.BookMessage(message, messageMetadata); err != nil {
					b.Events.Error.Trigger(errors.Errorf("failed to book message with %s when processing booked transaction %s: %w", attachment.MessageID(), id, err))
				}
			})
		})
	})
}

func (b *Booker) propagateBooking(messageID MessageID) {
	// TODO: this should be handled with eventloop
	b.tangle.Storage.Approvers(messageID).Consume(func(approver *Approver) {
		go b.bookPayload(approver.ApproverMessageID())
	})
}

func (b *Booker) readyForBooking(message *Message, metadata *MessageMetadata) (ready bool) {
	if metadata.IsBooked() {
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
	// TODO: we need to enforce that the dislike references contain "the other" branch with respect to the strong references
	// it should be done as part of the solidification refactor, as a payload can only be solid if all its inputs are solid,
	// therefore we would know the payload branch from the solidifier and we could check for this

	// Like and dislike references need to point to Messages containing transactions to evaluate opinion.
	for _, parentType := range []ParentsType{ShallowDislikeParentType, ShallowLikeParentType} {
		if !b.allMessagesContainTransactions(message.ParentsByType(parentType)) {
			messageMetadata.SetObjectivelyInvalid(true)
			err = errors.Errorf("message like or dislike reference does not contain a transaction %s", messageMetadata.ID())
			b.tangle.Events.MessageInvalid.Trigger(&MessageInvalidEvent{MessageID: messageMetadata.ID(), Error: err})
			return
		}
	}

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
		b.MarkersManager.SetBranchIDs(inheritedStructureDetails.PastMarkers.Marker(), inheritedBranchIDs)
		return nil
	}

	// TODO: do not retrieve markers branches once again, determineBookingDetails already does it
	pastMarkersBranchIDs, inheritedStructureDetailsBranchIDsErr := b.branchIDsFromStructureDetails(inheritedStructureDetails)
	if inheritedStructureDetailsBranchIDsErr != nil {
		return errors.Errorf("failed to determine BranchIDs of inherited StructureDetails of Message with %s: %w", message.ID(), inheritedStructureDetailsBranchIDsErr)
	}

	addedBranchIDs := inheritedBranchIDs.Clone()
	addedBranchIDs.DeleteAll(pastMarkersBranchIDs)
	addedBranchIDs.Delete(branchdag.MasterBranchID)

	subtractedBranchIDs := pastMarkersBranchIDs.Clone()
	subtractedBranchIDs.DeleteAll(inheritedBranchIDs)
	subtractedBranchIDs.Delete(branchdag.MasterBranchID)

	if addedBranchIDs.Size()+subtractedBranchIDs.Size() == 0 {
		return nil
	}

	if inheritedStructureDetails.IsPastMarker {
		b.MarkersManager.SetBranchIDs(inheritedStructureDetails.PastMarkers.Marker(), inheritedBranchIDs)
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
func (b *Booker) determineBookingDetails(message *Message) (parentsStructureDetails []*markers.StructureDetails, parentsPastMarkersBranchIDs, inheritedBranchIDs branchdag.BranchIDs, err error) {
	branchIDsOfPayload, err := b.PayloadBranchIDs(message.ID())
	if err != nil {
		return
	}

	parentsStructureDetails, parentsPastMarkersBranchIDs, strongParentsBranchIDs, bookingDetailsErr := b.collectStrongParentsBookingDetails(message)
	if bookingDetailsErr != nil {
		err = errors.Errorf("failed to retrieve booking details of parents of Message with %s: %w", message.ID(), bookingDetailsErr)
		return
	}

	strongParentsBranchIDs.AddAll(branchIDsOfPayload)
	arithmeticBranchIDs := branchdag.NewArithmeticBranchIDs(strongParentsBranchIDs)

	weakPayloadBranchIDs, weakParentsErr := b.collectWeakParentsBranchIDs(message)
	if weakParentsErr != nil {
		return nil, nil, nil, errors.Errorf("failed to collect weak parents of %s: %w", message.ID(), weakParentsErr)
	}
	arithmeticBranchIDs.Add(weakPayloadBranchIDs)

	likedBranchIDs, dislikedBranchIDs, shallowLikeErr := b.collectShallowLikedParentsBranchIDs(message)
	if shallowLikeErr != nil {
		return nil, nil, nil, errors.Errorf("failed to collect shallow likes of %s: %w", message.ID(), shallowLikeErr)
	}
	arithmeticBranchIDs.Add(likedBranchIDs)
	arithmeticBranchIDs.Subtract(dislikedBranchIDs)

	dislikedBranchIDs, shallowDislikeErr := b.collectShallowDislikedParentsBranchIDs(message)
	if shallowDislikeErr != nil {
		return nil, nil, nil, errors.Errorf("failed to collect shallow dislikes of %s: %w", message.ID(), shallowDislikeErr)
	}
	arithmeticBranchIDs.Subtract(dislikedBranchIDs)

	// Make sure that we do not return confirmed branches (aka merge to master).
	inheritedBranchIDs = b.tangle.Ledger.BranchDAG.FilterPendingBranches(arithmeticBranchIDs.BranchIDs())

	return parentsStructureDetails, parentsPastMarkersBranchIDs, inheritedBranchIDs, nil
}

// allMessagesContainTransactions checks whether all passed messages contain a transaction.
func (b *Booker) allMessagesContainTransactions(messageIDs MessageIDs) (areAllTransactions bool) {
	areAllTransactions = true
	for messageID := range messageIDs {
		b.tangle.Storage.Message(messageID).Consume(func(message *Message) {
			if message.Payload().Type() != devnetvm.TransactionType {
				areAllTransactions = false
			}
		})
		if !areAllTransactions {
			return
		}
	}
	return
}

// messageBookingDetails returns the Branch and Marker related details of the given Message.
func (b *Booker) messageBookingDetails(messageID MessageID) (structureDetails *markers.StructureDetails, pastMarkersBranchIDs, messageBranchIDs branchdag.BranchIDs, err error) {
	pastMarkersBranchIDs = branchdag.NewBranchIDs()
	messageBranchIDs = branchdag.NewBranchIDs()

	if !b.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		structureDetails = messageMetadata.StructureDetails()
		if structureDetails == nil {
			err = errors.Errorf("failed to retrieve StructureDetails of Message with %s: %w", messageID, cerrors.ErrFatal)
			return
		}

		structureDetailsBranchIDs, structureDetailsBranchIDsErr := b.branchIDsFromStructureDetails(structureDetails)
		if structureDetailsBranchIDsErr != nil {
			err = errors.Errorf("failed to retrieve BranchIDs from Structure Details %s: %w", structureDetails, structureDetailsBranchIDsErr)
			return
		}
		pastMarkersBranchIDs.AddAll(structureDetailsBranchIDs)
		messageBranchIDs.AddAll(structureDetailsBranchIDs)

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
func (b *Booker) branchIDsFromStructureDetails(structureDetails *markers.StructureDetails) (structureDetailsBranchIDs branchdag.BranchIDs, err error) {
	structureDetailsBranchIDs = branchdag.NewBranchIDs()
	// obtain all the Markers
	structureDetails.PastMarkers.ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
		branchIDs := b.MarkersManager.PendingBranchIDs(markers.NewMarker(sequenceID, index))
		structureDetailsBranchIDs.AddAll(branchIDs)
		return true
	})

	return
}

// collectStrongParentsBookingDetails returns the booking details of a Message's strong parents.
func (b *Booker) collectStrongParentsBookingDetails(message *Message) (parentsStructureDetails []*markers.StructureDetails, parentsPastMarkersBranchIDs, parentsBranchIDs branchdag.BranchIDs, err error) {
	parentsStructureDetails = make([]*markers.StructureDetails, 0)
	parentsPastMarkersBranchIDs = branchdag.NewBranchIDs()
	parentsBranchIDs = branchdag.NewBranchIDs()

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
func (b *Booker) collectShallowLikedParentsBranchIDs(message *Message) (collectedLikedBranchIDs, collectedDislikedBranchIDs branchdag.BranchIDs, err error) {
	collectedLikedBranchIDs = branchdag.NewBranchIDs()
	collectedDislikedBranchIDs = branchdag.NewBranchIDs()
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

			for it := b.tangle.Ledger.Utils.ConflictingTransactions(transaction).Iterator(); it.HasNext(); {
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
func (b *Booker) collectShallowDislikedParentsBranchIDs(message *Message) (collectedDislikedBranchIDs branchdag.BranchIDs, err error) {
	collectedDislikedBranchIDs = branchdag.NewBranchIDs()
	message.ForEachParentByType(ShallowDislikeParentType, func(parentMessageID MessageID) bool {
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

			for it := b.tangle.Ledger.Utils.ConflictingTransactions(transaction).Iterator(); it.HasNext(); {
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

	return collectedDislikedBranchIDs, err
}

// collectShallowDislikedParentsBranchIDs removes the BranchIDs of the shallow dislike reference and all its conflicts from
// the supplied ArithmeticBranchIDs.
func (b *Booker) collectWeakParentsBranchIDs(message *Message) (payloadBranchIDs branchdag.BranchIDs, err error) {
	payloadBranchIDs = branchdag.NewBranchIDs()
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
func (b *Booker) PropagateForkedBranch(transactionID utxo.TransactionID, forkedBranchID branchdag.BranchID) (err error) {
	b.tangle.Utils.WalkMessageMetadata(func(messageMetadata *MessageMetadata, messageWalker *walker.Walker[MessageID]) {
		if !messageMetadata.IsBooked() {
			return
		}

		if structureDetails := messageMetadata.StructureDetails(); structureDetails.IsPastMarker {
			if err = b.propagateForkedTransactionToMarkerFutureCone(structureDetails.PastMarkers.Marker(), forkedBranchID); err != nil {
				err = errors.Errorf("failed to propagate conflict%s to future cone of %s: %w", forkedBranchID, structureDetails.PastMarkers.Marker(), err)
				messageWalker.StopWalk()
			}
			return
		}

		if !messageMetadata.AddBranchID(forkedBranchID) {
			return
		}

		b.Events.MessageBranchUpdated.Trigger(&MessageBranchUpdatedEvent{
			MessageID: messageMetadata.ID(),
			BranchID:  forkedBranchID,
		})

		for approvingMessageID := range b.tangle.Utils.ApprovingMessageIDs(messageMetadata.ID(), StrongApprover) {
			messageWalker.Push(approvingMessageID)
		}
	}, b.tangle.Storage.AttachmentMessageIDs(transactionID), false)

	return
}

// propagateForkedTransactionToMarkerFutureCone propagates a newly created BranchID into the future cone of the given Marker.
func (b *Booker) propagateForkedTransactionToMarkerFutureCone(marker *markers.Marker, branchID branchdag.BranchID) (err error) {
	markerWalker := walker.New[*markers.Marker](false)
	markerWalker.Push(marker)

	for markerWalker.HasNext() {
		currentMarker := markerWalker.Next()

		if err = b.forkSingleMarker(currentMarker, branchID, markerWalker); err != nil {
			err = errors.Errorf("failed to propagate Conflict%s to Messages approving %s: %w", branchID, currentMarker, err)
			return
		}
	}

	return
}

// forkSingleMarker propagates a newly created BranchID to a single marker and queues the next elements that need to be
// visited.
func (b *Booker) forkSingleMarker(currentMarker *markers.Marker, newBranchID branchdag.BranchID, markerWalker *walker.Walker[*markers.Marker]) (err error) {
	// update BranchID mapping
	oldBranchIDs := b.MarkersManager.PendingBranchIDs(currentMarker)

	if oldBranchIDs.Has(newBranchID) {
		return nil
	}

	newBranchIDs := oldBranchIDs.Clone()
	newBranchIDs.Add(newBranchID)
	if !b.MarkersManager.SetBranchIDs(currentMarker, newBranchIDs) {
		return nil
	}

	// trigger event
	b.Events.MarkerBranchAdded.Trigger(&MarkerBranchAddedEvent{
		Marker:       currentMarker,
		OldBranchIDs: oldBranchIDs,
		NewBranchID:  newBranchID,
	})

	// propagate updates to later BranchID mappings of the same sequence.
	b.MarkersManager.ForEachBranchIDMapping(currentMarker.SequenceID(), currentMarker.Index(), func(mappedMarker *markers.Marker, _ branchdag.BranchIDs) {
		markerWalker.Push(mappedMarker)
	})

	// propagate updates to referencing markers of later sequences ...
	b.MarkersManager.ForEachMarkerReferencingMarker(currentMarker, func(referencingMarker *markers.Marker) {
		markerWalker.Push(referencingMarker)
	})

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
