package tangle

import (
	"fmt"
	"sync"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/datastructure/set"
	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/xerrors"
)

// region ApprovalWeightManager ////////////////////////////////////////////////////////////////////////////////////////

type ApprovalWeightManager struct {
	tangle           *Tangle
	lastStatements   map[Supporter]*Statement
	branchSupporters map[ledgerstate.BranchID]*Supporters
}

func NewApprovalWeightManager(tangle *Tangle) (approvalWeightManager *ApprovalWeightManager) {
	approvalWeightManager = &ApprovalWeightManager{
		tangle:           tangle,
		lastStatements:   make(map[Supporter]*Statement),
		branchSupporters: make(map[ledgerstate.BranchID]*Supporters),
	}

	return
}

func (a *ApprovalWeightManager) ProcessMessage(messageID MessageID) {
	a.tangle.Storage.Message(messageID).Consume(func(message *Message) {
		a.updateBranchSupporters(message)
		a.updateSequenceSupporters(message)
	})
}

func (a *ApprovalWeightManager) SupportersOfMarker(marker *markers.Marker) (supporters *Supporters) {
	if !a.tangle.Storage.SequenceSupporters(marker.SequenceID()).Consume(func(sequenceSupporters *SequenceSupporters) {
		supporters = sequenceSupporters.Supporters(marker.Index())
	}) {
		supporters = NewSupporters()
	}

	return
}

func (a *ApprovalWeightManager) updateSequenceSupporters(message *Message) {
	a.tangle.Storage.MessageMetadata(message.ID()).Consume(func(messageMetadata *MessageMetadata) {
		supportWalker := walker.New()

		messageMetadata.StructureDetails().PastMarkers.ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
			supportWalker.Push(markers.NewMarker(sequenceID, index))

			return true
		})

		for supportWalker.HasNext() {
			a.addSupportToMarker(supportWalker.Next().(*markers.Marker), message.IssuerPublicKey(), supportWalker)
		}
	})
}

func (a *ApprovalWeightManager) addSupportToMarker(marker *markers.Marker, supporter Supporter, walker *walker.Walker) {
	a.tangle.Storage.SequenceSupporters(marker.SequenceID(), func() *SequenceSupporters {
		return NewSequenceSupporters(marker.SequenceID())
	}).Consume(func(sequenceSupporters *SequenceSupporters) {
		if sequenceSupporters.AddSupporter(supporter, marker.Index()) {
			a.tangle.Booker.MarkersManager.Manager.Sequence(marker.SequenceID()).Consume(func(sequence *markers.Sequence) {
				sequence.ParentReferences().HighestReferencedMarkers(marker.Index()).ForEach(func(sequenceID markers.SequenceID, index markers.Index) bool {
					walker.Push(markers.NewMarker(sequenceID, index))

					return true
				})
			})
		}
	})
}

func (a *ApprovalWeightManager) updateBranchSupporters(message *Message) {
	lastStatement, lastStatementExists := a.lastStatements[message.IssuerPublicKey()]
	if lastStatementExists && !a.isNewStatement(lastStatement, message) {
		return
	}

	newStatement := a.statementFromMessage(message)
	a.lastStatements[message.IssuerPublicKey()] = newStatement

	if lastStatementExists && lastStatement.BranchID == newStatement.BranchID {
		return
	}

	a.propagateSupportToBranches(newStatement.BranchID, message.IssuerPublicKey())
}

func (a *ApprovalWeightManager) propagateSupportToBranches(branchID ledgerstate.BranchID, issuer Supporter) {
	conflictBranchIDs, err := a.tangle.LedgerState.BranchDAG.ResolveConflictBranchIDs(ledgerstate.NewBranchIDs(branchID))
	if err != nil {
		panic(err)
	}

	supportWalker := walker.New()
	for conflictBranchID := range conflictBranchIDs {
		supportWalker.Push(conflictBranchID)
	}

	for supportWalker.HasNext() {
		a.addSupportToBranch(supportWalker.Next().(ledgerstate.BranchID), issuer, supportWalker)
	}
}

func (a *ApprovalWeightManager) addSupportToBranch(branchID ledgerstate.BranchID, issuer Supporter, walk *walker.Walker) {
	if _, exists := a.branchSupporters[branchID]; !exists {
		a.branchSupporters[branchID] = NewSupporters()
	}

	if !a.branchSupporters[branchID].Add(issuer) {
		return
	}

	a.tangle.LedgerState.BranchDAG.ForEachConflictingBranchID(branchID, func(conflictingBranchID ledgerstate.BranchID) {
		revokeWalker := walker.New()
		revokeWalker.Push(conflictingBranchID)

		for revokeWalker.HasNext() {
			a.revokeSupportFromBranch(revokeWalker.Next().(ledgerstate.BranchID), issuer, revokeWalker)
		}
	})

	a.tangle.LedgerState.BranchDAG.Branch(branchID).Consume(func(branch ledgerstate.Branch) {
		for parentBranchID := range branch.Parents() {
			walk.Push(parentBranchID)
		}
	})
}

func (a *ApprovalWeightManager) revokeSupportFromBranch(branchID ledgerstate.BranchID, issuer Supporter, walker *walker.Walker) {
	if supporters, exists := a.branchSupporters[branchID]; !exists || !supporters.Delete(issuer) {
		return
	}

	a.tangle.LedgerState.BranchDAG.ChildBranches(branchID).Consume(func(childBranch *ledgerstate.ChildBranch) {
		if childBranch.ChildBranchType() != ledgerstate.ConflictBranchType {
			return
		}

		walker.Push(childBranch.ChildBranchID())
	})
}

func (a *ApprovalWeightManager) isNewStatement(lastStatement *Statement, message *Message) bool {
	return lastStatement.SequenceNumber < message.SequenceNumber()
}

func (a *ApprovalWeightManager) statementFromMessage(message *Message) (statement *Statement) {
	if !a.tangle.Storage.MessageMetadata(message.ID()).Consume(func(messageMetadata *MessageMetadata) {
		statement = &Statement{
			SequenceNumber: message.SequenceNumber(),
			BranchID:       messageMetadata.BranchID(),
		}

	}) {
		panic(fmt.Errorf("failed to load MessageMetadata of Message with %s", message.ID()))
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Statement ////////////////////////////////////////////////////////////////////////////////////////////////////

type Statement struct {
	SequenceNumber uint64
	BranchID       ledgerstate.BranchID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Supporter ////////////////////////////////////////////////////////////////////////////////////////////////////

type Supporter = ed25519.PublicKey

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Supporters ///////////////////////////////////////////////////////////////////////////////////////////////////

type Supporters struct {
	set.Set
}

func NewSupporters() (supporters *Supporters) {
	return &Supporters{
		Set: set.New(),
	}
}

func (s *Supporters) Add(supporter Supporter) (added bool) {
	return s.Set.Add(supporter)
}

func (s *Supporters) Delete(supporter Supporter) (deleted bool) {
	return s.Set.Delete(supporter)
}

func (s *Supporters) Has(supporter Supporter) (has bool) {
	return s.Set.Has(supporter)
}

func (s *Supporters) ForEach(callback func(supporter Supporter)) {
	s.Set.ForEach(func(element interface{}) {
		callback(element.(Supporter))
	})
}

func (s *Supporters) String() string {
	structBuilder := stringify.StructBuilder("Supporters")
	s.ForEach(func(supporter Supporter) {
		structBuilder.AddField(stringify.StructField(supporter.String(), "true"))
	})

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SequenceSupporters /////////////////////////////////////////////////////////////////////////////////////////////

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
		supporter, supporterErr := ed25519.ParsePublicKey(marshalUtil)
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

// Bytes returns a marshaled version of the MessageMetadata.
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

// ObjectStorageValue marshals the MessageMetadata into a sequence of bytes that are used as the value part in the
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
