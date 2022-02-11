package tangle

import (
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
)

const bookerQueueSize = 1024

// region Booker ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Booker is a Tangle component that takes care of booking Messages and Transactions by assigning them to the
// corresponding Branch of the ledger state.
type Booker struct {
	// Events is a dictionary for the Booker related Events.
	Events *BookerEvents

	tangle         *Tangle
	MarkersManager *BranchMarkersMapper

	bookerQueue chan MessageID
	shutdown    chan struct{}
	shutdownWG  sync.WaitGroup

	sync.RWMutex
}

// NewBooker is the constructor of a Booker.
func NewBooker(tangle *Tangle) (messageBooker *Booker) {
	messageBooker = &Booker{
		Events: &BookerEvents{
			MessageBooked:        events.NewEvent(MessageIDCaller),
			MarkerBranchAdded:    events.NewEvent(markerBranchUpdatedCaller),
			MessageBranchUpdated: events.NewEvent(messageBranchUpdatedCaller),
			Error:                events.NewEvent(events.ErrorCaller),
		},
		tangle:         tangle,
		MarkersManager: NewBranchMarkersMapper(tangle),
		bookerQueue:    make(chan MessageID, bookerQueueSize),
		shutdown:       make(chan struct{}),
	}

	messageBooker.run()

	return
}

// Setup sets up the behavior of the component by making it attach to the relevant events of other components.
func (b *Booker) Setup() {
	b.tangle.Solidifier.Events.MessageSolid.Attach(events.NewClosure(func(messageID MessageID) {
		b.bookerQueue <- messageID
	}))

	b.tangle.Scheduler.Events.MessageDiscarded.Attach(events.NewClosure(func(messageID MessageID) {
		b.tangle.Storage.Message(messageID).Consume(func(message *Message) {
			nodeID := identity.NewID(message.IssuerPublicKey())
			b.MarkersManager.discardedNodes[nodeID] = time.Now()
		})
	}))

	b.tangle.LedgerState.UTXODAG.Events().TransactionBranchIDUpdatedByFork.Attach(events.NewClosure(func(event *ledgerstate.TransactionBranchIDUpdatedByForkEvent) {
		if err := b.PropagateForkedBranch(event.TransactionID, event.ForkedBranchID); err != nil {
			b.Events.Error.Trigger(errors.Errorf("failed to propagate Branch update of %s to tangle: %w", event.TransactionID, err))
		}
	}))
}

func (b *Booker) run() {
	b.shutdownWG.Add(1)

	go func() {
		defer b.shutdownWG.Done()
		for {
			select {
			case messageID := <-b.bookerQueue:
				if err := b.BookMessage(messageID); err != nil {
					b.Events.Error.Trigger(errors.Errorf("failed to book message with %s: %w", messageID, err))
				}
			case <-b.shutdown:
				// wait until all messages are booked
				if len(b.bookerQueue) == 0 {
					return
				}
			}
		}
	}()
}

// MessageBranchIDs returns the ConflictBranchIDs of the given Message.
func (b *Booker) MessageBranchIDs(messageID MessageID) (branchIDs ledgerstate.BranchIDs, err error) {
	if messageID == EmptyMessageID {
		return ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID), nil
	}

	if _, _, branchIDs, err = b.messageBookingDetails(messageID); err != nil {
		err = errors.Errorf("failed to retrieve booking details of Message with %s: %w", messageID, err)
	}

	if len(branchIDs) == 0 {
		return ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID), nil
	}

	if len(branchIDs) > 1 {
		branchIDs.Subtract(ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID))
	}

	return
}

// PayloadBranchIDs returns the BranchIDs of the payload contained in the given Message.
func (b *Booker) PayloadBranchIDs(messageID MessageID) (branchIDs ledgerstate.BranchIDs, err error) {
	branchIDs = ledgerstate.NewBranchIDs()

	b.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		transaction, isTransaction := message.Payload().(*ledgerstate.Transaction)
		if !isTransaction {
			branchIDs.Add(ledgerstate.MasterBranchID)
			return
		}

		b.tangle.LedgerState.TransactionMetadata(transaction.ID()).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
			resolvedConflictBranchIDs, resolveErr := b.tangle.LedgerState.ResolvePendingConflictBranchIDs(ledgerstate.NewBranchIDs(transactionMetadata.BranchID()))
			if resolveErr != nil {
				err = errors.Errorf("failed to resolve conflict branch ids of transaction with %s: %w", transaction.ID(), resolveErr)
				return
			}
			branchIDs.AddAll(resolvedConflictBranchIDs)
		})
	})

	return
}

// Shutdown shuts down the Booker and persists its state.
func (b *Booker) Shutdown() {
	close(b.shutdown)
	b.shutdownWG.Wait()

	b.MarkersManager.Shutdown()
}

// region BOOK LOGIC ///////////////////////////////////////////////////////////////////////////////////////////////////

// BookMessage tries to book the given Message (and potentially its contained Transaction) into the LedgerState and the Tangle.
// It fires a MessageBooked event if it succeeds. If the Message is invalid it fires a MessageInvalid event.
// Booking a message essentially means that parents are examined, the branch of the message determined based on the
// branch inheritance rules of the like switch and markers are inherited. If everything is valid, the message is marked
// as booked. Following, the message branch is set, and it can continue in the dataflow to add support to the determined
// branches and markers.
func (b *Booker) BookMessage(messageID MessageID) (err error) {
	b.RLock()
	defer b.RUnlock()

	b.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		b.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
			// TODO: we need to enforce that the dislike references contain "the other" branch with respect to the strong references
			// it should be done as part of the solidification refactor, as a payload can only be solid if all its inputs are solid,
			// therefore we would know the payload branch from the solidifier and we could check for this

			// Like and dislike references need to point to Messages containing transactions to evaluate opinion.
			for _, parentType := range []ParentsType{ShallowDislikeParentType, ShallowLikeParentType} {
				if !b.allMessagesContainTransactions(message.ParentsByType(parentType)) {
					messageMetadata.SetObjectivelyInvalid(true)
					err = errors.Errorf("message like or dislike reference does not contain a transaction %s", messageID)
					b.tangle.Events.MessageInvalid.Trigger(&MessageInvalidEvent{MessageID: messageID, Error: err})
					return
				}
			}

			if err = b.inheritBranchIDs(message, messageMetadata); err != nil {
				err = errors.Errorf("failed to inherit BranchIDs of Message with %s: %w", messageID, err)
				return
			}

			messageMetadata.SetBooked(true)

			b.Events.MessageBooked.Trigger(message.ID())
		})
	})

	return
}

func (b *Booker) inheritBranchIDs(message *Message, messageMetadata *MessageMetadata) (err error) {
	structureDetails, _, inheritedBranchIDs, bookingDetailsErr := b.determineBookingDetails(message)
	if bookingDetailsErr != nil {
		return errors.Errorf("failed to determine booking details of Message with %s: %w", message.ID(), bookingDetailsErr)
	}

	aggregatedInheritedBranchID := b.tangle.LedgerState.AggregateConflictBranchesID(inheritedBranchIDs)

	inheritedStructureDetails, newSequenceCreated := b.MarkersManager.InheritStructureDetails(message, structureDetails)
	messageMetadata.SetStructureDetails(inheritedStructureDetails)

	if newSequenceCreated {
		b.MarkersManager.SetBranchID(inheritedStructureDetails.PastMarkers.Marker(), aggregatedInheritedBranchID)
		return nil
	}

	// TODO: do not retrieve markers branches once again, determineBookingDetails already does it
	pastMarkersBranchIDs, inheritedStructureDetailsBranchIDsErr := b.branchIDsFromStructureDetails(inheritedStructureDetails)
	if inheritedStructureDetailsBranchIDsErr != nil {
		return errors.Errorf("failed to determine BranchIDs of inherited StructureDetails of Message with %s: %w", message.ID(), inheritedStructureDetailsBranchIDsErr)
	}

	addedBranchIDs := inheritedBranchIDs.Clone().Subtract(pastMarkersBranchIDs).Subtract(ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID))
	subtractedBranchIDs := pastMarkersBranchIDs.Clone().Subtract(inheritedBranchIDs).Subtract(ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID))

	if len(addedBranchIDs)+len(subtractedBranchIDs) == 0 {
		return nil
	}

	if inheritedStructureDetails.IsPastMarker {
		b.MarkersManager.SetBranchID(inheritedStructureDetails.PastMarkers.Marker(), aggregatedInheritedBranchID)
		return nil
	}

	if len(addedBranchIDs) != 0 {
		if aggregatedAddedBranchIDs := b.tangle.LedgerState.AggregateConflictBranchesID(addedBranchIDs); aggregatedAddedBranchIDs != ledgerstate.MasterBranchID {
			messageMetadata.SetAddedBranchIDs(aggregatedAddedBranchIDs)
		}
	}

	if len(subtractedBranchIDs) != 0 {
		if aggregatedSubtractedBranchIDs := b.tangle.LedgerState.AggregateConflictBranchesID(subtractedBranchIDs); aggregatedSubtractedBranchIDs != ledgerstate.MasterBranchID {
			messageMetadata.SetSubtractedBranchIDs(aggregatedSubtractedBranchIDs)
		}
	}

	return nil
}

// determineBookingDetails determines the booking details of an unbooked Message.
func (b *Booker) determineBookingDetails(message *Message) (parentsStructureDetails []*markers.StructureDetails, parentsPastMarkersBranchIDs, inheritedBranchIDs ledgerstate.BranchIDs, err error) {
	branchIDsOfPayload, bookingErr := b.bookPayload(message)
	if bookingErr != nil {
		return nil, nil, nil, errors.Errorf("failed to book payload of %s: %w", message.ID(), bookingErr)
	}

	parentsStructureDetails, parentsPastMarkersBranchIDs, strongParentsBranchIDs, bookingDetailsErr := b.collectStrongParentsBookingDetails(message)
	if bookingDetailsErr != nil {
		err = errors.Errorf("failed to retrieve booking details of parents of Message with %s: %w", message.ID(), bookingErr)
		return
	}

	arithmeticBranchIDs := ledgerstate.NewArithmeticBranchIDs(strongParentsBranchIDs.AddAll(branchIDsOfPayload))

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

	return parentsStructureDetails, parentsPastMarkersBranchIDs, arithmeticBranchIDs.BranchIDs(), nil
}

// allMessagesContainTransactions checks whether all passed messages contain a transaction.
func (b *Booker) allMessagesContainTransactions(messageIDs MessageIDsSlice) (areAllTransactions bool) {
	areAllTransactions = true
	for _, messageID := range messageIDs {
		b.tangle.Storage.Message(messageID).Consume(func(message *Message) {
			if message.Payload().Type() != ledgerstate.TransactionType {
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
func (b *Booker) messageBookingDetails(messageID MessageID) (structureDetails *markers.StructureDetails, pastMarkersBranchIDs, messageBranchIDs ledgerstate.BranchIDs, err error) {
	pastMarkersBranchIDs = ledgerstate.NewBranchIDs()
	messageBranchIDs = ledgerstate.NewBranchIDs()

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

		if metadataDiffAdd := messageMetadata.AddedBranchIDs(); metadataDiffAdd != ledgerstate.UndefinedBranchID {
			conflictBranchIDs, conflictBranchIDsErr := b.tangle.LedgerState.ResolveConflictBranchIDs(ledgerstate.NewBranchIDs(metadataDiffAdd))
			if conflictBranchIDsErr != nil {
				err = errors.Errorf("failed to resolve DiffAdd branches %s: %w", messageID, conflictBranchIDsErr)
				return
			}

			messageBranchIDs.AddAll(conflictBranchIDs)
		}

		if metadataDiffSubtract := messageMetadata.SubtractedBranchIDs(); metadataDiffSubtract != ledgerstate.UndefinedBranchID {
			conflictBranchIDs, conflictBranchIDsErr := b.tangle.LedgerState.ResolveConflictBranchIDs(ledgerstate.NewBranchIDs(metadataDiffSubtract))
			if conflictBranchIDsErr != nil {
				err = errors.Errorf("failed to resolve DiffSubtract branches %s: %w", messageID, conflictBranchIDsErr)
				return
			}

			messageBranchIDs.Subtract(conflictBranchIDs)
		}
	}) {
		err = errors.Errorf("failed to retrieve MessageMetadata with %s: %w", messageID, cerrors.ErrFatal)
	}

	return structureDetails, pastMarkersBranchIDs, messageBranchIDs, err
}

// branchIDsFromStructureDetails returns the BranchIDs from StructureDetails.
func (b *Booker) branchIDsFromStructureDetails(structureDetails *markers.StructureDetails) (branchIDs ledgerstate.BranchIDs, err error) {
	branchIDs = ledgerstate.NewBranchIDs()
	// obtain all the Markers
	structureDetails.PastMarkers.ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
		conflictBranchIDs, conflictBranchIDsErr := b.MarkersManager.ConflictBranchIDs(markers.NewMarker(sequenceID, index))
		if conflictBranchIDsErr != nil {
			err = errors.Errorf("failed to retrieve ConflictBranchIDs of %s: %w", markers.NewMarker(sequenceID, index), conflictBranchIDsErr)
			return false
		}

		branchIDs.AddAll(conflictBranchIDs)

		return true
	})

	return
}

// collectStrongParentsBookingDetails returns the booking details of a Message's strong parents.
func (b *Booker) collectStrongParentsBookingDetails(message *Message) (parentsStructureDetails []*markers.StructureDetails, parentsPastMarkersBranchIDs, parentsBranchIDs ledgerstate.BranchIDs, err error) {
	parentsStructureDetails = make([]*markers.StructureDetails, 0)
	parentsPastMarkersBranchIDs = ledgerstate.NewBranchIDs()
	parentsBranchIDs = ledgerstate.NewBranchIDs()

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
func (b *Booker) collectShallowLikedParentsBranchIDs(message *Message) (likedBranchIDs, dislikedBranchIDs ledgerstate.BranchIDs, err error) {
	likedBranchIDs = ledgerstate.NewBranchIDs()
	dislikedBranchIDs = ledgerstate.NewBranchIDs()
	message.ForEachParentByType(ShallowLikeParentType, func(parentMessageID MessageID) bool {
		if !b.tangle.Storage.Message(parentMessageID).Consume(func(message *Message) {
			transaction, isTransaction := message.Payload().(*ledgerstate.Transaction)
			if !isTransaction {
				err = errors.Errorf("%s referenced by a shallow like of %s does not contain a Transaction: %w", parentMessageID, message.ID(), cerrors.ErrFatal)
				return
			}

			likedConflictBranchIDs, likedConflictBranchesErr := b.tangle.LedgerState.TransactionBranchIDs(transaction.ID())
			if likedConflictBranchesErr != nil {
				err = errors.Errorf("failed to retrieve liked BranchIDs of Transaction with %s contained in %s referenced by a shallow like of %s: %w", transaction.ID(), parentMessageID, message.ID(), likedConflictBranchesErr)
				return
			}
			likedBranchIDs.AddAll(likedConflictBranchIDs)

			for conflictingTransactionID := range b.tangle.LedgerState.ConflictingTransactions(transaction) {
				dislikedConflictBranches, dislikedConflictBranchesErr := b.tangle.LedgerState.TransactionBranchIDs(conflictingTransactionID)
				if dislikedConflictBranchesErr != nil {
					err = errors.Errorf("failed to retrieve disliked BranchIDs of Transaction with %s contained in %s referenced by a shallow like of %s: %w", conflictingTransactionID, parentMessageID, message.ID(), dislikedConflictBranchesErr)
					return
				}
				dislikedBranchIDs.AddAll(dislikedConflictBranches)
			}
		}) {
			err = errors.Errorf("failed to load MessageMetadata of shallow like with %s: %w", parentMessageID, cerrors.ErrFatal)
		}

		return err == nil
	})

	return likedBranchIDs, dislikedBranchIDs, err
}

// collectShallowDislikedParentsBranchIDs removes the BranchIDs of the shallow dislike reference and all its conflicts from
// the supplied ArithmeticBranchIDs.
func (b *Booker) collectShallowDislikedParentsBranchIDs(message *Message) (dislikedBranchIDs ledgerstate.BranchIDs, err error) {
	dislikedBranchIDs = ledgerstate.NewBranchIDs()
	message.ForEachParentByType(ShallowDislikeParentType, func(parentMessageID MessageID) bool {
		if !b.tangle.Storage.Message(parentMessageID).Consume(func(message *Message) {
			transaction, isTransaction := message.Payload().(*ledgerstate.Transaction)
			if !isTransaction {
				err = errors.Errorf("%s referenced by a shallow like of %s does not contain a Transaction: %w", parentMessageID, message.ID(), cerrors.ErrFatal)
				return
			}

			referenceDislikedBranchIDs, referenceDislikedBranchIDsErr := b.tangle.LedgerState.TransactionBranchIDs(transaction.ID())
			if referenceDislikedBranchIDsErr != nil {
				err = errors.Errorf("failed to retrieve liked BranchIDs of Transaction with %s contained in %s referenced by a shallow like of %s: %w", transaction.ID(), parentMessageID, message.ID(), referenceDislikedBranchIDsErr)
				return
			}
			dislikedBranchIDs.AddAll(referenceDislikedBranchIDs)

			for conflictingTransactionID := range b.tangle.LedgerState.ConflictingTransactions(transaction) {
				dislikedConflictBranches, dislikedConflictBranchesErr := b.tangle.LedgerState.TransactionBranchIDs(conflictingTransactionID)
				if dislikedConflictBranchesErr != nil {
					err = errors.Errorf("failed to retrieve disliked BranchIDs of Transaction with %s contained in %s referenced by a shallow like of %s: %w", conflictingTransactionID, parentMessageID, message.ID(), dislikedConflictBranchesErr)
					return
				}
				dislikedBranchIDs.AddAll(dislikedConflictBranches)
			}
		}) {
			err = errors.Errorf("failed to load MessageMetadata of shallow like with %s: %w", parentMessageID, cerrors.ErrFatal)
		}

		return err == nil
	})

	return dislikedBranchIDs, err
}

// collectShallowDislikedParentsBranchIDs removes the BranchIDs of the shallow dislike reference and all its conflicts from
// the supplied ArithmeticBranchIDs.
func (b *Booker) collectWeakParentsBranchIDs(message *Message) (payloadBranchIDs ledgerstate.BranchIDs, err error) {
	payloadBranchIDs = ledgerstate.NewBranchIDs()
	message.ForEachParentByType(WeakParentType, func(parentMessageID MessageID) bool {
		if !b.tangle.Storage.Message(parentMessageID).Consume(func(message *Message) {
			transaction, isTransaction := message.Payload().(*ledgerstate.Transaction)
			// Payloads other than Transactions are MasterBranch
			if !isTransaction {
				return
			}

			weakReferencePayloadBranch, weakReferenceErr := b.tangle.LedgerState.TransactionBranchIDs(transaction.ID())
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

// bookPayload books the Payload of a Message and returns its assigned BranchID.
func (b *Booker) bookPayload(message *Message) (conflictBranchIDs ledgerstate.BranchIDs, err error) {
	payload := message.Payload()
	if payload == nil || payload.Type() != ledgerstate.TransactionType {
		return ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID), nil
	}

	transaction := payload.(*ledgerstate.Transaction)

	if transactionErr := b.tangle.LedgerState.TransactionValid(transaction, message.ID()); transactionErr != nil {
		return nil, errors.Errorf("invalid transaction in message with %s: %w", message.ID(), transactionErr)
	}

	aggregatedBranchID, err := b.tangle.LedgerState.BookTransaction(transaction, message.ID())
	if err != nil {
		return nil, errors.Errorf("failed to book Transaction of Message with %s: %w", message.ID(), err)
	}

	conflictBranchIDs, err = b.tangle.LedgerState.ResolvePendingConflictBranchIDs(ledgerstate.NewBranchIDs(aggregatedBranchID))
	if err != nil {
		return nil, errors.Errorf("failed to resolve pending ConflictBranches of aggregated %s: %w", aggregatedBranchID, err)
	}

	for _, output := range transaction.Essence().Outputs() {
		b.tangle.LedgerState.UTXODAG.ManageStoreAddressOutputMapping(output)
	}

	if attachment, stored := b.tangle.Storage.StoreAttachment(transaction.ID(), message.ID()); stored {
		attachment.Release()
	}

	return conflictBranchIDs, nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region FORK LOGIC ///////////////////////////////////////////////////////////////////////////////////////////////////

// PropagateForkedBranch propagates the forked BranchID to the future cone of the attachments of the given Transaction.
func (b *Booker) PropagateForkedBranch(transactionID ledgerstate.TransactionID, forkedBranchID ledgerstate.BranchID) (err error) {
	b.tangle.Utils.WalkMessageMetadata(func(messageMetadata *MessageMetadata, messageWalker *walker.Walker) {
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

		if err = b.propagateForkedTransactionToMetadataFutureCone(messageMetadata, forkedBranchID, messageWalker); err != nil {
			err = errors.Errorf("failed to propagate conflict%s to MessageMetadata future cone of %s: %w", forkedBranchID, messageMetadata.ID(), err)
			messageWalker.StopWalk()
			return
		}
	}, b.tangle.Storage.AttachmentMessageIDs(transactionID), false)

	return
}

// propagateForkedTransactionToMarkerFutureCone propagates a newly created BranchID into the future cone of the given Marker.
func (b *Booker) propagateForkedTransactionToMarkerFutureCone(marker *markers.Marker, branchID ledgerstate.BranchID) (err error) {
	markerWalker := walker.New(false)
	markerWalker.Push(marker)

	for markerWalker.HasNext() {
		currentMarker := markerWalker.Next().(*markers.Marker)

		if err = b.forkSingleMarker(currentMarker, branchID, markerWalker); err != nil {
			err = errors.Errorf("failed to propagate Conflict%s to Messages approving %s: %w", branchID, currentMarker, err)
			return
		}
	}

	return
}

// forkSingleMarker propagates a newly created BranchID to a single marker and queues the next elements that need to be
// visited.
func (b *Booker) forkSingleMarker(currentMarker *markers.Marker, newBranchID ledgerstate.BranchID, markerWalker *walker.Walker) (err error) {
	// update BranchID mapping
	oldConflictBranchIDs, err := b.MarkersManager.ConflictBranchIDs(currentMarker)
	if err != nil {
		return errors.Errorf("failed to retrieve ConflictBranchIDs of %s: %w", currentMarker, err)
	}

	_, newBranchIDExists := oldConflictBranchIDs[newBranchID]
	if newBranchIDExists {
		return nil
	}

	if !b.MarkersManager.SetBranchID(currentMarker, b.tangle.LedgerState.AggregateConflictBranchesID(oldConflictBranchIDs.Clone().Add(newBranchID))) {
		return nil
	}

	// trigger event
	b.Events.MarkerBranchAdded.Trigger(currentMarker, oldConflictBranchIDs, newBranchID)

	// propagate updates to later BranchID mappings of the same sequence.
	b.MarkersManager.ForEachBranchIDMapping(currentMarker.SequenceID(), currentMarker.Index(), func(mappedMarker *markers.Marker, _ ledgerstate.BranchID) {
		markerWalker.Push(mappedMarker)
	})

	// propagate updates to referencing markers of later sequences ...
	b.MarkersManager.ForEachMarkerReferencingMarker(currentMarker, func(referencingMarker *markers.Marker) {
		markerWalker.Push(referencingMarker)
	})

	return
}

// propagateForkedTransactionToMetadataFutureCone updates the future cone of a Message to belong to the given conflict BranchID.
func (b *Booker) propagateForkedTransactionToMetadataFutureCone(messageMetadata *MessageMetadata, newConflictBranchID ledgerstate.BranchID, messageWalker *walker.Walker) (err error) {
	branchIDAdded, err := b.addBranchIDToAddedBranchIDs(messageMetadata, newConflictBranchID)
	if err != nil {
		return errors.Errorf("failed to add conflict %s to addedBranchIDs of Message with %s: %w", newConflictBranchID, messageMetadata.ID(), err)
	}

	if !branchIDAdded {
		return nil
	}

	b.Events.MessageBranchUpdated.Trigger(messageMetadata.ID(), newConflictBranchID)

	for _, approvingMessageID := range b.tangle.Utils.ApprovingMessageIDs(messageMetadata.ID(), StrongApprover) {
		messageWalker.Push(approvingMessageID)
	}

	return
}

func (b *Booker) addBranchIDToAddedBranchIDs(messageMetadata *MessageMetadata, newBranchID ledgerstate.BranchID) (added bool, err error) {
	addedBranchIDs, err := b.addedConflictBranchIDs(messageMetadata)
	if err != nil {
		return false, errors.Errorf("failed to retrieve added ConflictBranchIDs from Message with %s: %w", messageMetadata.ID(), err)
	}

	return messageMetadata.SetAddedBranchIDs(b.tangle.LedgerState.AggregateConflictBranchesID(addedBranchIDs.Add(newBranchID))), nil
}

func (b *Booker) addedConflictBranchIDs(messageMetadata *MessageMetadata) (addedConflictBranchIDs ledgerstate.BranchIDs, err error) {
	aggregatedAddedBranchID := messageMetadata.AddedBranchIDs()
	if aggregatedAddedBranchID == ledgerstate.UndefinedBranchID {
		return ledgerstate.NewBranchIDs(), nil
	}

	if addedConflictBranchIDs, err = b.tangle.LedgerState.ResolveConflictBranchIDs(ledgerstate.NewBranchIDs(aggregatedAddedBranchID)); err != nil {
		err = errors.Errorf("failed to resolve conflict BranchIDs of %s: %w", aggregatedAddedBranchID, cerrors.ErrFatal)
	}

	return addedConflictBranchIDs, err
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BookerEvents /////////////////////////////////////////////////////////////////////////////////////////////////

// BookerEvents represents events happening in the Booker.
type BookerEvents struct {
	// MessageBooked is triggered when a Message was booked (it's Branch, and it's Payload's Branch were determined).
	MessageBooked *events.Event

	// MessageBranchUpdated is triggered when the BranchID of a Message is changed in its MessageMetadata.
	MessageBranchUpdated *events.Event

	// MarkerBranchAdded is triggered when a Marker is mapped to a new BranchID.
	MarkerBranchAdded *events.Event

	// Error gets triggered when the Booker faces an unexpected error.
	Error *events.Event
}

func markerBranchUpdatedCaller(handler interface{}, params ...interface{}) {
	handler.(func(marker *markers.Marker, oldBranchID ledgerstate.BranchIDs, newBranchID ledgerstate.BranchID))(params[0].(*markers.Marker), params[1].(ledgerstate.BranchIDs), params[2].(ledgerstate.BranchID))
}

func messageBranchUpdatedCaller(handler interface{}, params ...interface{}) {
	handler.(func(messageID MessageID, newBranchID ledgerstate.BranchID))(params[0].(MessageID), params[1].(ledgerstate.BranchID))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
