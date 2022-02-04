package tangle

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/datastructure/set"
	"github.com/iotaledger/hive.go/datastructure/thresholdmap"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
)

// region ApprovalWeightManager Models /////////////////////////////////////////////////////////////////////////////////

// region BranchWeight /////////////////////////////////////////////////////////////////////////////////////////////////

// BranchWeight is a data structure that tracks the weight of a BranchID.
type BranchWeight struct {
	branchID ledgerstate.BranchID
	weight   float64

	weightMutex sync.RWMutex

	objectstorage.StorableObjectFlags
}

// NewBranchWeight creates a new BranchWeight.
func NewBranchWeight(branchID ledgerstate.BranchID) (branchWeight *BranchWeight) {
	branchWeight = &BranchWeight{
		branchID: branchID,
	}

	branchWeight.Persist()
	branchWeight.SetModified()

	return
}

// BranchWeightFromBytes unmarshals a BranchWeight object from a sequence of bytes.
func BranchWeightFromBytes(bytes []byte) (branchWeight *BranchWeight, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if branchWeight, err = BranchWeightFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse BranchWeight from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// BranchWeightFromMarshalUtil unmarshals a BranchWeight object using a MarshalUtil (for easier unmarshalling).
func BranchWeightFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (branchWeight *BranchWeight, err error) {
	branchWeight = &BranchWeight{}
	if branchWeight.branchID, err = ledgerstate.BranchIDFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse BranchID from MarshalUtil: %w", err)
		return
	}

	if branchWeight.weight, err = marshalUtil.ReadFloat64(); err != nil {
		err = errors.Errorf("failed to parse weight (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// BranchWeightFromObjectStorage restores a BranchWeight object from the object storage.
func BranchWeightFromObjectStorage(key, data []byte) (result objectstorage.StorableObject, err error) {
	if result, _, err = BranchWeightFromBytes(byteutils.ConcatBytes(key, data)); err != nil {
		err = errors.Errorf("failed to parse BranchWeight from bytes: %w", err)
		return
	}

	return
}

// BranchID returns the BranchID that is being tracked.
func (b *BranchWeight) BranchID() (branchID ledgerstate.BranchID) {
	return b.branchID
}

// Weight returns the weight of the BranchID.
func (b *BranchWeight) Weight() (weight float64) {
	b.weightMutex.RLock()
	defer b.weightMutex.RUnlock()

	return b.weight
}

// SetWeight sets the weight for the BranchID and returns true if it was modified.
func (b *BranchWeight) SetWeight(weight float64) (modified bool) {
	b.weightMutex.Lock()
	defer b.weightMutex.Unlock()

	if weight == b.weight {
		return false
	}

	b.weight = weight
	modified = true
	b.SetModified()

	return
}

// Bytes returns a marshaled version of the BranchWeight.
func (b *BranchWeight) Bytes() (marshaledBranchWeight []byte) {
	return byteutils.ConcatBytes(b.ObjectStorageKey(), b.ObjectStorageValue())
}

// String returns a human-readable version of the BranchWeight.
func (b *BranchWeight) String() string {
	return stringify.Struct("BranchWeight",
		stringify.StructField("branchID", b.BranchID()),
		stringify.StructField("weight", b.Weight()),
	)
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (b *BranchWeight) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (b *BranchWeight) ObjectStorageKey() []byte {
	return b.BranchID().Bytes()
}

// ObjectStorageValue marshals the BranchWeight into a sequence of bytes that are used as the value part in the
// object storage.
func (b *BranchWeight) ObjectStorageValue() []byte {
	return marshalutil.New(marshalutil.Float64Size).
		WriteFloat64(b.Weight()).
		Bytes()
}

// code contract (make sure the struct implements all required methods).
var _ objectstorage.StorableObject = &BranchWeight{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedBranchWeight ///////////////////////////////////////////////////////////////////////////////////////////

// CachedBranchWeight is a wrapper for the generic CachedObject returned by the object storage that overrides the
// accessor methods with a type-casted one.
type CachedBranchWeight struct {
	objectstorage.CachedObject
}

// Retain marks the CachedObject to still be in use by the program.
func (c *CachedBranchWeight) Retain() *CachedBranchWeight {
	return &CachedBranchWeight{c.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (c *CachedBranchWeight) Unwrap() *BranchWeight {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*BranchWeight)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (c *CachedBranchWeight) Consume(consumer func(branchWeight *BranchWeight), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*BranchWeight))
	}, forceRelease...)
}

// String returns a human-readable version of the CachedBranchWeight.
func (c *CachedBranchWeight) String() string {
	return stringify.Struct("CachedBranchWeight",
		stringify.StructField("CachedObject", c.Unwrap()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Voter ////////////////////////////////////////////////////////////////////////////////////////////////////////

// Voter is a type wrapper for identity.ID and defines a node that supports a branch or marker.
type Voter = identity.ID

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////////

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
func (s *Supporters) Add(supporter Voter) (added bool) {
	return s.Set.Add(supporter)
}

// AddAll adds all new Supporters to the Set.
func (s *Supporters) AddAll(supporters *Supporters) {
	supporters.ForEach(func(supporter Voter) {
		s.Set.Add(supporter)
	})
}

// Delete removes the Supporter from the Set and returns true if it did exist.
func (s *Supporters) Delete(supporter Voter) (deleted bool) {
	return s.Set.Delete(supporter)
}

// Has returns true if the Supporter exists in the Set.
func (s *Supporters) Has(supporter Voter) (has bool) {
	return s.Set.Has(supporter)
}

// ForEach iterates through the Supporters and calls the callback for every element.
func (s *Supporters) ForEach(callback func(supporter Voter)) {
	s.Set.ForEach(func(element interface{}) {
		callback(element.(Voter))
	})
}

// Clone returns a copy of the Supporters.
func (s *Supporters) Clone() (clonedSupporters *Supporters) {
	clonedSupporters = NewSupporters()
	s.ForEach(func(supporter Voter) {
		clonedSupporters.Add(supporter)
	})

	return
}

// Intersect creates an intersection of two set of Supporters.
func (s *Supporters) Intersect(other *Supporters) (intersection *Supporters) {
	intersection = NewSupporters()
	s.ForEach(func(supporter Voter) {
		if other.Has(supporter) {
			intersection.Add(supporter)
		}
	})

	return
}

// String returns a human-readable version of the Supporters.
func (s *Supporters) String() string {
	structBuilder := stringify.StructBuilder("Supporters")
	s.ForEach(func(supporter Voter) {
		structBuilder.AddField(stringify.StructField(supporter.String(), "true"))
	})

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchSupporters /////////////////////////////////////////////////////////////////////////////////////////////

// BranchSupporters is a data structure that tracks which nodes support a branch.
type BranchSupporters struct {
	branchID   ledgerstate.BranchID
	supporters *Supporters

	supportersMutex sync.RWMutex

	objectstorage.StorableObjectFlags
}

// NewBranchSupporters is the constructor for the BranchSupporters object.
func NewBranchSupporters(branchID ledgerstate.BranchID) (branchSupporters *BranchSupporters) {
	branchSupporters = &BranchSupporters{
		branchID:   branchID,
		supporters: NewSupporters(),
	}

	branchSupporters.Persist()
	branchSupporters.SetModified()

	return
}

// BranchSupportersFromBytes unmarshals a BranchSupporters object from a sequence of bytes.
func BranchSupportersFromBytes(bytes []byte) (branchSupporters *BranchSupporters, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if branchSupporters, err = BranchSupportersFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse SequenceSupporters from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// BranchSupportersFromMarshalUtil unmarshals a BranchSupporters object using a MarshalUtil (for easier unmarshalling).
func BranchSupportersFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (branchSupporters *BranchSupporters, err error) {
	branchSupporters = &BranchSupporters{}
	if branchSupporters.branchID, err = ledgerstate.BranchIDFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse BranchID from MarshalUtil: %w", err)
		return
	}

	supportersCount, err := marshalUtil.ReadUint64()
	if err != nil {
		err = errors.Errorf("failed to parse supporters count (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	branchSupporters.supporters = NewSupporters()
	for i := uint64(0); i < supportersCount; i++ {
		supporter, supporterErr := identity.IDFromMarshalUtil(marshalUtil)
		if supporterErr != nil {
			err = errors.Errorf("failed to parse Supporter (%v): %w", supporterErr, cerrors.ErrParseBytesFailed)
			return
		}

		branchSupporters.supporters.Add(supporter)
	}

	return
}

// BranchSupportersFromObjectStorage restores a BranchSupporters object from the object storage.
func BranchSupportersFromObjectStorage(key, data []byte) (result objectstorage.StorableObject, err error) {
	if result, _, err = BranchSupportersFromBytes(byteutils.ConcatBytes(key, data)); err != nil {
		err = errors.Errorf("failed to parse BranchSupporters from bytes: %w", err)
		return
	}

	return
}

// BranchID returns the BranchID that is being tracked.
func (b *BranchSupporters) BranchID() (branchID ledgerstate.BranchID) {
	return b.branchID
}

// Has returns true if the given Voter is currently supporting this Branch.
func (b *BranchSupporters) Has(voter Voter) bool {
	b.supportersMutex.RLock()
	defer b.supportersMutex.RUnlock()

	return b.supporters.Has(voter)
}

// AddSupporter adds a new Supporter to the tracked BranchID.
func (b *BranchSupporters) AddSupporter(supporter Voter) (added bool) {
	b.supportersMutex.Lock()
	defer b.supportersMutex.Unlock()

	if added = b.supporters.Add(supporter); !added {
		return
	}
	b.SetModified()

	return
}

// AddSupporters adds the supporters set to the tracked BranchID.
func (b *BranchSupporters) AddSupporters(supporters *Supporters) (added bool) {
	supporters.ForEach(func(supporter Voter) {
		if b.supporters.Add(supporter) {
			added = true
		}
	})

	if added {
		b.SetModified()
	}

	return
}

// DeleteSupporter deletes a Supporter from the tracked BranchID.
func (b *BranchSupporters) DeleteSupporter(supporter Voter) (deleted bool) {
	b.supportersMutex.Lock()
	defer b.supportersMutex.Unlock()

	if deleted = b.supporters.Delete(supporter); !deleted {
		return
	}
	b.SetModified()

	return
}

// Supporters returns the set of Supporters that are supporting the given BranchID.
func (b *BranchSupporters) Supporters() (supporters *Supporters) {
	b.supportersMutex.RLock()
	defer b.supportersMutex.RUnlock()

	return b.supporters.Clone()
}

// Bytes returns a marshaled version of the BranchSupporters.
func (b *BranchSupporters) Bytes() (marshaledBranchSupporters []byte) {
	return byteutils.ConcatBytes(b.ObjectStorageKey(), b.ObjectStorageValue())
}

// String returns a human-readable version of the BranchSupporters.
func (b *BranchSupporters) String() string {
	return stringify.Struct("BranchSupporters",
		stringify.StructField("branchID", b.BranchID()),
		stringify.StructField("supporters", b.Supporters()),
	)
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (b *BranchSupporters) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (b *BranchSupporters) ObjectStorageKey() []byte {
	return b.BranchID().Bytes()
}

// ObjectStorageValue marshals the BranchSupporters into a sequence of bytes that are used as the value part in the
// object storage.
func (b *BranchSupporters) ObjectStorageValue() []byte {
	b.supportersMutex.RLock()
	defer b.supportersMutex.RUnlock()

	marshalUtil := marshalutil.New(marshalutil.Uint64Size + b.supporters.Size()*identity.IDLength)
	marshalUtil.WriteUint64(uint64(b.supporters.Size()))

	b.supporters.ForEach(func(supporter Voter) {
		marshalUtil.WriteBytes(supporter.Bytes())
	})

	return marshalUtil.Bytes()
}

// code contract (make sure the struct implements all required methods).
var _ objectstorage.StorableObject = &BranchSupporters{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedBranchSupporters ///////////////////////////////////////////////////////////////////////////////////////

// CachedBranchSupporters is a wrapper for the generic CachedObject returned by the object storage that overrides the
// accessor methods with a type-casted one.
type CachedBranchSupporters struct {
	objectstorage.CachedObject
}

// Retain marks the CachedObject to still be in use by the program.
func (c *CachedBranchSupporters) Retain() *CachedBranchSupporters {
	return &CachedBranchSupporters{c.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (c *CachedBranchSupporters) Unwrap() *BranchSupporters {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*BranchSupporters)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (c *CachedBranchSupporters) Consume(consumer func(branchSupporters *BranchSupporters), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*BranchSupporters))
	}, forceRelease...)
}

// String returns a human-readable version of the CachedBranchSupporters.
func (c *CachedBranchSupporters) String() string {
	return stringify.Struct("CachedBranchSupporters",
		stringify.StructField("CachedObject", c.Unwrap()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Opinion //////////////////////////////////////////////////////////////////////////////////////////////////////

// Opinion is a type that represents the Opinion of a node on a certain Branch.
type Opinion uint8

const (
	// UndefinedOpinion represents the zero value of the Opinion type.
	UndefinedOpinion Opinion = iota

	// Confirmed represents the Opinion that a given Branch is the winning one.
	Confirmed

	// Rejected represents the Opinion that a given Branch is the loosing one.
	Rejected
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region LatestMarkerVotes ////////////////////////////////////////////////////////////////////////////////////////////

// LatestMarkerVotesKeyPartition defines the partition of the storage key of the LastMarkerVotes model.
var LatestMarkerVotesKeyPartition = objectstorage.PartitionKey(markers.SequenceIDLength, identity.IDLength)

// LatestMarkerVotes represents the markers supported from a certain Voter.
type LatestMarkerVotes struct {
	sequenceID        markers.SequenceID
	voter             Voter
	latestMarkerVotes *thresholdmap.ThresholdMap

	sync.RWMutex
	objectstorage.StorableObjectFlags
}

// NewLatestMarkerVotes creates a new NewLatestMarkerVotes instance associated with the given details.
func NewLatestMarkerVotes(sequenceID markers.SequenceID, voter Voter) (newLatestMarkerVotes *LatestMarkerVotes) {
	newLatestMarkerVotes = &LatestMarkerVotes{
		sequenceID:        sequenceID,
		voter:             voter,
		latestMarkerVotes: thresholdmap.New(thresholdmap.UpperThresholdMode, markers.IndexComparator),
	}

	newLatestMarkerVotes.SetModified()
	newLatestMarkerVotes.Persist()

	return
}

// LatestMarkerVotesFromBytes unmarshals a LatestMarkerVotes from a sequence of bytes.
func LatestMarkerVotesFromBytes(bytes []byte) (latestMarkerVotes *LatestMarkerVotes, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if latestMarkerVotes, err = LatestMarkerVotesFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse LatestBranchVotes from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// LatestMarkerVotesFromMarshalUtil unmarshals a LatestMarkerVotes using a MarshalUtil (for easier unmarshalling).
func LatestMarkerVotesFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (latestMarkerVotes *LatestMarkerVotes, err error) {
	latestMarkerVotes = &LatestMarkerVotes{}
	if latestMarkerVotes.sequenceID, err = markers.SequenceIDFromMarshalUtil(marshalUtil); err != nil {
		return nil, errors.Errorf("failed to parse SequenceID from MarshalUtil: %w", err)
	}
	if latestMarkerVotes.voter, err = identity.IDFromMarshalUtil(marshalUtil); err != nil {
		return nil, errors.Errorf("failed to parse Voter from MarshalUtil: %w", err)
	}

	mapSize, err := marshalUtil.ReadUint64()
	if err != nil {
		return nil, errors.Errorf("failed to read mapSize from MarshalUtil: %w", err)
	}

	latestMarkerVotes.latestMarkerVotes = thresholdmap.New(thresholdmap.UpperThresholdMode, markers.IndexComparator)
	for i := uint64(0); i < mapSize; i++ {
		markerIndex, markerIndexErr := markers.IndexFromMarshalUtil(marshalUtil)
		if markerIndexErr != nil {
			return nil, errors.Errorf("failed to read Index from MarshalUtil: %w", markerIndexErr)
		}

		sequenceNumber, sequenceNumberErr := marshalUtil.ReadUint64()
		if markerIndexErr != nil {
			return nil, errors.Errorf("failed to read sequence number from MarshalUtil: %w", sequenceNumberErr)
		}

		latestMarkerVotes.latestMarkerVotes.Set(markerIndex, sequenceNumber)
	}

	return latestMarkerVotes, nil
}

// LatestMarkerVotesFromObjectStorage restores a LatestMarkerVotes that was stored in the ObjectStorage.
func LatestMarkerVotesFromObjectStorage(key, data []byte) (result objectstorage.StorableObject, err error) {
	if result, _, err = LatestMarkerVotesFromBytes(byteutils.ConcatBytes(key, data)); err != nil {
		err = errors.Errorf("failed to parse LatestMarkerVotes from bytes: %w", err)
		return
	}

	return
}

// Voter returns the Voter for the LatestMarkerVotes.
func (l *LatestMarkerVotes) Voter() Voter {
	return l.voter
}

// SequenceNumber returns the sequence number of the vote for the given marker Index.
func (l *LatestMarkerVotes) SequenceNumber(index markers.Index) (sequenceNumber uint64, exists bool) {
	l.RLock()
	defer l.RUnlock()

	key, exists := l.latestMarkerVotes.Get(index)
	if !exists {
		return 0, exists
	}

	return key.(uint64), exists
}

// Store stores the vote with the given marker Index and sequence number.
func (l *LatestMarkerVotes) Store(index markers.Index, sequenceNumber uint64) (stored bool, previousHighestIndex markers.Index) {
	l.Lock()
	defer l.Unlock()

	if maxElement := l.latestMarkerVotes.MaxElement(); maxElement != nil {
		previousHighestIndex = maxElement.Key().(markers.Index)
	}

	// abort if we already have a higher value on an Index that is larger or equal
	_, ceilingValue, ceilingExists := l.latestMarkerVotes.Ceiling(index)
	if ceilingExists && sequenceNumber < ceilingValue.(uint64) {
		return false, previousHighestIndex
	}

	// set the new value
	l.latestMarkerVotes.Set(index, sequenceNumber)

	// remove all predecessors that are lower than the newly set value
	floorKey, floorValue, floorExists := l.latestMarkerVotes.Floor(index - 1)
	for floorExists && floorValue.(uint64) < sequenceNumber {
		l.latestMarkerVotes.Delete(floorKey)

		floorKey, floorValue, floorExists = l.latestMarkerVotes.Floor(index - 1)
	}

	l.SetModified()

	return true, previousHighestIndex
}

// String returns a human-readable version of the LatestMarkerVotes.
func (l *LatestMarkerVotes) String() string {
	builder := stringify.StructBuilder("LatestMarkerVotes")

	l.latestMarkerVotes.ForEach(func(node *thresholdmap.Element) bool {
		builder.AddField(stringify.StructField(node.Key().(markers.Index).String(), node.Value()))

		return true
	})

	return builder.String()
}

// Bytes returns a marshalled version of the LatestMarkerVotes.
func (l *LatestMarkerVotes) Bytes() []byte {
	return byteutils.ConcatBytes(l.ObjectStorageKey(), l.ObjectStorageValue())
}

// Update panics as updates are not supported for this object.
func (l *LatestMarkerVotes) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the storage key for this instance of LatestMarkerVotes.
func (l *LatestMarkerVotes) ObjectStorageKey() []byte {
	return marshalutil.New().
		Write(l.sequenceID).
		Write(l.voter).
		Bytes()
}

// ObjectStorageValue returns the storage value for this instance of LatestMarkerVotes.
func (l *LatestMarkerVotes) ObjectStorageValue() []byte {
	marshalUtil := marshalutil.New()
	marshalUtil.WriteUint64(uint64(l.latestMarkerVotes.Size()))
	l.latestMarkerVotes.ForEach(func(node *thresholdmap.Element) bool {
		marshalUtil.Write(node.Key().(markers.Index))
		marshalUtil.WriteUint64(node.Value().(uint64))

		return true
	})

	return marshalUtil.Bytes()
}

var _ objectstorage.StorableObject = &LatestMarkerVotes{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedLatestMarkerVotes //////////////////////////////////////////////////////////////////////////////////////

// CachedLatestMarkerVotes is a wrapper for the generic CachedObject returned by the object storage that overrides the
// accessor methods with a type-casted one.
type CachedLatestMarkerVotes struct {
	objectstorage.CachedObject
}

// Retain marks the CachedObject to still be in use by the program.
func (c *CachedLatestMarkerVotes) Retain() *CachedLatestMarkerVotes {
	return &CachedLatestMarkerVotes{c.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (c *CachedLatestMarkerVotes) Unwrap() *LatestMarkerVotes {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*LatestMarkerVotes)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (c *CachedLatestMarkerVotes) Consume(consumer func(latestMarkerVotes *LatestMarkerVotes), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*LatestMarkerVotes))
	}, forceRelease...)
}

// String returns a human-readable version of the CachedLatestMarkerVotes.
func (c *CachedLatestMarkerVotes) String() string {
	return stringify.Struct("CachedLatestMarkerVotes",
		stringify.StructField("CachedObject", c.Unwrap()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedLatestMarkerVotesByVoter ///////////////////////////////////////////////////////////////////////////////

// CachedLatestMarkerVotesByVoter represents a cached LatestMarkerVotesByVoter mapped by Voter.
type CachedLatestMarkerVotesByVoter map[Voter]*CachedLatestMarkerVotes

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (c CachedLatestMarkerVotesByVoter) Consume(consumer func(latestMarkerVotes *LatestMarkerVotes), forceRelease ...bool) (consumed bool) {
	for _, cachedLatestMarkerVotes := range c {
		consumed = cachedLatestMarkerVotes.Consume(consumer, forceRelease...) || consumed
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region LatestBranchVotes ////////////////////////////////////////////////////////////////////////////////////////////

// LatestBranchVotes represents the branch supported from an Issuer.
type LatestBranchVotes struct {
	voter             Voter
	latestBranchVotes map[ledgerstate.BranchID]*Vote

	sync.RWMutex
	objectstorage.StorableObjectFlags
}

// Vote returns the Vote for the LatestBranchVotes.
func (l *LatestBranchVotes) Vote(branchID ledgerstate.BranchID) (vote *Vote, exists bool) {
	l.RLock()
	defer l.RUnlock()

	vote, exists = l.latestBranchVotes[branchID]

	return
}

// Store stores the vote for the LatestBranchVotes.
func (l *LatestBranchVotes) Store(vote *Vote) (stored bool) {
	l.Lock()
	defer l.Unlock()

	if currentVote, exists := l.latestBranchVotes[vote.BranchID]; exists && currentVote.SequenceNumber >= vote.SequenceNumber {
		return false
	}

	l.latestBranchVotes[vote.BranchID] = vote
	l.SetModified()

	return true
}

// NewLatestBranchVotes creates a new LatestBranchVotes.
func NewLatestBranchVotes(supporter Voter) (latestBranchVotes *LatestBranchVotes) {
	latestBranchVotes = &LatestBranchVotes{
		voter:             supporter,
		latestBranchVotes: make(map[ledgerstate.BranchID]*Vote),
	}

	latestBranchVotes.Persist()
	latestBranchVotes.SetModified()

	return
}

// LatestBranchVotesFromBytes unmarshals a LatestBranchVotes object from a sequence of bytes.
func LatestBranchVotesFromBytes(bytes []byte) (latestBranchVotes *LatestBranchVotes, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if latestBranchVotes, err = LatestBranchVotesFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse LatestBranchVotes from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// LatestBranchVotesFromMarshalUtil unmarshals a LatestBranchVotes object using a MarshalUtil (for easier unmarshalling).
func LatestBranchVotesFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (latestBranchVotes *LatestBranchVotes, err error) {
	latestBranchVotes = &LatestBranchVotes{}
	if latestBranchVotes.voter, err = identity.IDFromMarshalUtil(marshalUtil); err != nil {
		return nil, errors.Errorf("failed to parse Voter from MarshalUtil: %w", err)
	}

	mapSize, err := marshalUtil.ReadUint64()
	if err != nil {
		return nil, errors.Errorf("failed to parse map size (%v): %w", err, cerrors.ErrParseBytesFailed)
	}

	latestBranchVotes.latestBranchVotes = make(map[ledgerstate.BranchID]*Vote, int(mapSize))

	for i := uint64(0); i < mapSize; i++ {
		branchID, voteErr := ledgerstate.BranchIDFromMarshalUtil(marshalUtil)
		if voteErr != nil {
			return nil, errors.Errorf("failed to parse BranchID from MarshalUtil: %w", voteErr)
		}

		vote, voteErr := VoteFromMarshalUtil(marshalUtil)
		if voteErr != nil {
			return nil, errors.Errorf("failed to parse Vote from MarshalUtil: %w", voteErr)
		}

		latestBranchVotes.latestBranchVotes[branchID] = vote
	}

	return latestBranchVotes, nil
}

// LatestBranchVotesFromObjectStorage restores a LatestBranchVotes object from the object storage.
func LatestBranchVotesFromObjectStorage(key, data []byte) (result objectstorage.StorableObject, err error) {
	if result, _, err = LatestBranchVotesFromBytes(byteutils.ConcatBytes(key, data)); err != nil {
		err = errors.Errorf("failed to parse LatestBranchVotes from bytes: %w", err)
		return
	}

	return
}

// Bytes returns a marshaled version of the LatestBranchVotes.
func (l *LatestBranchVotes) Bytes() (marshaledSequenceSupporters []byte) {
	return byteutils.ConcatBytes(l.ObjectStorageKey(), l.ObjectStorageValue())
}

// String returns a human-readable version of the LatestBranchVotes.
func (l *LatestBranchVotes) String() string {
	return stringify.Struct("LatestBranchVotes",
		stringify.StructField("voter", l.voter),
	)
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (l *LatestBranchVotes) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (l *LatestBranchVotes) ObjectStorageKey() []byte {
	return l.voter.Bytes()
}

// ObjectStorageValue marshals the LatestBranchVotes into a sequence of bytes that are used as the value part in the
// object storage.
func (l *LatestBranchVotes) ObjectStorageValue() []byte {
	l.RLock()
	defer l.RUnlock()

	marshalUtil := marshalutil.New()

	marshalUtil.WriteUint64(uint64(len(l.latestBranchVotes)))

	for branchID, vote := range l.latestBranchVotes {
		marshalUtil.Write(branchID)
		marshalUtil.Write(vote)
	}

	return marshalUtil.Bytes()
}

// code contract (make sure the struct implements all required methods).
var _ objectstorage.StorableObject = &LatestBranchVotes{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedLatestBranchVotes //////////////////////////////////////////////////////////////////////////////////////

// CachedLatestBranchVotes is a wrapper for the generic CachedObject returned by the object storage that overrides the
// accessor methods with a type-casted one.
type CachedLatestBranchVotes struct {
	objectstorage.CachedObject
}

// Retain marks the CachedObject to still be in use by the program.
func (c *CachedLatestBranchVotes) Retain() *CachedLatestBranchVotes {
	return &CachedLatestBranchVotes{c.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (c *CachedLatestBranchVotes) Unwrap() *LatestBranchVotes {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*LatestBranchVotes)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (c *CachedLatestBranchVotes) Consume(consumer func(latestBranchVotes *LatestBranchVotes), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*LatestBranchVotes))
	}, forceRelease...)
}

// String returns a human-readable version of the CachedLatestBranchVotes.
func (c *CachedLatestBranchVotes) String() string {
	return stringify.Struct("CachedLatestBranchVotes",
		stringify.StructField("CachedObject", c.Unwrap()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Vote /////////////////////////////////////////////////////////////////////////////////////////////////////////

// Vote represents a struct that holds information about the shared Opinion of a node regarding an individual Branch.
type Vote struct {
	Voter          Voter
	BranchID       ledgerstate.BranchID
	Opinion        Opinion
	SequenceNumber uint64
}

// VoteFromMarshalUtil unmarshals a Vote structure using a MarshalUtil (for easier unmarshalling).
func VoteFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (vote *Vote, err error) {
	vote = &Vote{}

	if vote.Voter, err = identity.IDFromMarshalUtil(marshalUtil); err != nil {
		return nil, errors.Errorf("failed to parse Voter from MarshalUtil: %w", err)
	}

	if vote.BranchID, err = ledgerstate.BranchIDFromMarshalUtil(marshalUtil); err != nil {
		return nil, errors.Errorf("failed to parse BranchID from MarshalUtil: %w", err)
	}

	untypedOpinion, err := marshalUtil.ReadUint8()
	if err != nil {
		return nil, errors.Errorf("failed to parse Opinion from MarshalUtil: %w", err)
	}
	vote.Opinion = Opinion(untypedOpinion)

	if vote.SequenceNumber, err = marshalUtil.ReadUint64(); err != nil {
		return nil, errors.Errorf("failed to parse SequenceNumber from MarshalUtil: %w", err)
	}

	return
}

// WithOpinion derives a vote for the given Opinion.
func (v *Vote) WithOpinion(opinion Opinion) (voteWithOpinion *Vote) {
	return &Vote{
		Voter:          v.Voter,
		BranchID:       v.BranchID,
		Opinion:        opinion,
		SequenceNumber: v.SequenceNumber,
	}
}

// WithBranchID derives a vote for the given BranchID.
func (v *Vote) WithBranchID(branchID ledgerstate.BranchID) (rejectedVote *Vote) {
	return &Vote{
		Voter:          v.Voter,
		BranchID:       branchID,
		Opinion:        v.Opinion,
		SequenceNumber: v.SequenceNumber,
	}
}

// Bytes returns the bytes of the Vote.
func (v *Vote) Bytes() []byte {
	return marshalutil.New().
		Write(v.Voter).
		Write(v.BranchID).
		WriteUint8(uint8(v.Opinion)).
		WriteUint64(v.SequenceNumber).
		Bytes()
}

// String returns a human-readable version of the Vote.
func (v *Vote) String() string {
	return stringify.Struct("Vote",
		stringify.StructField("Voter", v.Voter),
		stringify.StructField("BranchID", v.BranchID),
		stringify.StructField("Opinion", int(v.Opinion)),
		stringify.StructField("SequenceNumber", v.SequenceNumber),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
