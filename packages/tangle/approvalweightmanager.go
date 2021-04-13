package tangle

import (
	"fmt"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/datastructure/set"
	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/xerrors"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
)

const (
	lowerWeightThreshold        = float64(0.01)
	confirmationThreshold       = 0.5
	markerConfirmationThreshold = 0.5
)

// region ApprovalWeightManager ////////////////////////////////////////////////////////////////////////////////////////

type ApprovalWeightManager struct {
	Events               *ApprovalWeightManagerEvents
	tangle               *Tangle
	supportersManager    *SupporterManager
	branchWeights        map[ledgerstate.BranchID]*BranchWeightPerEpoch
	branchWeightsMutex   sync.RWMutex
	lastConfirmedMarkers map[markers.SequenceID]markers.Index
}

func NewApprovalWeightManager(tangle *Tangle) *ApprovalWeightManager {
	return &ApprovalWeightManager{
		Events: &ApprovalWeightManagerEvents{
			MessageProcessed: events.NewEvent(MessageIDCaller),
			BranchConfirmed:  events.NewEvent(branchIDCaller),
			MarkerConfirmed:  events.NewEvent(markerCaller),
		},
		tangle:               tangle,
		supportersManager:    NewSupporterManager(tangle),
		branchWeights:        make(map[ledgerstate.BranchID]*BranchWeightPerEpoch),
		lastConfirmedMarkers: make(map[markers.SequenceID]markers.Index),
	}
}

func (a *ApprovalWeightManager) Setup() {
	a.tangle.Booker.Events.MessageBooked.Attach(events.NewClosure(a.ProcessMessage))
	a.tangle.Booker.Events.MessageBranchUpdated.Attach(events.NewClosure(a.UpdateMessageBranch))
	a.tangle.Booker.Events.MarkerBranchUpdated.Attach(events.NewClosure(a.UpdateMarkerBranch))

	a.supportersManager.Events.BranchSupportAdded.Attach(events.NewClosure(a.onBranchSupportAdded))
	a.supportersManager.Events.BranchSupportRemoved.Attach(events.NewClosure(a.onBranchSupportRemoved))
	a.supportersManager.Events.SequenceSupportUpdated.Attach(events.NewClosure(a.onSequenceSupportUpdated))
}

func (a *ApprovalWeightManager) UpdateMessageBranch(messageID MessageID, _, newBranchID ledgerstate.BranchID) {
	a.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		a.supportersManager.propagateSupportToBranches(newBranchID, message)
	})
}

func (a *ApprovalWeightManager) UpdateMarkerBranch(marker *markers.Marker, oldBranchID, newBranchID ledgerstate.BranchID) {
	fmt.Println("HUHU!!! :D")
	a.supportersManager.MigrateMarkerSupportersToNewBranch(marker, oldBranchID, newBranchID)
}

func (a *ApprovalWeightManager) ProcessMessage(messageID MessageID) {
	a.supportersManager.ProcessMessage(messageID)

	a.Events.MessageProcessed.Trigger(messageID)
}

// WeightOfBranch returns the weight of the given Branch that was added by Supporters of the given epoch.
func (a *ApprovalWeightManager) WeightOfBranch(branchID ledgerstate.BranchID, epochID uint64) (weight float64) {
	return a.weightsPerEpoch(branchID).Weight(epochID)
}

func (a *ApprovalWeightManager) WeightOfMarker(marker *markers.Marker, anchorTime time.Time) (weight float64) {
	activeMana, totalMana := a.tangle.WeightProvider.WeightsOfRelevantSupporters(anchorTime)
	branchID := a.tangle.Booker.MarkersManager.BranchID(marker)
	supportersOfMarker := a.supportersManager.SupportersOfMarker(marker)
	supporterMana := float64(0)
	if branchID == ledgerstate.MasterBranchID {
		supportersOfMarker.ForEach(func(supporter Supporter) {
			supporterMana += activeMana[supporter]
		})
	} else {
		fmt.Println(branchID)
		a.supportersManager.SupportersOfBranch(branchID).ForEach(func(supporter Supporter) {
			if supportersOfMarker.Has(supporter) {
				supporterMana += activeMana[supporter]
			}
		})
	}

	return supporterMana / totalMana
}

func (a *ApprovalWeightManager) weightsPerEpoch(branchID ledgerstate.BranchID) *BranchWeightPerEpoch {
	a.branchWeightsMutex.RLock()
	weightsPerEpoch, exists := a.branchWeights[branchID]
	if exists {
		a.branchWeightsMutex.RUnlock()
		return weightsPerEpoch
	}

	a.branchWeightsMutex.RUnlock()
	a.branchWeightsMutex.Lock()
	defer a.branchWeightsMutex.Unlock()

	weightsPerEpoch, exists = a.branchWeights[branchID]
	if exists {
		return weightsPerEpoch
	}

	weightsPerEpoch = NewBranchWeightPerEpoch()
	a.branchWeights[branchID] = weightsPerEpoch

	return weightsPerEpoch
}

func (a *ApprovalWeightManager) onSequenceSupportUpdated(marker *markers.Marker, message *Message) {
	if index, exists := a.lastConfirmedMarkers[marker.SequenceID()]; exists && index >= marker.Index() {
		return
	}

	fmt.Println("onSequenceSupportUpdated", marker)
	activeMana, totalMana := a.tangle.WeightProvider.WeightsOfRelevantSupporters(message.IssuingTime())

	for i := a.lowestLastConfirmedMarker(marker.SequenceID()); i <= marker.Index(); i++ {
		currentMarker := markers.NewMarker(marker.SequenceID(), i)
		fmt.Println(currentMarker)
		branchID := a.tangle.Booker.MarkersManager.BranchID(currentMarker)
		if a.tangle.LedgerState.BranchDAG.InclusionState(branchID) != ledgerstate.Confirmed {
			break
		}

		supportersOfMarker := a.supportersManager.SupportersOfMarker(currentMarker)
		supporterMana := float64(0)
		if branchID == ledgerstate.MasterBranchID {
			supportersOfMarker.ForEach(func(supporter Supporter) {
				supporterMana += activeMana[supporter]
			})
		} else {
			a.supportersManager.SupportersOfBranch(branchID).ForEach(func(supporter Supporter) {
				if supportersOfMarker.Has(supporter) {
					supporterMana += activeMana[supporter]
				}
			})
		}

		fmt.Println("weight:", supporterMana/totalMana)

		if supporterMana/totalMana < markerConfirmationThreshold {
			break
		}

		a.lastConfirmedMarkers[marker.SequenceID()] = currentMarker.Index()

		fmt.Println("Marker confirmed!!")
		a.Events.MarkerConfirmed.Trigger(currentMarker)
	}

	fmt.Println("===============================================")
}

// lowestLastConfirmedMarker is an internal utility function that returns the last confirmed or lowest marker in a sequence.
func (a *ApprovalWeightManager) lowestLastConfirmedMarker(sequenceID markers.SequenceID) (index markers.Index) {
	index = a.lastConfirmedMarkers[sequenceID] + 1
	a.tangle.Booker.MarkersManager.Manager.Sequence(sequenceID).Consume(func(sequence *markers.Sequence) {
		if index < sequence.LowestIndex() {
			index = sequence.LowestIndex()
		}
	})

	return
}

func (a *ApprovalWeightManager) onBranchSupportAdded(branchID ledgerstate.BranchID, message *Message) {
	fmt.Println("onBranchSupportAdded", branchID)
	if a.tangle.LedgerState.BranchDAG.InclusionState(branchID) == ledgerstate.Confirmed {
		return
	}

	weightOfSupporter, totalMana, epochID := a.tangle.WeightProvider.Weight(message)
	fmt.Printf("weightOfSupporter: %0.2f, totalMana: %0.2f\n", weightOfSupporter, totalMana)

	branchWeight := a.weightsPerEpoch(branchID).AddWeight(epochID, weightOfSupporter/totalMana)
	fmt.Println("BranchWeight:", branchWeight)
	fmt.Println("weightOfLargestConflictingBranch", a.weightOfLargestConflictingBranch(branchID, epochID))
	fmt.Println("conf", branchWeight-a.weightOfLargestConflictingBranch(branchID, epochID))
	diff := branchWeight - a.weightOfLargestConflictingBranch(branchID, epochID)
	if diff-confirmationThreshold > -0.0005 {
		a.Events.BranchConfirmed.Trigger(branchID)
	}
}

func (a *ApprovalWeightManager) onBranchSupportRemoved(branchID ledgerstate.BranchID, message *Message) {
	weightOfSupporter, totalWeight, epochID := a.tangle.WeightProvider.Weight(message)

	a.weightsPerEpoch(branchID).RemoveWeight(epochID, weightOfSupporter/totalWeight)
	fmt.Println("onBranchSupportRemoved", message.ID(), branchID)
}

func (a *ApprovalWeightManager) weightOfLargestConflictingBranch(branchID ledgerstate.BranchID, epochID uint64) (weight float64) {
	a.tangle.LedgerState.BranchDAG.ForEachConflictingBranchID(branchID, func(conflictingBranchID ledgerstate.BranchID) {
		if branchWeight := a.weightsPerEpoch(conflictingBranchID).Weight(epochID); branchWeight > weight {
			weight = branchWeight
		}
	})

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ApprovalWeightManagerEvents //////////////////////////////////////////////////////////////////////////////////

type ApprovalWeightManagerEvents struct {
	MessageProcessed *events.Event
	BranchConfirmed  *events.Event
	MarkerConfirmed  *events.Event
}

func branchIDCaller(handler interface{}, params ...interface{}) {
	handler.(func(ledgerstate.BranchID))(params[0].(ledgerstate.BranchID))
}

func markerCaller(handler interface{}, params ...interface{}) {
	handler.(func(*markers.Marker))(params[0].(*markers.Marker))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchWeightPerEpoch /////////////////////////////////////////////////////////////////////////////////////////

type BranchWeightPerEpoch struct {
	weightPerEpoch      map[uint64]float64
	weightPerEpochMutex sync.RWMutex
}

func NewBranchWeightPerEpoch() *BranchWeightPerEpoch {
	return &BranchWeightPerEpoch{
		weightPerEpoch: make(map[uint64]float64),
	}
}

func (b *BranchWeightPerEpoch) Weight(epochID uint64) (totalWeight float64) {
	b.weightPerEpochMutex.RLock()
	defer b.weightPerEpochMutex.RUnlock()

	return b.weightPerEpoch[epochID]
}

func (b *BranchWeightPerEpoch) AddWeight(epochID uint64, weight float64) (totalWeight float64) {
	b.weightPerEpochMutex.Lock()
	defer b.weightPerEpochMutex.Unlock()

	totalWeight = b.weightPerEpoch[epochID] + weight
	b.weightPerEpoch[epochID] = totalWeight

	return
}

func (b *BranchWeightPerEpoch) RemoveWeight(epochID uint64, weight float64) (totalWeight float64) {
	b.weightPerEpochMutex.Lock()
	defer b.weightPerEpochMutex.Unlock()

	totalWeight = b.weightPerEpoch[epochID] - weight
	b.weightPerEpoch[epochID] = totalWeight

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SupporterManager /////////////////////////////////////////////////////////////////////////////////////////////

type SupporterManager struct {
	Events           *SupporterManagerEvents
	tangle           *Tangle
	branchSupporters map[ledgerstate.BranchID]*Supporters
}

func NewSupporterManager(tangle *Tangle) (supporterManager *SupporterManager) {
	supporterManager = &SupporterManager{
		Events: &SupporterManagerEvents{
			BranchSupportAdded:     events.NewEvent(branchSupportEventCaller),
			BranchSupportRemoved:   events.NewEvent(branchSupportEventCaller),
			SequenceSupportUpdated: events.NewEvent(sequenceSupportEventCaller),
		},
		tangle:           tangle,
		branchSupporters: make(map[ledgerstate.BranchID]*Supporters),
	}

	return
}

func (s *SupporterManager) ProcessMessage(messageID MessageID) {
	s.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		s.updateBranchSupporters(message)
		s.updateSequenceSupporters(message)
	})
}

func (s *SupporterManager) SupportersOfBranch(branchID ledgerstate.BranchID) (supporters *Supporters) {
	return s.branchSupporters[branchID]
}

func (s *SupporterManager) SupportersOfMarker(marker *markers.Marker) (supporters *Supporters) {
	if !s.tangle.Storage.SequenceSupporters(marker.SequenceID()).Consume(func(sequenceSupporters *SequenceSupporters) {
		supporters = sequenceSupporters.Supporters(marker.Index())
	}) {
		supporters = NewSupporters()
	}

	return
}

func (s *SupporterManager) MigrateMarkerSupportersToNewBranch(marker *markers.Marker, oldBranchID, newBranchID ledgerstate.BranchID) {
	supporters, exists := s.branchSupporters[newBranchID]
	if !exists {
		supporters = NewSupporters()
		s.branchSupporters[newBranchID] = supporters
	}

	supportersOfMarker := s.SupportersOfMarker(marker)
	if oldBranchID == ledgerstate.MasterBranchID {
		supportersOfMarker.ForEach(func(supporter Supporter) {
			supporters.Add(supporter)
		})
	}

	s.SupportersOfBranch(oldBranchID).ForEach(func(supporter Supporter) {
		if supportersOfMarker.Has(supporter) {
			supporters.Add(supporter)
		}
	})
}

func (s *SupporterManager) updateSequenceSupporters(message *Message) {
	s.tangle.Storage.MessageMetadata(message.ID()).Consume(func(messageMetadata *MessageMetadata) {
		supportWalker := walker.New()

		messageMetadata.StructureDetails().PastMarkers.ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
			supportWalker.Push(markers.NewMarker(sequenceID, index))

			return true
		})

		for supportWalker.HasNext() {
			s.addSupportToMarker(supportWalker.Next().(*markers.Marker), message, supportWalker)
		}
	})
}

func (s *SupporterManager) addSupportToMarker(marker *markers.Marker, message *Message, walker *walker.Walker) {
	// Avoid tracking support of markers in sequence 0.
	if marker.SequenceID() == 0 {
		return
	}

	s.tangle.Storage.SequenceSupporters(marker.SequenceID(), func() *SequenceSupporters {
		return NewSequenceSupporters(marker.SequenceID())
	}).Consume(func(sequenceSupporters *SequenceSupporters) {
		if sequenceSupporters.AddSupporter(identity.NewID(message.IssuerPublicKey()), marker.Index()) {
			s.Events.SequenceSupportUpdated.Trigger(marker, message)

			s.tangle.Booker.MarkersManager.Manager.Sequence(marker.SequenceID()).Consume(func(sequence *markers.Sequence) {
				sequence.ReferencedMarkers(marker.Index()).ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
					// Avoid adding and tracking support of markers in sequence 0.
					if sequenceID == 0 {
						return true
					}

					walker.Push(markers.NewMarker(sequenceID, index))

					return true
				})
			})
		}
	})
}

func (s *SupporterManager) updateBranchSupporters(message *Message) {
	statement, isNewStatement := s.statementFromMessage(message)
	if !isNewStatement {
		return
	}

	s.propagateSupportToBranches(statement.BranchID(), message)
}

func (s *SupporterManager) propagateSupportToBranches(branchID ledgerstate.BranchID, message *Message) {
	conflictBranchIDs, err := s.tangle.LedgerState.BranchDAG.ResolveConflictBranchIDs(ledgerstate.NewBranchIDs(branchID))
	if err != nil {
		panic(err)
	}

	supportWalker := walker.New()
	for conflictBranchID := range conflictBranchIDs {
		supportWalker.Push(conflictBranchID)
	}

	for supportWalker.HasNext() {
		s.addSupportToBranch(supportWalker.Next().(ledgerstate.BranchID), message, supportWalker)
	}
}

func (s *SupporterManager) isRelevantSupporter(message *Message) bool {
	supporterWeight, totalWeight, _ := s.tangle.WeightProvider.Weight(message)

	return supporterWeight/totalWeight >= lowerWeightThreshold
}

func (s *SupporterManager) addSupportToBranch(branchID ledgerstate.BranchID, message *Message, walk *walker.Walker) {
	if branchID == ledgerstate.MasterBranchID || !s.isRelevantSupporter(message) {
		return
	}

	if _, isNewStatement := s.statementFromMessage(message, branchID); !isNewStatement {
		return
	}

	if _, exists := s.branchSupporters[branchID]; !exists {
		s.branchSupporters[branchID] = NewSupporters()
	}

	if !s.branchSupporters[branchID].Add(identity.NewID(message.IssuerPublicKey())) {
		return
	}

	s.tangle.LedgerState.BranchDAG.ForEachConflictingBranchID(branchID, func(conflictingBranchID ledgerstate.BranchID) {
		revokeWalker := walker.New()
		revokeWalker.Push(conflictingBranchID)

		for revokeWalker.HasNext() {
			s.revokeSupportFromBranch(revokeWalker.Next().(ledgerstate.BranchID), message, revokeWalker)
		}
	})

	s.Events.BranchSupportAdded.Trigger(branchID, message)

	s.tangle.LedgerState.BranchDAG.Branch(branchID).Consume(func(branch ledgerstate.Branch) {
		for parentBranchID := range branch.Parents() {
			walk.Push(parentBranchID)
		}
	})
}

func (s *SupporterManager) revokeSupportFromBranch(branchID ledgerstate.BranchID, message *Message, walker *walker.Walker) {
	if _, isNewStatement := s.statementFromMessage(message, branchID); !isNewStatement {
		return
	}

	if supporters, exists := s.branchSupporters[branchID]; !exists || !supporters.Delete(identity.NewID(message.IssuerPublicKey())) {
		return
	}

	s.Events.BranchSupportRemoved.Trigger(branchID, message)

	s.tangle.LedgerState.BranchDAG.ChildBranches(branchID).Consume(func(childBranch *ledgerstate.ChildBranch) {
		if childBranch.ChildBranchType() != ledgerstate.ConflictBranchType {
			return
		}

		walker.Push(childBranch.ChildBranchID())
	})
}

func (s *SupporterManager) statementFromMessage(message *Message, optionalBranchID ...ledgerstate.BranchID) (statement *Statement, isNewStatement bool) {
	nodeID := identity.NewID(message.IssuerPublicKey())

	var branchID ledgerstate.BranchID
	if len(optionalBranchID) > 0 {
		branchID = optionalBranchID[0]
	} else {
		var err error
		branchID, err = s.tangle.Booker.MessageBranchID(message.ID())
		if err != nil {
			// TODO: handle error properly
			panic(err)
		}
	}

	s.tangle.Storage.Statement(branchID, nodeID, func() *Statement {
		return NewStatement(branchID, nodeID)
	}).Consume(func(consumedStatement *Statement) {
		statement = consumedStatement
		// We already have a newer statement for this branchID of this supporter.
		if !statement.UpdateSequenceNumber(message.SequenceNumber()) {
			return
		}

		isNewStatement = true
	})

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SupporterManagerEvents ///////////////////////////////////////////////////////////////////////////////////////

type SupporterManagerEvents struct {
	BranchSupportAdded     *events.Event
	BranchSupportRemoved   *events.Event
	SequenceSupportUpdated *events.Event
}

func branchSupportEventCaller(handler interface{}, params ...interface{}) {
	handler.(func(ledgerstate.BranchID, *Message))(params[0].(ledgerstate.BranchID), params[1].(*Message))
}

func sequenceSupportEventCaller(handler interface{}, params ...interface{}) {
	handler.(func(*markers.Marker, *Message))(params[0].(*markers.Marker), params[1].(*Message))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Statement ////////////////////////////////////////////////////////////////////////////////////////////////////

// Statement is a data structure that tracks the latest statement by a Supporter per ledgerstate.BranchID.
type Statement struct {
	branchID       ledgerstate.BranchID
	supporter      Supporter
	sequenceNumber uint64

	sequenceNumberMutex sync.RWMutex

	objectstorage.StorableObjectFlags
}

// NewStatement creates a new Statement
func NewStatement(branchID ledgerstate.BranchID, supporter Supporter) (statement *Statement) {
	statement = &Statement{
		branchID:  branchID,
		supporter: supporter,
	}

	statement.Persist()
	statement.SetModified()

	return
}

// StatementFromBytes unmarshals a SequenceSupporters object from a sequence of bytes.
func StatementFromBytes(bytes []byte) (statement *Statement, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if statement, err = StatementFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse SequenceSupporters from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// StatementFromMarshalUtil unmarshals a Statement object using a MarshalUtil (for easier unmarshaling).
func StatementFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (statement *Statement, err error) {
	statement = &Statement{}
	if statement.branchID, err = ledgerstate.BranchIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse BranchID from MarshalUtil: %w", err)
		return
	}

	if statement.supporter, err = identity.IDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Supporter from MarshalUtil: %w", err)
		return
	}

	if statement.sequenceNumber, err = marshalUtil.ReadUint64(); err != nil {
		err = xerrors.Errorf("failed to parse sequence number (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// StatementFromObjectStorage restores a Statement object from the object storage.
func StatementFromObjectStorage(key []byte, data []byte) (result objectstorage.StorableObject, err error) {
	if result, _, err = StatementFromBytes(byteutils.ConcatBytes(key, data)); err != nil {
		err = xerrors.Errorf("failed to parse Statement from bytes: %w", err)
		return
	}

	return
}

// BranchID returns the ledgerstate.BranchID that is being tracked.
func (s *Statement) BranchID() (branchID ledgerstate.BranchID) {
	return s.branchID
}

// Supporter returns the Supporter that is being tracked.
func (s *Statement) Supporter() (supporter Supporter) {
	return s.supporter
}

// UpdateSequenceNumber updates the sequence number of the Statement if it is greater than the currently stored
// sequence number and returns true if it was updated.
func (s *Statement) UpdateSequenceNumber(sequenceNumber uint64) (updated bool) {
	s.sequenceNumberMutex.Lock()
	defer s.sequenceNumberMutex.Unlock()

	if s.sequenceNumber > sequenceNumber {
		return false
	}

	s.sequenceNumber = sequenceNumber
	updated = true
	s.SetModified()

	return
}

// SequenceNumber returns the sequence number of the Statement.
func (s *Statement) SequenceNumber() (sequenceNumber uint64) {
	s.sequenceNumberMutex.RLock()
	defer s.sequenceNumberMutex.RUnlock()

	return sequenceNumber
}

// Bytes returns a marshaled version of the Statement.
func (s *Statement) Bytes() (marshaledSequenceSupporters []byte) {
	return byteutils.ConcatBytes(s.ObjectStorageKey(), s.ObjectStorageValue())
}

// String returns a human readable version of the Statement.
func (s *Statement) String() string {
	return stringify.Struct("Statement",
		stringify.StructField("branchID", s.BranchID()),
		stringify.StructField("supporter", s.Supporter()),
		stringify.StructField("sequenceNumber", s.SequenceNumber()),
	)
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (s *Statement) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (s *Statement) ObjectStorageKey() []byte {
	return byteutils.ConcatBytes(s.BranchID().Bytes(), s.Supporter().Bytes())
}

// ObjectStorageValue marshals the Statement into a sequence of bytes that are used as the value part in the
// object storage.
func (s *Statement) ObjectStorageValue() []byte {
	return marshalutil.New(marshalutil.Uint64Size).
		WriteUint64(s.SequenceNumber()).
		Bytes()
}

// code contract (make sure the struct implements all required methods)
var _ objectstorage.StorableObject = &Statement{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedStatement /////////////////////////////////////////////////////////////////////////////////////

// CachedStatement is a wrapper for the generic CachedObject returned by the object storage that overrides the
// accessor methods with a type-casted one.
type CachedStatement struct {
	objectstorage.CachedObject
}

// Retain marks the CachedObject to still be in use by the program.
func (c *CachedStatement) Retain() *CachedStatement {
	return &CachedStatement{c.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (c *CachedStatement) Unwrap() *Statement {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*Statement)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (c *CachedStatement) Consume(consumer func(statement *Statement), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Statement))
	}, forceRelease...)
}

// String returns a human readable version of the CachedStatement.
func (c *CachedStatement) String() string {
	return stringify.Struct("CachedStatement",
		stringify.StructField("CachedObject", c.Unwrap()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Supporter ////////////////////////////////////////////////////////////////////////////////////////////////////

type Supporter = identity.ID

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Supporters ///////////////////////////////////////////////////////////////////////////////////////////////////

// Supporters is a set of node identities that votes for a particular Branch.
type Supporters struct {
	set.Set
}

// NewSupporters is the constructor of the Supporters type.
func NewSupporters() (supporters *Supporters) {
	return &Supporters{
		Set: set.New(),
	}
}

// Add adds a new Supporter to the Set and returns true if the Supporter was not present in the set before.
func (s *Supporters) Add(supporter Supporter) (added bool) {
	return s.Set.Add(supporter)
}

// Delete removes the Supporter from the Set and returns true if it did exist.
func (s *Supporters) Delete(supporter Supporter) (deleted bool) {
	return s.Set.Delete(supporter)
}

// Has returns true if the Supporter exists in the Set.
func (s *Supporters) Has(supporter Supporter) (has bool) {
	return s.Set.Has(supporter)
}

// ForEach iterates through the Supporters and calls the callback for every element.
func (s *Supporters) ForEach(callback func(supporter Supporter)) {
	s.Set.ForEach(func(element interface{}) {
		callback(element.(Supporter))
	})
}

// String returns a human readable version of the Supporters.
func (s *Supporters) String() string {
	structBuilder := stringify.StructBuilder("Supporters")
	s.ForEach(func(supporter Supporter) {
		structBuilder.AddField(stringify.StructField(supporter.String(), "true"))
	})

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SequenceSupporters ///////////////////////////////////////////////////////////////////////////////////////////

// SequenceSupporters is a data structure that tracks which nodes have confirmed which Index in a given Sequence.
type SequenceSupporters struct {
	sequenceID              markers.SequenceID
	supportersPerIndex      map[Supporter]markers.Index
	supportersPerIndexMutex sync.RWMutex

	objectstorage.StorableObjectFlags
}

// NewSequenceSupporters is the constructor for the SequenceSupporters object.
func NewSequenceSupporters(sequenceID markers.SequenceID) (sequenceSupporters *SequenceSupporters) {
	sequenceSupporters = &SequenceSupporters{
		sequenceID:         sequenceID,
		supportersPerIndex: make(map[Supporter]markers.Index),
	}

	sequenceSupporters.Persist()
	sequenceSupporters.SetModified()

	return
}

// SequenceSupportersFromBytes unmarshals a SequenceSupporters object from a sequence of bytes.
func SequenceSupportersFromBytes(bytes []byte) (sequenceSupporters *SequenceSupporters, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if sequenceSupporters, err = SequenceSupportersFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse SequenceSupporters from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// SequenceSupportersFromMarshalUtil unmarshals a SequenceSupporters object using a MarshalUtil (for easier unmarshaling).
func SequenceSupportersFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (sequenceSupporters *SequenceSupporters, err error) {
	sequenceSupporters = &SequenceSupporters{}
	if sequenceSupporters.sequenceID, err = markers.SequenceIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse SequenceID from MarshalUtil: %w", err)
		return
	}
	supportersCount, err := marshalUtil.ReadUint64()
	if err != nil {
		err = xerrors.Errorf("failed to parse supporters count (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	sequenceSupporters.supportersPerIndex = make(map[Supporter]markers.Index)
	for i := uint64(0); i < supportersCount; i++ {
		supporter, supporterErr := identity.IDFromMarshalUtil(marshalUtil)
		if supporterErr != nil {
			err = xerrors.Errorf("failed to parse Supporter (%v): %w", supporterErr, cerrors.ErrParseBytesFailed)
			return
		}

		index, indexErr := markers.IndexFromMarshalUtil(marshalUtil)
		if indexErr != nil {
			err = xerrors.Errorf("failed to parse Index: %w", indexErr)
			return
		}

		sequenceSupporters.supportersPerIndex[supporter] = index
	}

	return
}

// SequenceSupportersFromObjectStorage restores a SequenceSupporters object from the object storage.
func SequenceSupportersFromObjectStorage(key []byte, data []byte) (result objectstorage.StorableObject, err error) {
	if result, _, err = SequenceSupportersFromBytes(byteutils.ConcatBytes(key, data)); err != nil {
		err = xerrors.Errorf("failed to parse SequenceSupporters from bytes: %w", err)
		return
	}

	return
}

// SequenceID returns the SequenceID that is being tracked.
func (s *SequenceSupporters) SequenceID() (sequenceID markers.SequenceID) {
	return s.sequenceID
}

// AddSupporter adds a new Supporter of a given index to the Sequence.
func (s *SequenceSupporters) AddSupporter(supporter Supporter, index markers.Index) (added bool) {
	s.supportersPerIndexMutex.Lock()
	defer s.supportersPerIndexMutex.Unlock()

	highestApprovedIndex, exists := s.supportersPerIndex[supporter]

	if added = !exists || highestApprovedIndex < index; !added {
		return
	}
	s.SetModified()

	s.supportersPerIndex[supporter] = index

	return
}

// Supporters returns the set of Supporters that have approved the given Index.
func (s *SequenceSupporters) Supporters(index markers.Index) (supporters *Supporters) {
	s.supportersPerIndexMutex.RLock()
	defer s.supportersPerIndexMutex.RUnlock()

	supporters = NewSupporters()
	for supporter, supportedIndex := range s.supportersPerIndex {
		if supportedIndex >= index {
			supporters.Add(supporter)
		}
	}

	return
}

// Bytes returns a marshaled version of the SequenceSupporters.
func (s *SequenceSupporters) Bytes() (marshaledSequenceSupporters []byte) {
	return byteutils.ConcatBytes(s.ObjectStorageKey(), s.ObjectStorageValue())
}

// String returns a human readable version of the SequenceSupporters.
func (s *SequenceSupporters) String() string {
	structBuilder := stringify.StructBuilder("SequenceSupporters",
		stringify.StructField("sequenceID", s.SequenceID()),
	)

	s.supportersPerIndexMutex.RLock()
	defer s.supportersPerIndexMutex.RUnlock()
	for supporter, index := range s.supportersPerIndex {
		stringify.StructField(supporter.String(), index)
	}

	return structBuilder.String()
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (s *SequenceSupporters) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (s *SequenceSupporters) ObjectStorageKey() []byte {
	return s.sequenceID.Bytes()
}

// ObjectStorageValue marshals the SequenceSupporters into a sequence of bytes that are used as the value part in the
// object storage.
func (s *SequenceSupporters) ObjectStorageValue() []byte {
	s.supportersPerIndexMutex.RLock()
	defer s.supportersPerIndexMutex.RUnlock()

	marshalUtil := marshalutil.New(marshalutil.Uint64Size + len(s.supportersPerIndex)*(ed25519.PublicKeySize+marshalutil.Uint64Size))
	marshalUtil.WriteUint64(uint64(len(s.supportersPerIndex)))
	for supporter, index := range s.supportersPerIndex {
		marshalUtil.WriteBytes(supporter.Bytes())
		marshalUtil.WriteUint64(uint64(index))
	}

	return marshalUtil.Bytes()
}

// code contract (make sure the struct implements all required methods)
var _ objectstorage.StorableObject = &SequenceSupporters{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedSequenceSupporters /////////////////////////////////////////////////////////////////////////////////////

// CachedSequenceSupporters is a wrapper for the generic CachedObject returned by the object storage that overrides the
// accessor methods with a type-casted one.
type CachedSequenceSupporters struct {
	objectstorage.CachedObject
}

// Retain marks the CachedObject to still be in use by the program.
func (c *CachedSequenceSupporters) Retain() *CachedSequenceSupporters {
	return &CachedSequenceSupporters{c.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (c *CachedSequenceSupporters) Unwrap() *SequenceSupporters {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*SequenceSupporters)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (c *CachedSequenceSupporters) Consume(consumer func(sequenceSupporters *SequenceSupporters), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*SequenceSupporters))
	}, forceRelease...)
}

// String returns a human readable version of the CachedSequenceSupporters.
func (c *CachedSequenceSupporters) String() string {
	return stringify.Struct("CachedSequenceSupporters",
		stringify.StructField("CachedObject", c.Unwrap()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
