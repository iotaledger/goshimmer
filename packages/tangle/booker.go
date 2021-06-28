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
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
)

// region Booker ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Booker is a Tangle component that takes care of booking Messages and Transactions by assigning them to the
// corresponding Branch of the ledger state.
type Booker struct {
	// Events is a dictionary for the Booker related Events.
	Events *BookerEvents

	tangle         *Tangle
	MarkersManager *MarkersManager
}

// NewBooker is the constructor of a Booker.
func NewBooker(tangle *Tangle) (messageBooker *Booker) {
	messageBooker = &Booker{
		Events: &BookerEvents{
			MessageBooked:        events.NewEvent(MessageIDCaller),
			MarkerBranchUpdated:  events.NewEvent(markerBranchUpdatedCaller),
			MessageBranchUpdated: events.NewEvent(messageBranchUpdatedCaller),
			Error:                events.NewEvent(events.ErrorCaller),
		},
		tangle:         tangle,
		MarkersManager: NewMarkersManager(tangle),
	}

	return
}

// Setup sets up the behavior of the component by making it attach to the relevant events of other components.
func (b *Booker) Setup() {
	b.tangle.Orderer.Events.MessageOrdered.Attach(events.NewClosure(func(messageID MessageID) {
		if err := b.BookMessage(messageID); err != nil {
			b.Events.Error.Trigger(errors.Errorf("failed to book message with %s: %w", messageID, err))
		}
	}))

	b.tangle.LedgerState.UTXODAG.Events.TransactionBranchIDUpdated.Attach(events.NewClosure(func(transactionID ledgerstate.TransactionID) {
		if err := b.BookConflictingTransaction(transactionID); err != nil {
			b.Events.Error.Trigger(errors.Errorf("failed to propagate ConflictBranch of %s to tangle: %w", transactionID, err))
		}
	}))
}

// BookMessage tries to book the given Message (and potentially its contained Transaction) into the LedgerState and the Tangle.
// It fires a MessageBooked event if it succeeds.
func (b *Booker) BookMessage(messageID MessageID) (err error) {
	b.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		b.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
			// don't book the same message more than once!
			if messageMetadata.IsBooked() {
				err = errors.Errorf("message already booked %s", messageID)
				return
			}

			branchIDOfPayload, bookingErr := b.branchIDOfPayload(message)
			if bookingErr != nil {
				err = errors.Errorf("failed to book payload of %s: %w", messageID, bookingErr)
				return
			}

			inheritedBranch, inheritErr := b.tangle.LedgerState.InheritBranch(b.parentsBranchIDs(message).Add(branchIDOfPayload))
			if inheritErr != nil {
				err = errors.Errorf("failed to inherit Branch when booking Message with %s: %w", message.ID(), inheritErr)
				return
			}

			inheritedStructureDetails := b.MarkersManager.InheritStructureDetails(message, markers.NewSequenceAlias(inheritedBranch.Bytes()))
			messageMetadata.SetStructureDetails(inheritedStructureDetails)

			if inheritedStructureDetails.PastMarkers.Size() != 1 || !b.MarkersManager.BranchMappedByPastMarkers(inheritedBranch, inheritedStructureDetails.PastMarkers) {
				if !inheritedStructureDetails.IsPastMarker {
					messageMetadata.SetBranchID(inheritedBranch)
					b.tangle.Storage.StoreIndividuallyMappedMessage(NewIndividuallyMappedMessage(inheritedBranch, message.ID(), inheritedStructureDetails.PastMarkers))
				} else {
					b.MarkersManager.SetBranchID(inheritedStructureDetails.PastMarkers.Marker(), inheritedBranch)
				}
			}

			messageMetadata.SetBooked(true)

			b.Events.MessageBooked.Trigger(message.ID())
		})
	})

	return
}

// BookConflictingTransaction propagates new conflicts.
func (b *Booker) BookConflictingTransaction(transactionID ledgerstate.TransactionID) (err error) {
	conflictBranchID := b.tangle.LedgerState.BranchID(transactionID)

	b.tangle.Utils.WalkMessageMetadata(func(messageMetadata *MessageMetadata, walker *walker.Walker) {
		if !messageMetadata.IsBooked() {
			return
		}

		if structureDetails := messageMetadata.StructureDetails(); structureDetails.IsPastMarker {
			if err = b.updateMarkerFutureCone(structureDetails.PastMarkers.Marker(), conflictBranchID); err != nil {
				err = errors.Errorf("failed to propagate conflict%s to future cone of %s: %w", conflictBranchID, structureDetails.PastMarkers.Marker(), err)
				walker.StopWalk()
			}

			return
		}

		if err = b.updateMetadataFutureCone(messageMetadata, conflictBranchID, walker); err != nil {
			err = errors.Errorf("failed to propagate conflict%s to MessageMetadata future cone of %s: %w", conflictBranchID, messageMetadata.ID(), err)
			walker.StopWalk()
			return
		}
	}, b.tangle.Storage.AttachmentMessageIDs(transactionID))

	return
}

// MessageBranchID returns the BranchID of the given Message.
func (b *Booker) MessageBranchID(messageID MessageID) (branchID ledgerstate.BranchID, err error) {
	if messageID == EmptyMessageID {
		return ledgerstate.MasterBranchID, nil
	}

	if !b.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
		if branchID = messageMetadata.BranchID(); branchID != ledgerstate.UndefinedBranchID {
			return
		}

		structureDetails := messageMetadata.StructureDetails()
		if structureDetails == nil {
			err = errors.Errorf("failed to retrieve StructureDetails of %s: %w", messageID, cerrors.ErrFatal)
			return
		}
		if structureDetails.PastMarkers.Size() != 1 {
			err = errors.Errorf("BranchID of %s should have been mapped in the MessageMetadata (multiple PastMarkers): %w", messageID, cerrors.ErrFatal)
			return
		}

		branchID = b.MarkersManager.BranchID(structureDetails.PastMarkers.Marker())
	}) {
		err = errors.Errorf("failed to load MessageMetadata of %s: %w", messageID, cerrors.ErrFatal)
		return
	}

	return
}

// Shutdown shuts down the Booker and persists its state.
func (b *Booker) Shutdown() {
	b.MarkersManager.Shutdown()
}

// parentsBranchIDs returns the BranchIDs of a Message's parents.
func (b *Booker) parentsBranchIDs(message *Message) (branchIDs ledgerstate.BranchIDs) {
	branchIDs = make(ledgerstate.BranchIDs)

	message.ForEachStrongParent(func(messageID MessageID) {
		if messageID == EmptyMessageID {
			branchIDs[ledgerstate.MasterBranchID] = types.Void
			return
		}

		if !b.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
			if branchID := messageMetadata.BranchID(); branchID != ledgerstate.UndefinedBranchID {
				branchIDs[branchID] = types.Void
				return
			}

			structureDetailsOfMessage := messageMetadata.StructureDetails()
			if structureDetailsOfMessage == nil {
				panic(fmt.Errorf("tried to retrieve BranchID from unbooked Message with %s: %v", messageID, cerrors.ErrFatal))
			}
			if structureDetailsOfMessage.PastMarkers.Size() > 1 {
				panic(fmt.Errorf("tried to retrieve BranchID from Message with multiple past markers - %s: %v", messageID, cerrors.ErrFatal))
			}

			branchIDs[b.MarkersManager.BranchID(structureDetailsOfMessage.PastMarkers.Marker())] = types.Void
		}) {
			panic(fmt.Errorf("failed to load MessageMetadata with %s", messageID))
		}
	})

	message.ForEachWeakParent(func(parentMessageID MessageID) {
		if parentMessageID == EmptyMessageID {
			return
		}

		if !b.tangle.Storage.Message(parentMessageID).Consume(func(message *Message) {
			if payload := message.Payload(); payload != nil && payload.Type() == ledgerstate.TransactionType {
				transactionID := payload.(*ledgerstate.Transaction).ID()

				if !b.tangle.LedgerState.UTXODAG.CachedTransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
					branchIDs[transactionMetadata.BranchID()] = types.Void
				}) {
					panic(fmt.Errorf("failed to load TransactionMetadata with %s", transactionID))
				}
			}
		}) {
			panic(fmt.Errorf("failed to load MessageMetadata with %s", parentMessageID))
		}
	})

	return branchIDs
}

// branchIDOfPayload returns the BranchID of the payload of the given Message.
func (b *Booker) branchIDOfPayload(message *Message) (branchID ledgerstate.BranchID, err error) {
	payload := message.Payload()
	if payload == nil || payload.Type() != ledgerstate.TransactionType {
		return ledgerstate.MasterBranchID, nil
	}

	transaction := payload.(*ledgerstate.Transaction)
	if !b.tangle.LedgerState.UTXODAG.CachedTransactionMetadata(transaction.ID()).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
		branchID = transactionMetadata.BranchID()
	}) {
		err = errors.Errorf("failed to load TransactionMetadata of %s: %w", transaction.ID(), cerrors.ErrFatal)
	}

	return
}

// updatedBranchID returns the BranchID that is the result of aggregating the passed in BranchIDs.
func (b *Booker) updatedBranchID(branchID, conflictBranchID ledgerstate.BranchID) (newBranchID ledgerstate.BranchID, branchIDUpdated bool, err error) {
	if branchID == conflictBranchID {
		return branchID, false, nil
	}

	if newBranchID, err = b.tangle.LedgerState.InheritBranch(ledgerstate.NewBranchIDs(branchID, conflictBranchID)); err != nil {
		return ledgerstate.UndefinedBranchID, false, errors.Errorf("failed to combine %s and %s into a new BranchID: %w", branchID, conflictBranchID, cerrors.ErrFatal)
	}

	if newBranchID == branchID {
		return branchID, false, nil
	}

	return newBranchID, true, nil
}

// updateMarkerFutureCone updates the future cone of a Marker to belong to the given conflict BranchID.
func (b *Booker) updateMarkerFutureCone(marker *markers.Marker, newConflictBranchID ledgerstate.BranchID) (err error) {
	walk := walker.New()
	walk.Push(marker)

	for walk.HasNext() {
		currentMarker := walk.Next().(*markers.Marker)

		if err = b.updateMarker(currentMarker, newConflictBranchID, walk); err != nil {
			err = errors.Errorf("failed to propagate Conflict%s to Messages approving %s: %w", newConflictBranchID, currentMarker, err)
			return
		}
	}

	return
}

// updateMarker updates a single Marker and queues the next Elements that need to be updated.
func (b *Booker) updateMarker(currentMarker *markers.Marker, conflictBranchID ledgerstate.BranchID, walk *walker.Walker) (err error) {
	oldBranchID := b.MarkersManager.BranchID(currentMarker)
	newBranchID, branchIDUpdated, err := b.updatedBranchID(oldBranchID, conflictBranchID)
	if err != nil {
		err = errors.Errorf("failed to add Conflict%s to BranchID %s: %w", b.MarkersManager.BranchID(currentMarker), conflictBranchID, err)
		return
	}
	if !branchIDUpdated || !b.MarkersManager.SetBranchID(currentMarker, newBranchID) {
		return
	}

	b.Events.MarkerBranchUpdated.Trigger(currentMarker, oldBranchID, newBranchID)

	b.MarkersManager.UnregisterSequenceAliasMapping(markers.NewSequenceAlias(oldBranchID.Bytes()), currentMarker.SequenceID())

	b.MarkersManager.Sequence(currentMarker.SequenceID()).Consume(func(sequence *markers.Sequence) {
		sequence.ReferencingMarkers(currentMarker.Index()).ForEachSorted(func(referencingSequenceID markers.SequenceID, referencingIndex markers.Index) bool {
			walk.Push(markers.NewMarker(referencingSequenceID, referencingIndex))

			b.updateIndividuallyMappedMessages(b.MarkersManager.BranchID(markers.NewMarker(referencingSequenceID, referencingIndex)), currentMarker, conflictBranchID)

			return true
		})
	})

	return
}

// updateIndividuallyMappedMessages updates the Messages that have their BranchID set in the MessageMetadata.
func (b *Booker) updateIndividuallyMappedMessages(oldChildBranch ledgerstate.BranchID, currentMarker *markers.Marker, newConflictBranchID ledgerstate.BranchID) {
	newBranchID, branchIDUpdated, err := b.updatedBranchID(oldChildBranch, newConflictBranchID)
	if err != nil {
		return
	} else if !branchIDUpdated {
		return
	}

	b.tangle.Storage.IndividuallyMappedMessages(oldChildBranch).Consume(func(individuallyMappedMessage *IndividuallyMappedMessage) {
		if index, sequenceExists := individuallyMappedMessage.PastMarkers().Get(currentMarker.SequenceID()); !sequenceExists || index < currentMarker.Index() {
			return
		}

		individuallyMappedMessage.Delete()

		b.tangle.Storage.MessageMetadata(individuallyMappedMessage.MessageID()).Consume(func(messageMetadata *MessageMetadata) {
			messageMetadata.SetBranchID(newBranchID)
			b.tangle.Storage.StoreIndividuallyMappedMessage(NewIndividuallyMappedMessage(newBranchID, individuallyMappedMessage.MessageID(), individuallyMappedMessage.PastMarkers()))
		})
	})
}

// updateMetadataFutureCone updates the future cone of a Message to belong to the given conflict BranchID.
func (b *Booker) updateMetadataFutureCone(messageMetadata *MessageMetadata, newConflictBranchID ledgerstate.BranchID, walk *walker.Walker) (err error) {
	oldBranchID, err := b.MessageBranchID(messageMetadata.ID())
	if err != nil {
		err = errors.Errorf("failed to propagate conflict%s to MessageMetadata of %s: %w", newConflictBranchID, messageMetadata.ID(), err)
		return
	}

	newBranchID, branchIDUpdated, err := b.updatedBranchID(oldBranchID, newConflictBranchID)
	if err != nil {
		err = errors.Errorf("failed to propagate conflict%s to MessageMetadata of %s: %w", newConflictBranchID, messageMetadata.ID(), err)
		return
	} else if !branchIDUpdated || !messageMetadata.SetBranchID(newBranchID) {
		return
	}

	b.tangle.Storage.DeleteIndividuallyMappedMessage(oldBranchID, messageMetadata.ID())
	b.tangle.Storage.StoreIndividuallyMappedMessage(NewIndividuallyMappedMessage(newBranchID, messageMetadata.ID(), messageMetadata.StructureDetails().PastMarkers))

	b.Events.MessageBranchUpdated.Trigger(messageMetadata.ID(), oldBranchID, newBranchID)

	for _, approvingMessageID := range b.tangle.Utils.ApprovingMessageIDs(messageMetadata.ID(), StrongApprover) {
		walk.Push(approvingMessageID)
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BookerEvents /////////////////////////////////////////////////////////////////////////////////////////////////

// BookerEvents represents events happening in the Booker.
type BookerEvents struct {
	// MessageBooked is triggered when a Message was booked (it's Branch and it's Payload's Branch where determined).
	MessageBooked *events.Event

	// MessageBranchUpdated is triggered when the BranchID of a Message is changed in its MessageMetadata.
	MessageBranchUpdated *events.Event

	// MarkerBranchUpdated is triggered when a Marker is mapped to a new BranchID.
	MarkerBranchUpdated *events.Event

	// Error gets triggered when the Booker faces an unexpected error.
	Error *events.Event
}

func markerBranchUpdatedCaller(handler interface{}, params ...interface{}) {
	handler.(func(marker *markers.Marker, oldBranchID, newBranchID ledgerstate.BranchID))(params[0].(*markers.Marker), params[1].(ledgerstate.BranchID), params[2].(ledgerstate.BranchID))
}

func messageBranchUpdatedCaller(handler interface{}, params ...interface{}) {
	handler.(func(messageID MessageID, oldBranchID, newBranchID ledgerstate.BranchID))(params[0].(MessageID), params[1].(ledgerstate.BranchID), params[2].(ledgerstate.BranchID))
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
// strong parents.
func (m *MarkersManager) InheritStructureDetails(message *Message, sequenceAlias markers.SequenceAlias) (structureDetails *markers.StructureDetails) {
	structureDetails, _ = m.Manager.InheritStructureDetails(m.structureDetailsOfStrongParents(message), m.tangle.Options.IncreaseMarkersIndexCallback, sequenceAlias)
	if structureDetails.IsPastMarker {
		m.SetMessageID(structureDetails.PastMarkers.Marker(), message.ID())
		m.tangle.Utils.WalkMessageMetadata(m.propagatePastMarkerToFutureMarkers(structureDetails.PastMarkers.Marker()), message.StrongParents())
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

// SetBranchID associates a BranchID with the given Marker.
func (m *MarkersManager) SetBranchID(marker *markers.Marker, branchID ledgerstate.BranchID) (updated bool) {
	if floorMarker, floorBranchID, exists := m.Floor(marker); exists {
		if floorBranchID == branchID {
			return false
		}

		if floorMarker == marker.Index() {
			m.UnregisterSequenceAliasMapping(markers.NewSequenceAlias(floorBranchID.Bytes()), marker.SequenceID())
		}
		m.RegisterSequenceAliasMapping(markers.NewSequenceAlias(branchID.Bytes()), marker.SequenceID())
	}

	m.tangle.Storage.MarkerIndexBranchIDMapping(marker.SequenceID(), NewMarkerIndexBranchIDMapping).Consume(func(markerIndexBranchIDMapping *MarkerIndexBranchIDMapping) {
		markerIndexBranchIDMapping.SetBranchID(marker.Index(), branchID)
	})

	return true
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
				for _, strongParentMessageID := range message.StrongParents() {
					walker.Push(strongParentMessageID)
				}
			})
		}
	}
}

// structureDetailsOfStrongParents is an internal utility function that returns a list of StructureDetails of all the
// strong parents.
func (m *MarkersManager) structureDetailsOfStrongParents(message *Message) (structureDetails []*markers.StructureDetails) {
	structureDetails = make([]*markers.StructureDetails, 0)
	message.ForEachStrongParent(func(parentMessageID MessageID) {
		if !m.tangle.Storage.MessageMetadata(parentMessageID).Consume(func(messageMetadata *MessageMetadata) {
			structureDetails = append(structureDetails, messageMetadata.StructureDetails())
		}) {
			panic(fmt.Errorf("failed to load MessageMetadata of Message with %s", parentMessageID))
		}
	})

	return
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

	markerBranchMapping.Persist()
	markerBranchMapping.SetModified()

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
// unmarshaling).
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

	m.SetModified()

	m.mapping.Set(index, branchID)
}

// Floor returns the largest Index that is <= the given Index which has a mapped BranchID (and a boolean value
// indicating if it exists).
func (m *MarkerIndexBranchIDMapping) Floor(index markers.Index) (marker markers.Index, branchID ledgerstate.BranchID, exists bool) {
	if untypedIndex, untypedBranchID, exists := m.mapping.Floor(index); exists {
		return untypedIndex.(markers.Index), untypedBranchID.(ledgerstate.BranchID), true
	}

	return 0, ledgerstate.UndefinedBranchID, false
}

// Ceiling returns the smallest Index that is >= the given Index which has a mapped BranchID (and a boolean value
// indicating if it exists).
func (m *MarkerIndexBranchIDMapping) Ceiling(index markers.Index) (marker markers.Index, branchID ledgerstate.BranchID, exists bool) {
	if untypedIndex, untypedBranchID, exists := m.mapping.Ceiling(index); exists {
		return untypedIndex.(markers.Index), untypedBranchID.(ledgerstate.BranchID), true
	}

	return 0, ledgerstate.UndefinedBranchID, false
}

// Bytes returns a marshaled version of the MarkerIndexBranchIDMapping.
func (m *MarkerIndexBranchIDMapping) Bytes() []byte {
	return byteutils.ConcatBytes(m.ObjectStorageKey(), m.ObjectStorageValue())
}

// String returns a human readable version of the MarkerIndexBranchIDMapping.
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

// String returns a human readable version of the CachedMarkerIndexBranchIDMapping.
func (c *CachedMarkerIndexBranchIDMapping) String() string {
	return stringify.Struct("CachedMarkerIndexBranchIDMapping",
		stringify.StructField("CachedObject", c.Unwrap()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region IndividuallyMappedMessage ////////////////////////////////////////////////////////////////////////////////////

// IndividuallyMappedMessagePartitionKeys defines the "layout" of the key. This enables prefix iterations in the object
// storage.
var IndividuallyMappedMessagePartitionKeys = objectstorage.PartitionKey([]int{ledgerstate.BranchIDLength, MessageIDLength}...)

// IndividuallyMappedMessage is a data structure that denotes if a Message has its BranchID set individually in its own
// MessageMetadata.
type IndividuallyMappedMessage struct {
	branchID    ledgerstate.BranchID
	messageID   MessageID
	pastMarkers *markers.Markers

	objectstorage.StorableObjectFlags
}

// NewIndividuallyMappedMessage is the constructor for the IndividuallyMappedMessage.
func NewIndividuallyMappedMessage(branchID ledgerstate.BranchID, messageID MessageID, pastMarkers *markers.Markers) *IndividuallyMappedMessage {
	return &IndividuallyMappedMessage{
		branchID:    branchID,
		messageID:   messageID,
		pastMarkers: pastMarkers,
	}
}

// IndividuallyMappedMessageFromBytes unmarshals an IndividuallyMappedMessage from a sequence of bytes.
func IndividuallyMappedMessageFromBytes(bytes []byte) (individuallyMappedMessage *IndividuallyMappedMessage, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if individuallyMappedMessage, err = IndividuallyMappedMessageFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse IndividuallyMappedMessage from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// IndividuallyMappedMessageFromMarshalUtil unmarshals an IndividuallyMappedMessage using a MarshalUtil (for easier unmarshaling).
func IndividuallyMappedMessageFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (individuallyMappedMessage *IndividuallyMappedMessage, err error) {
	individuallyMappedMessage = &IndividuallyMappedMessage{}
	if individuallyMappedMessage.branchID, err = ledgerstate.BranchIDFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse BranchID from MarshalUtil: %w", err)
		return
	}
	if individuallyMappedMessage.messageID, err = MessageIDFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse MessageID from MarshalUtil: %w", err)
		return
	}
	if individuallyMappedMessage.pastMarkers, err = markers.FromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse Markers from MarshalUtil: %w", err)
		return
	}

	return
}

// IndividuallyMappedMessageFromObjectStorage is a factory method that creates a new IndividuallyMappedMessage instance
// from a storage key of the object storage. It is used by the object storage, to create new instances of this entity.
func IndividuallyMappedMessageFromObjectStorage(key, value []byte) (result objectstorage.StorableObject, err error) {
	if result, _, err = IndividuallyMappedMessageFromBytes(byteutils.ConcatBytes(key, value)); err != nil {
		err = errors.Errorf("failed to parse IndividuallyMappedMessage from bytes: %w", err)
		return
	}

	return
}

// BranchID returns the BranchID that the Message that has its Branch mapped in its MessageMetadata is currently booked
// into.
func (i *IndividuallyMappedMessage) BranchID() ledgerstate.BranchID {
	return i.branchID
}

// MessageID returns the MessageID of the Message that has its Branch mapped in its MessageMetadata.
func (i *IndividuallyMappedMessage) MessageID() MessageID {
	return i.messageID
}

// PastMarkers returns the PastMarkers of the Message that has its Branch mapped in its MessageMetadata.
func (i *IndividuallyMappedMessage) PastMarkers() *markers.Markers {
	return i.pastMarkers
}

// Bytes returns a marshaled version of the IndividuallyMappedMessage.
func (i *IndividuallyMappedMessage) Bytes() []byte {
	return byteutils.ConcatBytes(i.ObjectStorageKey(), i.ObjectStorageValue())
}

// String returns a human readable version of the IndividuallyMappedMessage.
func (i *IndividuallyMappedMessage) String() string {
	return stringify.Struct("IndividuallyMappedMessage",
		stringify.StructField("branchID", i.branchID),
		stringify.StructField("messageID", i.messageID),
		stringify.StructField("pastMarkers", i.pastMarkers),
	)
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (i *IndividuallyMappedMessage) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (i *IndividuallyMappedMessage) ObjectStorageKey() []byte {
	return byteutils.ConcatBytes(i.branchID.Bytes(), i.messageID.Bytes())
}

// ObjectStorageValue marshals the IndividuallyMappedMessage into a sequence of bytes that are used as the value part in
// the object storage.
func (i *IndividuallyMappedMessage) ObjectStorageValue() []byte {
	return i.pastMarkers.Bytes()
}

// code contract (make sure the type implements all required methods)
var _ objectstorage.StorableObject = &IndividuallyMappedMessage{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedIndividuallyMappedMessage //////////////////////////////////////////////////////////////////////////////

// CachedIndividuallyMappedMessage is a wrapper for the generic CachedObject returned by the object storage that
// overrides the accessor methods with a type-casted one.
type CachedIndividuallyMappedMessage struct {
	objectstorage.CachedObject
}

// Retain marks the CachedObject to still be in use by the program.
func (c *CachedIndividuallyMappedMessage) Retain() *CachedIndividuallyMappedMessage {
	return &CachedIndividuallyMappedMessage{c.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (c *CachedIndividuallyMappedMessage) Unwrap() *IndividuallyMappedMessage {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*IndividuallyMappedMessage)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (c *CachedIndividuallyMappedMessage) Consume(consumer func(individuallyMappedMessage *IndividuallyMappedMessage), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*IndividuallyMappedMessage))
	}, forceRelease...)
}

// String returns a human readable version of the CachedIndividuallyMappedMessage.
func (c *CachedIndividuallyMappedMessage) String() string {
	return stringify.Struct("CachedIndividuallyMappedMessage",
		stringify.StructField("CachedObject", c.Unwrap()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedIndividuallyMappedMessages /////////////////////////////////////////////////////////////////////////////

// CachedIndividuallyMappedMessages defines a slice of *CachedIndividuallyMappedMessage.
type CachedIndividuallyMappedMessages []*CachedIndividuallyMappedMessage

// Unwrap is the type-casted equivalent of Get. It returns a slice of unwrapped objects with the object being nil if it
// does not exist.
func (c CachedIndividuallyMappedMessages) Unwrap() (unwrappedIndividuallyMappedMessages []*IndividuallyMappedMessage) {
	unwrappedIndividuallyMappedMessages = make([]*IndividuallyMappedMessage, len(c))
	for i, cachedIndividuallyMappedMessage := range c {
		untypedObject := cachedIndividuallyMappedMessage.Get()
		if untypedObject == nil {
			continue
		}

		typedObject := untypedObject.(*IndividuallyMappedMessage)
		if typedObject == nil || typedObject.IsDeleted() {
			continue
		}

		unwrappedIndividuallyMappedMessages[i] = typedObject
	}

	return
}

// Consume iterates over the CachedObjects, unwraps them and passes a type-casted version to the consumer (if the object
// is not empty - it exists). It automatically releases the object when the consumer finishes. It returns true, if at
// least one object was consumed.
func (c CachedIndividuallyMappedMessages) Consume(consumer func(individuallyMappedMessage *IndividuallyMappedMessage), forceRelease ...bool) (consumed bool) {
	for _, cachedIndividuallyMappedMessage := range c {
		consumed = cachedIndividuallyMappedMessage.Consume(consumer, forceRelease...) || consumed
	}

	return
}

// Release is a utility function that allows us to release all CachedObjects in the collection.
func (c CachedIndividuallyMappedMessages) Release(force ...bool) {
	for _, cachedIndividuallyMappedMessage := range c {
		cachedIndividuallyMappedMessage.Release(force...)
	}
}

// String returns a human readable version of the CachedIndividuallyMappedMessages.
func (c CachedIndividuallyMappedMessages) String() string {
	structBuilder := stringify.StructBuilder("CachedIndividuallyMappedMessages")
	for i, cachedIndividuallyMappedMessage := range c {
		structBuilder.AddField(stringify.StructField(strconv.Itoa(i), cachedIndividuallyMappedMessage))
	}

	return structBuilder.String()
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

// MarkerMessageMappingFromMarshalUtil unmarshals an MarkerMessageMapping using a MarshalUtil (for easier unmarshaling).
func MarkerMessageMappingFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (markerMessageMapping *MarkerMessageMapping, err error) {
	markerMessageMapping = &MarkerMessageMapping{}
	if markerMessageMapping.marker, err = markers.MarkerFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse Marker from MarshalUtil: %w", err)
		return
	}
	if markerMessageMapping.messageID, err = MessageIDFromMarshalUtil(marshalUtil); err != nil {
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

// String returns a human readable version of the MarkerMessageMapping.
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

// code contract (make sure the type implements all required methods)
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

// String returns a human readable version of the CachedMarkerMessageMapping.
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

// String returns a human readable version of the CachedMarkerMessageMappings.
func (c CachedMarkerMessageMappings) String() string {
	structBuilder := stringify.StructBuilder("CachedMarkerMessageMappings")
	for i, cachedMarkerMessageMapping := range c {
		structBuilder.AddField(stringify.StructField(strconv.Itoa(i), cachedMarkerMessageMapping))
	}

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
