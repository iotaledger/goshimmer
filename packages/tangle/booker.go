package tangle

import (
	"fmt"
	"sort"
	"strconv"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/datastructure/thresholdmap"
	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/debuglogger"
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
	MarkersManager *MarkersManager

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
		MarkersManager: NewMarkersManager(tangle),
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

	b.tangle.LedgerState.UTXODAG.Events().TransactionBranchIDUpdatedByFork.Attach(events.NewClosure(func(event *ledgerstate.TransactionBranchIDUpdatedByForkEvent) {
		if err := b.PropagateForkedBranch(event.TransactionID, event.ForkedBranchID, debuglogger.New("PropagateForkedBranch")); err != nil {
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
	debugLogger := debuglogger.New("BookMessage")

	debugLogger.MethodStart("Booker", "BookMessage", messageID)
	defer debugLogger.MethodEnd()

	b.RLock()
	defer b.RUnlock()

	b.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		b.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
			// TODO: we need to enforce that the dislike references contain "the other" branch with respect to the strong references
			// it should be done as part of the solidification refactor, as a payload can only be solid if all its inputs are solid,
			// therefore we would know the payload branch from the solidifier and we could check for this

			if b.isAnyParentObjectivelyInvalid(message) {
				messageMetadata.SetObjectivelyInvalid(true)
				err = errors.Errorf("failed to book message %s: referencing objectively invalid parent", messageID)
				b.tangle.Events.MessageInvalid.Trigger(&MessageInvalidEvent{MessageID: messageID, Error: err})
				return
			}

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
	structureDetails, pastMarkersBranchIDs, inheritedBranchIDs, bookingDetailsErr := b.determineBookingDetails(message)
	if bookingDetailsErr != nil {
		return errors.Errorf("failed to determine booking details of Message with %s: %w", message.ID(), bookingDetailsErr)
	}

	aggregatedInheritedBranchID := b.tangle.LedgerState.AggregateConflictBranchesID(inheritedBranchIDs)
	addedBranchIDs := inheritedBranchIDs.Clone().Subtract(pastMarkersBranchIDs).Subtract(ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID))
	subtractedBranchIDs := pastMarkersBranchIDs.Clone().Subtract(inheritedBranchIDs).Subtract(ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID))

	inheritedStructureDetails, newSequenceCreated := b.MarkersManager.InheritStructureDetails(message, structureDetails, markers.NewSequenceAlias(aggregatedInheritedBranchID.Bytes()))
	messageMetadata.SetStructureDetails(inheritedStructureDetails)

	switch b.branchMappingTarget(newSequenceCreated, len(addedBranchIDs)+len(subtractedBranchIDs) == 0, inheritedStructureDetails.IsPastMarker) {
	case MarkerMappingTarget:
		b.MarkersManager.SetBranchID(inheritedStructureDetails.PastMarkers.Marker(), aggregatedInheritedBranchID)
	case MetadataMappingTarget:
		if len(addedBranchIDs) != 0 {
			messageMetadata.SetAddedBranchIDs(b.tangle.LedgerState.AggregateConflictBranchesID(addedBranchIDs))
		}

		if len(subtractedBranchIDs) != 0 {
			messageMetadata.SetSubtractedBranchIDs(b.tangle.LedgerState.AggregateConflictBranchesID(subtractedBranchIDs))
		}
	}

	return nil
}

func (b *Booker) branchMappingTarget(newSequenceCreated bool, diffsEmpty bool, isPastMarker bool) (mappingTarget MappingTarget) {
	if newSequenceCreated {
		return MarkerMappingTarget
	}

	if diffsEmpty {
		return UndefinedMappingTarget
	}

	if isPastMarker {
		return MarkerMappingTarget
	}

	return MetadataMappingTarget
}

// determineBookingDetails determines the booking details of an unbooked Message.
func (b *Booker) determineBookingDetails(message *Message) (parentsStructureDetails []*markers.StructureDetails, parentsPastMarkersBranchIDs, inheritedBranchIDs ledgerstate.BranchIDs, err error) {
	branchIDsOfPayload, bookingErr := b.bookPayload(message)
	if bookingErr != nil {
		return nil, nil, nil, errors.Errorf("failed to book payload of %s: %w", message.ID(), bookingErr)
	}

	parentsStructureDetails, parentsPastMarkersBranchIDs, parentsBranchIDs, bookingDetailsErr := b.collectStrongParentsBookingDetails(message)
	if bookingDetailsErr != nil {
		err = errors.Errorf("failed to retrieve booking details of parents of Message with %s: %w", message.ID(), bookingErr)
		return
	}

	arithmeticBranchIDs := NewArithmeticBranchIDs(parentsBranchIDs.AddAll(branchIDsOfPayload))
	if weakParentsErr := b.collectWeakParentsBranchIDs(message, arithmeticBranchIDs); weakParentsErr != nil {
		return nil, nil, nil, errors.Errorf("failed to collect weak parents of %s: %w", message.ID(), weakParentsErr)
	}
	if shallowLikeErr := b.collectShallowLikedParentsBranchIDs(message, arithmeticBranchIDs); shallowLikeErr != nil {
		return nil, nil, nil, errors.Errorf("failed to collect shallow likes of %s: %w", message.ID(), shallowLikeErr)
	}
	if shallowDislikeErr := b.collectShallowDislikedParentsBranchIDs(message, arithmeticBranchIDs); shallowDislikeErr != nil {
		return nil, nil, nil, errors.Errorf("failed to collect shallow dislikes of %s: %w", message.ID(), shallowDislikeErr)
	}

	return parentsStructureDetails, parentsPastMarkersBranchIDs, arithmeticBranchIDs.BranchIDs(), nil
}

func (b *Booker) isAnyParentObjectivelyInvalid(message *Message) (isAnyParentObjectivelyInvalid bool) {
	isAnyParentObjectivelyInvalid = false
	message.ForEachParent(func(parent Parent) {
		if isAnyParentObjectivelyInvalid {
			return
		}
		b.tangle.Storage.MessageMetadata(parent.ID).Consume(func(messageMetadata *MessageMetadata) {
			isAnyParentObjectivelyInvalid = messageMetadata.IsObjectivelyInvalid()
		})
	})
	return
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

		// obtain all the Markers
		structureDetails.PastMarkers.ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
			conflictBranchIDs, conflictBranchIDsErr := b.MarkersManager.ConflictBranchIDs(markers.NewMarker(sequenceID, index))
			if conflictBranchIDsErr != nil {
				err = errors.Errorf("failed to retrieve ConflictBranchIDs of %s: %w", markers.NewMarker(sequenceID, index), conflictBranchIDsErr)
				return false
			}

			pastMarkersBranchIDs.AddAll(conflictBranchIDs)
			messageBranchIDs.AddAll(conflictBranchIDs)

			return true
		})

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

// strongParentsBranchIDs returns the branches of the Message's strong parents.
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
func (b *Booker) collectShallowLikedParentsBranchIDs(message *Message, arithmeticBranchIDs ArithmeticBranchIDs) (err error) {
	message.ForEachParentByType(ShallowLikeParentType, func(parentMessageID MessageID) bool {
		if !b.tangle.Storage.Message(parentMessageID).Consume(func(message *Message) {
			transaction, isTransaction := message.Payload().(*ledgerstate.Transaction)
			if !isTransaction {
				err = errors.Errorf("%s referenced by a shallow like of %s does not contain a Transaction: %w", parentMessageID, message.ID(), cerrors.ErrFatal)
				return
			}

			likedConflictBranches, likedConflictBranchesErr := b.tangle.LedgerState.TransactionBranchIDs(transaction.ID())
			if likedConflictBranchesErr != nil {
				err = errors.Errorf("failed to retrieve liked BranchIDs of Transaction with %s contained in %s referenced by a shallow like of %s: %w", transaction.ID(), parentMessageID, message.ID(), likedConflictBranchesErr)
				return
			}
			arithmeticBranchIDs.Add(likedConflictBranches)

			dislikedBranchIDs := ledgerstate.NewBranchIDs()
			for conflictingTransactionID := range b.tangle.LedgerState.ConflictingTransactions(transaction) {
				dislikedConflictBranches, dislikedConflictBranchesErr := b.tangle.LedgerState.TransactionBranchIDs(conflictingTransactionID)
				if dislikedConflictBranchesErr != nil {
					err = errors.Errorf("failed to retrieve disliked BranchIDs of Transaction with %s contained in %s referenced by a shallow like of %s: %w", conflictingTransactionID, parentMessageID, message.ID(), dislikedConflictBranchesErr)
					return
				}
				dislikedBranchIDs.AddAll(dislikedConflictBranches)
			}
			arithmeticBranchIDs.Subtract(dislikedBranchIDs)
		}) {
			err = errors.Errorf("failed to load MessageMetadata of shallow like with %s: %w", parentMessageID, cerrors.ErrFatal)
		}

		return err == nil
	})

	return err
}

// collectShallowDislikedParentsBranchIDs removes the BranchIDs of the shallow dislike reference and all its conflicts from
// the supplied ArithmeticBranchIDs.
func (b *Booker) collectShallowDislikedParentsBranchIDs(message *Message, arithmeticBranchIDs ArithmeticBranchIDs) (err error) {
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

			dislikedBranchIDs := ledgerstate.NewBranchIDs(referenceDislikedBranchIDs.Slice()...)
			for conflictingTransactionID := range b.tangle.LedgerState.ConflictingTransactions(transaction) {
				dislikedConflictBranches, dislikedConflictBranchesErr := b.tangle.LedgerState.TransactionBranchIDs(conflictingTransactionID)
				if dislikedConflictBranchesErr != nil {
					err = errors.Errorf("failed to retrieve disliked BranchIDs of Transaction with %s contained in %s referenced by a shallow like of %s: %w", conflictingTransactionID, parentMessageID, message.ID(), dislikedConflictBranchesErr)
					return
				}
				dislikedBranchIDs.AddAll(dislikedConflictBranches)
			}
			arithmeticBranchIDs.Subtract(dislikedBranchIDs)
		}) {
			err = errors.Errorf("failed to load MessageMetadata of shallow like with %s: %w", parentMessageID, cerrors.ErrFatal)
		}

		return err == nil
	})

	return err
}

// collectShallowDislikedParentsBranchIDs removes the BranchIDs of the shallow dislike reference and all its conflicts from
// the supplied ArithmeticBranchIDs.
func (b *Booker) collectWeakParentsBranchIDs(message *Message, arithmeticBranchIDs ArithmeticBranchIDs) (err error) {
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

			arithmeticBranchIDs.Add(weakReferencePayloadBranch)
		}) {
			err = errors.Errorf("failed to load MessageMetadata of %s weakly referenced by %s: %w", parentMessageID, message.ID(), cerrors.ErrFatal)
		}

		return err == nil
	})

	return err
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
func (b *Booker) PropagateForkedBranch(transactionID ledgerstate.TransactionID, forkedBranchID ledgerstate.BranchID, debugLogger *debuglogger.DebugLogger) (err error) {
	defer debugLogger.MethodStart("Booker", "PropagateForkedBranch", transactionID, forkedBranchID).MethodEnd()

	b.tangle.Utils.WalkMessageMetadata(func(messageMetadata *MessageMetadata, messageWalker *walker.Walker) {
		defer debugLogger.MethodStart("Utils", "WalkMessageMetadata", messageMetadata.ID()).MethodEnd()

		if !messageMetadata.IsBooked() {
			debugLogger.Println("return // Message is not booked")
			return
		}

		if structureDetails := messageMetadata.StructureDetails(); structureDetails.IsPastMarker {
			if err = b.propagateForkedTransactionToMarkerFutureCone(structureDetails.PastMarkers.Marker(), forkedBranchID, debugLogger); err != nil {
				err = errors.Errorf("failed to propagate conflict%s to future cone of %s: %w", forkedBranchID, structureDetails.PastMarkers.Marker(), err)
				messageWalker.StopWalk()
			}
			return
		}

		if err = b.propagateForkedTransactionToMetadataFutureCone(messageMetadata, forkedBranchID, messageWalker, debugLogger); err != nil {
			err = errors.Errorf("failed to propagate conflict%s to MessageMetadata future cone of %s: %w", forkedBranchID, messageMetadata.ID(), err)
			messageWalker.StopWalk()
			return
		}
	}, b.tangle.Storage.AttachmentMessageIDs(transactionID), false)

	return
}

// propagateForkedTransactionToMarkerFutureCone propagates a newly created BranchID into the future cone of the given Marker.
func (b *Booker) propagateForkedTransactionToMarkerFutureCone(marker *markers.Marker, branchID ledgerstate.BranchID, debugLogger *debuglogger.DebugLogger) (err error) {
	debugLogger.MethodStart("Booker", "propagateForkedTransactionToMarkerFutureCone", marker, branchID)
	defer debugLogger.MethodEnd()

	markerWalker := walker.New(false)
	markerWalker.Push(marker)

	for markerWalker.HasNext() {
		currentMarker := markerWalker.Next().(*markers.Marker)

		if err = b.forkSingleMarker(currentMarker, branchID, markerWalker, debugLogger); err != nil {
			err = errors.Errorf("failed to propagate Conflict%s to Messages approving %s: %w", branchID, currentMarker, err)
			return
		}
	}

	return
}

// forkSingleMarker propagates a newly created BranchID to a single marker and queues the next elements that need to be
// visited.
func (b *Booker) forkSingleMarker(currentMarker *markers.Marker, newBranchID ledgerstate.BranchID, markerWalker *walker.Walker, debugLogger *debuglogger.DebugLogger) (err error) {
	defer debugLogger.MethodStart("Booker", "forkSingleMarker", currentMarker, newBranchID).MethodEnd()

	// update BranchID mapping
	oldConflictBranchIDs, err := b.MarkersManager.ConflictBranchIDs(currentMarker)
	if err != nil {
		debugLogger.Println("return // error occurred")
		return errors.Errorf("failed to retrieve ConflictBranchIDs of %s: %w", currentMarker, err)
	}

	_, newBranchIDExists := oldConflictBranchIDs[newBranchID]
	if newBranchIDExists {
		debugLogger.Println("return // BranchID not updated")
		return nil
	}

	if !b.MarkersManager.SetBranchID(currentMarker, b.tangle.LedgerState.AggregateConflictBranchesID(oldConflictBranchIDs.Clone().Add(newBranchID))) {
		debugLogger.Println("return // BranchID not updated")
		return nil
	}

	// trigger event
	b.Events.MarkerBranchAdded.Trigger(currentMarker, oldConflictBranchIDs, newBranchID)

	// propagate updates to later BranchID mappings of the same sequence.
	b.MarkersManager.ForEachBranchIDMapping(currentMarker.SequenceID(), currentMarker.Index(), func(mappedMarker *markers.Marker, _ ledgerstate.BranchID) {
		debugLogger.Println("markerWalker.Push(", mappedMarker, ") // later mapping of same sequence")
		markerWalker.Push(mappedMarker)
	})

	// propagate updates to referencing markers of later sequences ...
	b.MarkersManager.ForEachMarkerReferencingMarker(currentMarker, func(referencingMarker *markers.Marker) {
		debugLogger.Println("markerWalker.Push(", referencingMarker, ") // referencing marker of later sequence")
		markerWalker.Push(referencingMarker)
	})

	return
}

// propagateForkedTransactionToMetadataFutureCone updates the future cone of a Message to belong to the given conflict BranchID.
func (b *Booker) propagateForkedTransactionToMetadataFutureCone(messageMetadata *MessageMetadata, newConflictBranchID ledgerstate.BranchID, messageWalker *walker.Walker, debugLogger *debuglogger.DebugLogger) (err error) {
	defer debugLogger.MethodStart("Booker", "propagateForkedTransactionToMetadataFutureCone", messageMetadata.ID(), newConflictBranchID).MethodEnd()

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

// region MarkersManager ///////////////////////////////////////////////////////////////////////////////////////////////

// MarkersManager is a Tangle component that takes care of managing the Markers which are used to infer structural
// information about the Tangle in an efficient way.
type MarkersManager struct {
	tangle *Tangle

	*markers.Manager
}

// NewMarkersManager is the constructor of the MarkersManager.
func NewMarkersManager(tangle *Tangle) *MarkersManager {
	return &MarkersManager{
		tangle:  tangle,
		Manager: markers.NewManager(tangle.Options.Store, tangle.Options.CacheTimeProvider),
	}
}

// InheritStructureDetails returns the structure Details of a Message that are derived from the StructureDetails of its
// strong and like parents.
func (m *MarkersManager) InheritStructureDetails(message *Message, structureDetails []*markers.StructureDetails, sequenceAlias markers.SequenceAlias) (newStructureDetails *markers.StructureDetails, newSequenceCreated bool) {
	newStructureDetails, newSequenceCreated = m.Manager.InheritStructureDetails(structureDetails, m.tangle.Options.IncreaseMarkersIndexCallback, sequenceAlias)
	if newStructureDetails.IsPastMarker {
		m.SetMessageID(newStructureDetails.PastMarkers.Marker(), message.ID())
		m.tangle.Utils.WalkMessageMetadata(m.propagatePastMarkerToFutureMarkers(newStructureDetails.PastMarkers.Marker()), message.ParentsByType(StrongParentType))
	}

	return
}

// MessageID retrieves the MessageID of the given Marker.
func (m *MarkersManager) MessageID(marker *markers.Marker) (messageID MessageID) {
	m.tangle.Storage.MarkerMessageMapping(marker).Consume(func(markerMessageMapping *MarkerMessageMapping) {
		messageID = markerMessageMapping.MessageID()
	})

	return
}

// SetMessageID associates a MessageID with the given Marker.
func (m *MarkersManager) SetMessageID(marker *markers.Marker, messageID MessageID) {
	m.tangle.Storage.StoreMarkerMessageMapping(NewMarkerMessageMapping(marker, messageID))
}

// BranchID returns the BranchID that is associated with the given Marker.
func (m *MarkersManager) BranchID(marker *markers.Marker) (branchID ledgerstate.BranchID) {
	if marker.SequenceID() == 0 {
		return ledgerstate.MasterBranchID
	}

	m.tangle.Storage.MarkerIndexBranchIDMapping(marker.SequenceID()).Consume(func(markerIndexBranchIDMapping *MarkerIndexBranchIDMapping) {
		branchID = markerIndexBranchIDMapping.BranchID(marker.Index())
	})

	return
}

// ConflictBranchIDs returns the ConflictBranchIDs that are associated with the given Marker.
func (m *MarkersManager) ConflictBranchIDs(marker *markers.Marker) (branchIDs ledgerstate.BranchIDs, err error) {
	if branchIDs, err = m.tangle.LedgerState.ResolvePendingConflictBranchIDs(ledgerstate.NewBranchIDs(m.BranchID(marker))); err != nil {
		err = errors.Errorf("failed to resolve ConflictBranchIDs of marker %s: %w", marker, err)
	}
	return
}

// SetBranchID associates a BranchID with the given Marker.
func (m *MarkersManager) SetBranchID(marker *markers.Marker, branchID ledgerstate.BranchID) (updated bool) {
	if floorMarker, floorBranchID, exists := m.Floor(marker); exists {
		if floorBranchID == branchID {
			return false
		}

		if floorMarker == marker.Index() {
			m.UnregisterSequenceAliasMapping(markers.NewSequenceAlias(floorBranchID.Bytes()), marker.SequenceID())

			m.deleteBranchIDMapping(markers.NewMarker(marker.SequenceID(), floorMarker))
		}

		m.registerSequenceAliasMappingIfLastMappedMarker(marker, branchID)
	}

	m.setBranchIDMapping(marker, branchID)

	return true
}

func (m *MarkersManager) setBranchIDMapping(marker *markers.Marker, branchID ledgerstate.BranchID) bool {
	return m.tangle.Storage.MarkerIndexBranchIDMapping(marker.SequenceID(), NewMarkerIndexBranchIDMapping).Consume(func(markerIndexBranchIDMapping *MarkerIndexBranchIDMapping) {
		markerIndexBranchIDMapping.SetBranchID(marker.Index(), branchID)
	})
}

func (m *MarkersManager) deleteBranchIDMapping(marker *markers.Marker) bool {
	return m.tangle.Storage.MarkerIndexBranchIDMapping(marker.SequenceID(), NewMarkerIndexBranchIDMapping).Consume(func(markerIndexBranchIDMapping *MarkerIndexBranchIDMapping) {
		markerIndexBranchIDMapping.DeleteBranchID(marker.Index())
	})
}

func (m *MarkersManager) registerSequenceAliasMappingIfLastMappedMarker(marker *markers.Marker, branchID ledgerstate.BranchID) {
	if _, _, exists := m.Ceiling(markers.NewMarker(marker.SequenceID(), marker.Index()+1)); !exists {
		m.RegisterSequenceAliasMapping(markers.NewSequenceAlias(branchID.Bytes()), marker.SequenceID())
	}
}

// BranchMappedByPastMarkers returns true if the given BranchID is associated to at least one of the given past Markers.
func (m *MarkersManager) BranchMappedByPastMarkers(branch ledgerstate.BranchID, pastMarkers *markers.Markers) (branchMappedByPastMarkers bool) {
	pastMarkers.ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
		branchMappedByPastMarkers = m.BranchID(markers.NewMarker(sequenceID, index)) == branch

		return !branchMappedByPastMarkers
	})

	return
}

// Floor returns the largest Index that is <= the given Marker, it's BranchID and a boolean value indicating if it
// exists.
func (m *MarkersManager) Floor(referenceMarker *markers.Marker) (marker markers.Index, branchID ledgerstate.BranchID, exists bool) {
	m.tangle.Storage.MarkerIndexBranchIDMapping(referenceMarker.SequenceID(), NewMarkerIndexBranchIDMapping).Consume(func(markerIndexBranchIDMapping *MarkerIndexBranchIDMapping) {
		marker, branchID, exists = markerIndexBranchIDMapping.Floor(referenceMarker.Index())
	})

	return
}

// Ceiling returns the smallest Index that is >= the given Marker, it's BranchID and a boolean value indicating if it
// exists.
func (m *MarkersManager) Ceiling(referenceMarker *markers.Marker) (marker markers.Index, branchID ledgerstate.BranchID, exists bool) {
	m.tangle.Storage.MarkerIndexBranchIDMapping(referenceMarker.SequenceID(), NewMarkerIndexBranchIDMapping).Consume(func(markerIndexBranchIDMapping *MarkerIndexBranchIDMapping) {
		marker, branchID, exists = markerIndexBranchIDMapping.Ceiling(referenceMarker.Index())
	})

	return
}

// ForEachMessageApprovingMarker iterates through all Messages that strongly approve the given Marker.
func (m *MarkersManager) ForEachMessageApprovingMarker(marker *markers.Marker, callback func(approvingMessageID MessageID)) {
	for _, approvingMessageID := range m.tangle.Utils.ApprovingMessageIDs(m.MessageID(marker), StrongApprover) {
		callback(approvingMessageID)
	}
}

// ForEachBranchIDMapping iterates over all BranchID mappings in the given Sequence that are bigger than the given
// thresholdIndex. Setting the thresholdIndex to 0 will iterate over all existing mappings.
func (m *MarkersManager) ForEachBranchIDMapping(sequenceID markers.SequenceID, thresholdIndex markers.Index, callback func(mappedMarker *markers.Marker, mappedBranchID ledgerstate.BranchID)) {
	currentMarker := markers.NewMarker(sequenceID, thresholdIndex)
	referencingMarkerIndexInSameSequence, mappedBranchID, exists := m.Ceiling(markers.NewMarker(currentMarker.SequenceID(), currentMarker.Index()+1))
	for ; exists; referencingMarkerIndexInSameSequence, mappedBranchID, exists = m.Ceiling(markers.NewMarker(currentMarker.SequenceID(), currentMarker.Index()+1)) {
		currentMarker = markers.NewMarker(currentMarker.SequenceID(), referencingMarkerIndexInSameSequence)
		callback(currentMarker, mappedBranchID)
	}
}

// ForEachMarkerReferencingMarker executes the callback function for each Marker of other Sequences that directly
// reference the given Marker.
func (m *MarkersManager) ForEachMarkerReferencingMarker(referencedMarker *markers.Marker, callback func(referencingMarker *markers.Marker)) {
	m.Sequence(referencedMarker.SequenceID()).Consume(func(sequence *markers.Sequence) {
		sequence.ReferencingMarkers(referencedMarker.Index()).ForEachSorted(func(referencingSequenceID markers.SequenceID, referencingIndex markers.Index) bool {
			if referencingSequenceID == referencedMarker.SequenceID() {
				return true
			}

			callback(markers.NewMarker(referencingSequenceID, referencingIndex))

			return true
		})
	})
}

// propagatePastMarkerToFutureMarkers updates the FutureMarkers of the strong parents of a given message when a new
// PastMaster was assigned.
func (m *MarkersManager) propagatePastMarkerToFutureMarkers(pastMarkerToInherit *markers.Marker) func(messageMetadata *MessageMetadata, walker *walker.Walker) {
	return func(messageMetadata *MessageMetadata, walker *walker.Walker) {
		updated, inheritFurther := m.UpdateStructureDetails(messageMetadata.StructureDetails(), pastMarkerToInherit)
		if updated {
			messageMetadata.SetModified(true)
		}
		if inheritFurther {
			m.tangle.Storage.Message(messageMetadata.ID()).Consume(func(message *Message) {
				for _, strongParentMessageID := range message.ParentsByType(StrongParentType) {
					walker.Push(strongParentMessageID)
				}
			})
		}
	}
}

// increaseMarkersIndexCallbackStrategy implements the default strategy for increasing marker Indexes in the Tangle.
func increaseMarkersIndexCallbackStrategy(markers.SequenceID, markers.Index) bool {
	return true
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MarkerIndexBranchIDMapping ///////////////////////////////////////////////////////////////////////////////////

// MarkerIndexBranchIDMapping is a data structure that allows to map marker Indexes to a BranchID.
type MarkerIndexBranchIDMapping struct {
	sequenceID   markers.SequenceID
	mapping      *thresholdmap.ThresholdMap
	mappingMutex sync.RWMutex

	objectstorage.StorableObjectFlags
}

// NewMarkerIndexBranchIDMapping creates a new MarkerIndexBranchIDMapping for the given SequenceID.
func NewMarkerIndexBranchIDMapping(sequenceID markers.SequenceID) (markerBranchMapping *MarkerIndexBranchIDMapping) {
	markerBranchMapping = &MarkerIndexBranchIDMapping{
		sequenceID: sequenceID,
		mapping:    thresholdmap.New(thresholdmap.LowerThresholdMode, markerIndexComparator),
	}

	markerBranchMapping.SetModified()
	markerBranchMapping.Persist()

	return
}

// MarkerIndexBranchIDMappingFromBytes unmarshals a MarkerIndexBranchIDMapping from a sequence of bytes.
func MarkerIndexBranchIDMappingFromBytes(bytes []byte) (markerIndexBranchIDMapping *MarkerIndexBranchIDMapping, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if markerIndexBranchIDMapping, err = MarkerIndexBranchIDMappingFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse MarkerIndexBranchIDMapping from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// MarkerIndexBranchIDMappingFromMarshalUtil unmarshals a MarkerIndexBranchIDMapping using a MarshalUtil (for easier
// unmarshalling).
func MarkerIndexBranchIDMappingFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (markerIndexBranchIDMapping *MarkerIndexBranchIDMapping, err error) {
	markerIndexBranchIDMapping = &MarkerIndexBranchIDMapping{}
	if markerIndexBranchIDMapping.sequenceID, err = markers.SequenceIDFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse SequenceID from MarshalUtil: %w", err)
		return
	}
	mappingCount, mappingCountErr := marshalUtil.ReadUint64()
	if mappingCountErr != nil {
		err = errors.Errorf("failed to parse reference count (%v): %w", mappingCountErr, cerrors.ErrParseBytesFailed)
		return
	}
	markerIndexBranchIDMapping.mapping = thresholdmap.New(thresholdmap.LowerThresholdMode, markerIndexComparator)
	for j := uint64(0); j < mappingCount; j++ {
		index, indexErr := marshalUtil.ReadUint64()
		if indexErr != nil {
			err = errors.Errorf("failed to parse Index (%v): %w", indexErr, cerrors.ErrParseBytesFailed)
			return
		}

		branchID, branchIDErr := ledgerstate.BranchIDFromMarshalUtil(marshalUtil)
		if branchIDErr != nil {
			err = errors.Errorf("failed to parse BranchID: %w", branchIDErr)
			return
		}

		markerIndexBranchIDMapping.mapping.Set(markers.Index(index), branchID)
	}

	return
}

// MarkerIndexBranchIDMappingFromObjectStorage restores a MarkerIndexBranchIDMapping that was stored in the object
// storage.
func MarkerIndexBranchIDMappingFromObjectStorage(key []byte, data []byte) (markerIndexBranchIDMapping objectstorage.StorableObject, err error) {
	if markerIndexBranchIDMapping, _, err = MarkerIndexBranchIDMappingFromBytes(byteutils.ConcatBytes(key, data)); err != nil {
		err = errors.Errorf("failed to parse MarkerIndexBranchIDMapping from bytes: %w", err)
		return
	}

	return
}

// SequenceID returns the SequenceID that this MarkerIndexBranchIDMapping represents.
func (m *MarkerIndexBranchIDMapping) SequenceID() markers.SequenceID {
	return m.sequenceID
}

// BranchID returns the BranchID that is associated to the given marker Index.
func (m *MarkerIndexBranchIDMapping) BranchID(markerIndex markers.Index) (branchID ledgerstate.BranchID) {
	m.mappingMutex.RLock()
	defer m.mappingMutex.RUnlock()

	value, exists := m.mapping.Get(markerIndex)
	if !exists {
		panic(fmt.Sprintf("tried to retrieve the BranchID of unknown marker.%s", markerIndex))
	}

	return value.(ledgerstate.BranchID)
}

// SetBranchID creates a mapping between the given marker Index and the given BranchID.
func (m *MarkerIndexBranchIDMapping) SetBranchID(index markers.Index, branchID ledgerstate.BranchID) {
	m.mappingMutex.Lock()
	defer m.mappingMutex.Unlock()

	m.mapping.Set(index, branchID)
	m.SetModified()
}

// DeleteBranchID deletes a mapping between the given marker Index and the stored BranchID.
func (m *MarkerIndexBranchIDMapping) DeleteBranchID(index markers.Index) {
	m.mappingMutex.Lock()
	defer m.mappingMutex.Unlock()

	m.mapping.Delete(index)
	m.SetModified()
}

// Floor returns the largest Index that is <= the given Index which has a mapped BranchID (and a boolean value
// indicating if it exists).
func (m *MarkerIndexBranchIDMapping) Floor(index markers.Index) (marker markers.Index, branchID ledgerstate.BranchID, exists bool) {
	m.mappingMutex.RLock()
	defer m.mappingMutex.RUnlock()

	if untypedIndex, untypedBranchID, exists := m.mapping.Floor(index); exists {
		return untypedIndex.(markers.Index), untypedBranchID.(ledgerstate.BranchID), true
	}

	return 0, ledgerstate.UndefinedBranchID, false
}

// Ceiling returns the smallest Index that is >= the given Index which has a mapped BranchID (and a boolean value
// indicating if it exists).
func (m *MarkerIndexBranchIDMapping) Ceiling(index markers.Index) (marker markers.Index, branchID ledgerstate.BranchID, exists bool) {
	m.mappingMutex.RLock()
	defer m.mappingMutex.RUnlock()

	if untypedIndex, untypedBranchID, exists := m.mapping.Ceiling(index); exists {
		return untypedIndex.(markers.Index), untypedBranchID.(ledgerstate.BranchID), true
	}

	return 0, ledgerstate.UndefinedBranchID, false
}

// Bytes returns a marshaled version of the MarkerIndexBranchIDMapping.
func (m *MarkerIndexBranchIDMapping) Bytes() []byte {
	return byteutils.ConcatBytes(m.ObjectStorageKey(), m.ObjectStorageValue())
}

// String returns a human-readable version of the MarkerIndexBranchIDMapping.
func (m *MarkerIndexBranchIDMapping) String() string {
	m.mappingMutex.RLock()
	defer m.mappingMutex.RUnlock()

	indexes := make([]markers.Index, 0)
	branchIDs := make(map[markers.Index]ledgerstate.BranchID)
	m.mapping.ForEach(func(node *thresholdmap.Element) bool {
		index := node.Key().(markers.Index)
		indexes = append(indexes, index)
		branchIDs[index] = node.Value().(ledgerstate.BranchID)

		return true
	})

	sort.Slice(indexes, func(i, j int) bool {
		return indexes[i] < indexes[j]
	})

	mapping := stringify.StructBuilder("Mapping")
	for i, referencingIndex := range indexes {
		thresholdStart := strconv.FormatUint(uint64(referencingIndex), 10)
		thresholdEnd := "INF"
		if len(indexes) > i+1 {
			thresholdEnd = strconv.FormatUint(uint64(indexes[i+1])-1, 10)
		}

		if thresholdStart == thresholdEnd {
			mapping.AddField(stringify.StructField("Index("+thresholdStart+")", branchIDs[referencingIndex]))
		} else {
			mapping.AddField(stringify.StructField("Index("+thresholdStart+" ... "+thresholdEnd+")", branchIDs[referencingIndex]))
		}
	}

	return stringify.Struct("MarkerIndexBranchIDMapping",
		stringify.StructField("sequenceID", m.sequenceID),
		stringify.StructField("mapping", mapping),
	)
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (m *MarkerIndexBranchIDMapping) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (m *MarkerIndexBranchIDMapping) ObjectStorageKey() []byte {
	return m.sequenceID.Bytes()
}

// ObjectStorageValue marshals the ConflictBranch into a sequence of bytes that are used as the value part in the
// object storage.
func (m *MarkerIndexBranchIDMapping) ObjectStorageValue() []byte {
	m.mappingMutex.RLock()
	defer m.mappingMutex.RUnlock()

	marshalUtil := marshalutil.New()
	marshalUtil.WriteUint64(uint64(m.mapping.Size()))
	m.mapping.ForEach(func(node *thresholdmap.Element) bool {
		marshalUtil.Write(node.Key().(markers.Index))
		marshalUtil.Write(node.Value().(ledgerstate.BranchID))

		return true
	})

	return marshalUtil.Bytes()
}

// markerIndexComparator is a comparator for marker Indexes.
func markerIndexComparator(a, b interface{}) int {
	aCasted := a.(markers.Index)
	bCasted := b.(markers.Index)

	switch {
	case aCasted < bCasted:
		return -1
	case aCasted > bCasted:
		return 1
	default:
		return 0
	}
}

// code contract (make sure the type implements all required methods)
var _ objectstorage.StorableObject = &MarkerIndexBranchIDMapping{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedMarkerIndexBranchIDMapping /////////////////////////////////////////////////////////////////////////////

// CachedMarkerIndexBranchIDMapping is a wrapper for the generic CachedObject returned by the object storage that
// overrides the accessor methods with a type-casted one.
type CachedMarkerIndexBranchIDMapping struct {
	objectstorage.CachedObject
}

// Retain marks the CachedObject to still be in use by the program.
func (c *CachedMarkerIndexBranchIDMapping) Retain() *CachedMarkerIndexBranchIDMapping {
	return &CachedMarkerIndexBranchIDMapping{c.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (c *CachedMarkerIndexBranchIDMapping) Unwrap() *MarkerIndexBranchIDMapping {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*MarkerIndexBranchIDMapping)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (c *CachedMarkerIndexBranchIDMapping) Consume(consumer func(markerIndexBranchIDMapping *MarkerIndexBranchIDMapping), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*MarkerIndexBranchIDMapping))
	}, forceRelease...)
}

// String returns a human-readable version of the CachedMarkerIndexBranchIDMapping.
func (c *CachedMarkerIndexBranchIDMapping) String() string {
	return stringify.Struct("CachedMarkerIndexBranchIDMapping",
		stringify.StructField("CachedObject", c.Unwrap()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MarkerMessageMapping /////////////////////////////////////////////////////////////////////////////////////////

// MarkerMessageMappingPartitionKeys defines the "layout" of the key. This enables prefix iterations in the object
// storage.
var MarkerMessageMappingPartitionKeys = objectstorage.PartitionKey(markers.SequenceIDLength, markers.IndexLength)

// MarkerMessageMapping is a data structure that denotes a mapping from a Marker to a Message.
type MarkerMessageMapping struct {
	marker    *markers.Marker
	messageID MessageID

	objectstorage.StorableObjectFlags
}

// NewMarkerMessageMapping is the constructor for the MarkerMessageMapping.
func NewMarkerMessageMapping(marker *markers.Marker, messageID MessageID) *MarkerMessageMapping {
	return &MarkerMessageMapping{
		marker:    marker,
		messageID: messageID,
	}
}

// MarkerMessageMappingFromBytes unmarshals an MarkerMessageMapping from a sequence of bytes.
func MarkerMessageMappingFromBytes(bytes []byte) (individuallyMappedMessage *MarkerMessageMapping, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if individuallyMappedMessage, err = MarkerMessageMappingFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse MarkerMessageMapping from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// MarkerMessageMappingFromMarshalUtil unmarshals an MarkerMessageMapping using a MarshalUtil (for easier unmarshalling).
func MarkerMessageMappingFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (markerMessageMapping *MarkerMessageMapping, err error) {
	markerMessageMapping = &MarkerMessageMapping{}
	if markerMessageMapping.marker, err = markers.MarkerFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse Marker from MarshalUtil: %w", err)
		return
	}
	if markerMessageMapping.messageID, err = ReferenceFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse MessageID from MarshalUtil: %w", err)
		return
	}

	return
}

// MarkerMessageMappingFromObjectStorage is a factory method that creates a new MarkerMessageMapping instance
// from a storage key of the object storage. It is used by the object storage, to create new instances of this entity.
func MarkerMessageMappingFromObjectStorage(key, value []byte) (result objectstorage.StorableObject, err error) {
	if result, _, err = MarkerMessageMappingFromBytes(byteutils.ConcatBytes(key, value)); err != nil {
		err = errors.Errorf("failed to parse MarkerMessageMapping from bytes: %w", err)
		return
	}

	return
}

// Marker returns the Marker that is mapped to a MessageID.
func (m *MarkerMessageMapping) Marker() *markers.Marker {
	return m.marker
}

// MessageID returns the MessageID of the Marker.
func (m *MarkerMessageMapping) MessageID() MessageID {
	return m.messageID
}

// Bytes returns a marshaled version of the MarkerMessageMapping.
func (m *MarkerMessageMapping) Bytes() []byte {
	return byteutils.ConcatBytes(m.ObjectStorageKey(), m.ObjectStorageValue())
}

// String returns a human-readable version of the MarkerMessageMapping.
func (m *MarkerMessageMapping) String() string {
	return stringify.Struct("MarkerMessageMapping",
		stringify.StructField("marker", m.marker),
		stringify.StructField("messageID", m.messageID),
	)
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (m *MarkerMessageMapping) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (m *MarkerMessageMapping) ObjectStorageKey() []byte {
	return m.marker.Bytes()
}

// ObjectStorageValue marshals the MarkerMessageMapping into a sequence of bytes that are used as the value part in
// the object storage.
func (m *MarkerMessageMapping) ObjectStorageValue() []byte {
	return m.messageID.Bytes()
}

// code contract (make sure the type implements all required methods).
var _ objectstorage.StorableObject = &MarkerMessageMapping{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedMarkerMessageMapping ///////////////////////////////////////////////////////////////////////////////////

// CachedMarkerMessageMapping is a wrapper for the generic CachedObject returned by the object storage that overrides
// the accessor methods with a type-casted one.
type CachedMarkerMessageMapping struct {
	objectstorage.CachedObject
}

// Retain marks the CachedObject to still be in use by the program.
func (c *CachedMarkerMessageMapping) Retain() *CachedMarkerMessageMapping {
	return &CachedMarkerMessageMapping{c.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (c *CachedMarkerMessageMapping) Unwrap() *MarkerMessageMapping {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*MarkerMessageMapping)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (c *CachedMarkerMessageMapping) Consume(consumer func(markerMessageMapping *MarkerMessageMapping), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*MarkerMessageMapping))
	}, forceRelease...)
}

// String returns a human-readable version of the CachedMarkerMessageMapping.
func (c *CachedMarkerMessageMapping) String() string {
	return stringify.Struct("CachedMarkerMessageMapping",
		stringify.StructField("CachedObject", c.Unwrap()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedMarkerMessageMappings //////////////////////////////////////////////////////////////////////////////////

// CachedMarkerMessageMappings defines a slice of *CachedMarkerMessageMapping.
type CachedMarkerMessageMappings []*CachedMarkerMessageMapping

// Unwrap is the type-casted equivalent of Get. It returns a slice of unwrapped objects with the object being nil if it
// does not exist.
func (c CachedMarkerMessageMappings) Unwrap() (unwrappedMarkerMessageMappings []*MarkerMessageMapping) {
	unwrappedMarkerMessageMappings = make([]*MarkerMessageMapping, len(c))
	for i, cachedMarkerMessageMapping := range c {
		untypedObject := cachedMarkerMessageMapping.Get()
		if untypedObject == nil {
			continue
		}

		typedObject := untypedObject.(*MarkerMessageMapping)
		if typedObject == nil || typedObject.IsDeleted() {
			continue
		}

		unwrappedMarkerMessageMappings[i] = typedObject
	}

	return
}

// Consume iterates over the CachedObjects, unwraps them and passes a type-casted version to the consumer (if the object
// is not empty - it exists). It automatically releases the object when the consumer finishes. It returns true, if at
// least one object was consumed.
func (c CachedMarkerMessageMappings) Consume(consumer func(markerMessageMapping *MarkerMessageMapping), forceRelease ...bool) (consumed bool) {
	for _, cachedMarkerMessageMapping := range c {
		consumed = cachedMarkerMessageMapping.Consume(consumer, forceRelease...) || consumed
	}

	return
}

// Release is a utility function that allows us to release all CachedObjects in the collection.
func (c CachedMarkerMessageMappings) Release(force ...bool) {
	for _, cachedMarkerMessageMapping := range c {
		cachedMarkerMessageMapping.Release(force...)
	}
}

// String returns a human-readable version of the CachedMarkerMessageMappings.
func (c CachedMarkerMessageMappings) String() string {
	structBuilder := stringify.StructBuilder("CachedMarkerMessageMappings")
	for i, cachedMarkerMessageMapping := range c {
		structBuilder.AddField(stringify.StructField(strconv.Itoa(i), cachedMarkerMessageMapping))
	}

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ArithmeticBranchIDs //////////////////////////////////////////////////////////////////////////////////////////

// ArithmeticBranchIDs represents an arithmetic collection of BranchIDs that allows us to add and subtract them from
// each other.
type ArithmeticBranchIDs map[ledgerstate.BranchID]int

// NewArithmeticBranchIDs returns a new ArithmeticBranchIDs object.
func NewArithmeticBranchIDs(optionalBranchIDs ...ledgerstate.BranchIDs) (newArithmeticBranchIDs ArithmeticBranchIDs) {
	newArithmeticBranchIDs = make(ArithmeticBranchIDs)
	if len(optionalBranchIDs) >= 1 {
		newArithmeticBranchIDs.Add(optionalBranchIDs[0])
	}

	return newArithmeticBranchIDs
}

// Add adds all BranchIDs to the collection.
func (a ArithmeticBranchIDs) Add(branchIDs ledgerstate.BranchIDs) {
	for branchID := range branchIDs {
		a[branchID]++
	}
}

// Subtract subtracts all BranchIDs from the collection.
func (a ArithmeticBranchIDs) Subtract(branchIDs ledgerstate.BranchIDs) {
	for branchID := range branchIDs {
		a[branchID]--
	}
}

// BranchIDs returns the BranchIDs represented by this collection.
func (a ArithmeticBranchIDs) BranchIDs() (branchIDs ledgerstate.BranchIDs) {
	branchIDs = ledgerstate.NewBranchIDs()
	for branchID, value := range a {
		if value >= 1 {
			branchIDs.Add(branchID)
		}
	}

	return
}

// String returns a human-readable version of the ArithmeticBranchIDs.
func (a ArithmeticBranchIDs) String() string {
	if len(a) == 0 {
		return "ArithmeticBranchIDs() = " + a.BranchIDs().String()
	}

	result := "ArithmeticBranchIDs("
	i := 0
	for branchID, value := range a {
		switch true {
		case value == 1:
			if i != 0 {
				result += " + "
			}

			result += branchID.String()
			i++
		case value > 1:
			if i != 0 {
				result += " + "
			}

			result += strconv.Itoa(value) + "*" + branchID.String()
			i++
		case value == 0:
		case value == -1:
			if i != 0 {
				result += " - "
			} else {
				result += "-"
			}

			result += branchID.String()
			i++
		case value < -1:
			if i != 0 {
				result += " - "
			}

			result += strconv.Itoa(-value) + "*" + branchID.String()
			i++
		}
	}
	result += ") = " + a.BranchIDs().String()

	return result
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MappingTarget ////////////////////////////////////////////////////////////////////////////////////////////////

type MappingTarget uint8

const (
	// UndefinedMappingTarget is the zero-value of the MappingTarget type.
	UndefinedMappingTarget MappingTarget = iota

	// MarkerMappingTarget represents the MarkerMappingTarget that indicates that the Branch should be mapped through
	// the Markers.
	MarkerMappingTarget

	// MetadataMappingTarget represents the MarkerMappingTarget that indicates that the Branch should be mapped through
	// the MessageMetadata.
	MetadataMappingTarget
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
