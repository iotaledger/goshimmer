package tangle

import (
	"fmt"
	"sort"
	"strconv"
	"sync"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/datastructure/thresholdmap"
	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/types"
	"golang.org/x/xerrors"
)

// region Booker ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Booker is a Tangle component that takes care of booking Messages and Transactions by assigning them to the
// corresponding Branch of the ledger state.
type Booker struct {
	// Events is a dictionary for the Booker related Events.
	Events *BookerEvents

	tangle                       *Tangle
	MarkersManager               *MarkersManager
	MarkerBranchIDMappingManager *MarkerBranchIDMappingManager
}

// NewBooker is the constructor of a Booker.
func NewBooker(tangle *Tangle) (messageBooker *Booker) {
	messageBooker = &Booker{
		Events: &BookerEvents{
			MessageBooked: events.NewEvent(messageIDEventHandler),
		},
		tangle:                       tangle,
		MarkersManager:               NewMarkersManager(tangle),
		MarkerBranchIDMappingManager: NewMarkerBranchIDMappingManager(tangle),
	}

	return
}

// Shutdown shuts down the Booker and persists its state.
func (b *Booker) Shutdown() {
	b.MarkersManager.Shutdown()
}

// Setup sets up the behavior of the component by making it attach to the relevant events of other components.
func (b *Booker) Setup() {
	b.tangle.Scheduler.Events.MessageScheduled.Attach(events.NewClosure(func(messageID MessageID) {
		err := b.Book(messageID)
		if err != nil {
			b.tangle.Events.Error.Trigger(err)
		}
	}))
	b.tangle.LedgerState.utxoDAG.Events.TransactionBranchIDUpdated.Attach(events.NewClosure(b.UpdateMessagesBranch))
}

// UpdateMessagesBranch propagates the update of the message's branchID (and its future cone) in case on changes of it contained transction's branchID.
func (b *Booker) UpdateMessagesBranch(transactionID ledgerstate.TransactionID) {
	b.tangle.Utils.WalkMessageAndMetadata(func(message *Message, messageMetadata *MessageMetadata, walker *walker.Walker) {
		if messageMetadata.IsBooked() {
			inheritedBranch, inheritErr := b.tangle.LedgerState.InheritBranch(b.branchIDsOfParents(message).Add(b.branchIDOfPayload(message)))
			if inheritErr != nil {
				panic(xerrors.Errorf("failed to inherit Branch when booking Message with %s: %w", message.ID(), inheritErr))
			}
			if messageMetadata.SetBranchID(inheritedBranch) {
				for _, approvingMessageID := range b.tangle.Utils.ApprovingMessageIDs(message.ID(), StrongApprover) {
					walker.Push(approvingMessageID)
				}
			}
		}
	}, b.tangle.Storage.AttachmentMessageIDs(transactionID), true)
}

// Book tries to book the given Message (and potentially its contained Transaction) into the LedgerState and the Tangle.
// It fires a MessageBooked event if it succeeds.
func (b *Booker) Book(messageID MessageID) (err error) {
	b.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		b.tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *MessageMetadata) {
			sequenceAlias := make([]markers.SequenceAlias, 0)
			combinedBranches := b.branchIDsOfParents(message)
			if payload := message.Payload(); payload != nil && payload.Type() == ledgerstate.TransactionType {
				transaction := payload.(*ledgerstate.Transaction)
				if valid, er := b.tangle.LedgerState.TransactionValid(transaction, messageID); !valid {
					err = er
					return
				}

				if !b.tangle.Utils.AllTransactionsApprovedByMessage(transaction.ReferencedTransactionIDs(), messageID) {
					b.tangle.Events.MessageInvalid.Trigger(messageID)
					err = fmt.Errorf("message does not reference all the transaction's dependencies")
					return
				}

				targetBranch, bookingErr := b.tangle.LedgerState.BookTransaction(transaction, messageID)
				if bookingErr != nil {
					err = xerrors.Errorf("failed to book Transaction of Message with %s: %w", messageID, err)
					return
				}
				combinedBranches = combinedBranches.Add(targetBranch)
				if ledgerstate.NewBranchID(transaction.ID()) == targetBranch {
					sequenceAlias = append(sequenceAlias, markers.NewSequenceAlias(targetBranch.Bytes()))
				}

				for _, output := range transaction.Essence().Outputs() {
					b.tangle.LedgerState.utxoDAG.StoreAddressOutputMapping(output.Address(), output.ID())
				}

				attachment, stored := b.tangle.Storage.StoreAttachment(transaction.ID(), messageID)
				if stored {
					attachment.Release()
				}
			}

			inheritedBranch, inheritErr := b.tangle.LedgerState.InheritBranch(combinedBranches)
			if inheritErr != nil {
				err = xerrors.Errorf("failed to inherit Branch when booking Message with %s: %w", messageID, inheritErr)
				return
			}

			messageMetadata.SetBranchID(inheritedBranch)
			messageMetadata.SetStructureDetails(b.MarkersManager.InheritStructureDetails(message, sequenceAlias...))
			messageMetadata.SetBooked(true)

			b.Events.MessageBooked.Trigger(messageID)
		})
	})

	return
}

func (b *Booker) branchIDOfPayload(message *Message) (branchIDOfPayload ledgerstate.BranchID) {
	payload := message.Payload()
	if payload == nil || payload.Type() != ledgerstate.TransactionType {
		branchIDOfPayload = ledgerstate.MasterBranchID
		return
	}
	transactionID := payload.(*ledgerstate.Transaction).ID()
	if !b.tangle.LedgerState.utxoDAG.TransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
		branchIDOfPayload = transactionMetadata.BranchID()
	}) {
		panic(fmt.Sprintf("failed to load TransactionMetadata of %s: ", transactionID))
	}
	return
}

// branchIDsOfParents returns the BranchIDs of the parents of the given Message.
func (b *Booker) branchIDsOfParents(message *Message) (branchIDs ledgerstate.BranchIDs) {
	branchIDs = make(ledgerstate.BranchIDs)

	message.ForEachStrongParent(func(parentMessageID MessageID) {
		if parentMessageID == EmptyMessageID {
			return
		}

		if !b.tangle.Storage.MessageMetadata(parentMessageID).Consume(func(messageMetadata *MessageMetadata) {
			branchIDs[messageMetadata.BranchID()] = types.Void
		}) {
			panic(fmt.Errorf("failed to load MessageMetadata with %s", parentMessageID))
		}
	})

	message.ForEachWeakParent(func(parentMessageID MessageID) {
		if parentMessageID == EmptyMessageID {
			return
		}
		if !b.tangle.Storage.Message(parentMessageID).Consume(func(message *Message) {
			if payload := message.Payload(); payload != nil && payload.Type() == ledgerstate.TransactionType {
				transactionID := payload.(*ledgerstate.Transaction).ID()

				if !b.tangle.LedgerState.utxoDAG.TransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
					branchIDs[transactionMetadata.BranchID()] = types.Void
				}) {
					panic(fmt.Errorf("failed to load TransactionMetadata with %s", transactionID))
				}
			}

		}) {
			panic(fmt.Errorf("failed to load MessageMetadata with %s", parentMessageID))
		}
	})

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BookerEvents /////////////////////////////////////////////////////////////////////////////////////////////////

// BookerEvents represents events happening in the Booker.
type BookerEvents struct {
	// MessageBooked is triggered when a Message was booked (it's Branch and it's Payload's Branch where determined).
	MessageBooked *events.Event
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
		Manager: markers.NewManager(tangle.Options.Store),
	}
}

// InheritStructureDetails returns the structure Details of a Message that are derived from the StructureDetails of its
// strong parents.
func (m *MarkersManager) InheritStructureDetails(message *Message, newSequenceAlias ...markers.SequenceAlias) (structureDetails *markers.StructureDetails) {
	structureDetails, _ = m.Manager.InheritStructureDetails(m.structureDetailsOfStrongParents(message), m.tangle.Options.IncreaseMarkersIndexCallback, newSequenceAlias...)

	if structureDetails.IsPastMarker {
		m.tangle.Utils.WalkMessageMetadata(m.propagatePastMarkerToFutureMarkers(structureDetails.PastMarkers.FirstMarker()), message.StrongParents())
	}

	return
}

// propagatePastMarkerToFutureMarkers updates the FutureMarkers of the strong parents of a given message when a new
// PastMaster was assigned.
func (m *MarkersManager) propagatePastMarkerToFutureMarkers(pastMarkerToInherit *markers.Marker) func(messageMetadata *MessageMetadata, walker *walker.Walker) {
	return func(messageMetadata *MessageMetadata, walker *walker.Walker) {
		_, inheritFurther := m.UpdateStructureDetails(messageMetadata.StructureDetails(), pastMarkerToInherit)
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
func increaseMarkersIndexCallbackStrategy(sequenceID markers.SequenceID, currentHighestIndex markers.Index) bool {
	return true
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MarkerBranchIDMappingManager /////////////////////////////////////////////////////////////////////////////////

// MarkerBranchIDMappingManager is a data structure that enables the mapping of Markers to BranchIDs with binary search
// efficiency (O(log(n)) where n is the amount of Markers that have a unique BranchID value).
type MarkerBranchIDMappingManager struct {
	tangle *Tangle
}

// NewMarkerBranchIDMappingManager is the constructor for the MarkerBranchIDMappingManager.
func NewMarkerBranchIDMappingManager(tangle *Tangle) (markerBranchIDMappingManager *MarkerBranchIDMappingManager) {
	markerBranchIDMappingManager = &MarkerBranchIDMappingManager{
		tangle: tangle,
	}

	return
}

// BranchID returns the BranchID that is associated with the given Marker.
func (m *MarkerBranchIDMappingManager) BranchID(marker *markers.Marker) (branchID ledgerstate.BranchID) {
	m.tangle.Storage.MarkerIndexBranchIDMapping(marker.SequenceID(), func(sequenceID markers.SequenceID) *MarkerIndexBranchIDMapping {
		panic(fmt.Sprintf("tried to retrieve the BranchID of unknown marker.%s", sequenceID))
	}).Consume(func(markerIndexBranchIDMapping *MarkerIndexBranchIDMapping) {
		branchID = markerIndexBranchIDMapping.BranchID(marker.Index())
	})

	return
}

// SetBranchID associates a BranchID with the given Marker.
func (m *MarkerBranchIDMappingManager) SetBranchID(marker *markers.Marker, branchID ledgerstate.BranchID) {
	m.tangle.Storage.MarkerIndexBranchIDMapping(marker.SequenceID(), NewMarkerIndexBranchIDMapping).Consume(func(markerIndexBranchIDMapping *MarkerIndexBranchIDMapping) {
		markerIndexBranchIDMapping.SetBranchID(marker.Index(), branchID)
	})
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

	return
}

// MarkerIndexBranchIDMappingFromBytes unmarshals a MarkerIndexBranchIDMapping from a sequence of bytes.
func MarkerIndexBranchIDMappingFromBytes(bytes []byte) (markerIndexBranchIDMapping *MarkerIndexBranchIDMapping, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if markerIndexBranchIDMapping, err = MarkerIndexBranchIDMappingFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse MarkerIndexBranchIDMapping from MarshalUtil: %w", err)
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
		err = xerrors.Errorf("failed to parse SequenceID from MarshalUtil: %w", err)
		return
	}
	mappingCount, mappingCountErr := marshalUtil.ReadUint64()
	if mappingCountErr != nil {
		err = xerrors.Errorf("failed to parse reference count (%v): %w", mappingCountErr, cerrors.ErrParseBytesFailed)
		return
	}
	markerIndexBranchIDMapping.mapping = thresholdmap.New(thresholdmap.LowerThresholdMode, markerIndexComparator)
	for j := uint64(0); j < mappingCount; j++ {
		index, indexErr := marshalUtil.ReadUint64()
		if indexErr != nil {
			err = xerrors.Errorf("failed to parse Index (%v): %w", indexErr, cerrors.ErrParseBytesFailed)
			return
		}

		branchID, branchIDErr := ledgerstate.BranchIDFromMarshalUtil(marshalUtil)
		if branchIDErr != nil {
			err = xerrors.Errorf("failed to parse BranchID: %w", branchIDErr)
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
		err = xerrors.Errorf("failed to parse MarkerIndexBranchIDMapping from bytes: %w", err)
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
func (m *MarkerIndexBranchIDMapping) Update(other objectstorage.StorableObject) {
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
