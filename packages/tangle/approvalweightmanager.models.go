package tangle

import (
	"context"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/generics/thresholdmap"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/serix"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
)

// region ApprovalWeightManager Models /////////////////////////////////////////////////////////////////////////////////

// region BranchWeight /////////////////////////////////////////////////////////////////////////////////////////////////

// BranchWeight is a data structure that tracks the weight of a BranchID.
type BranchWeight struct {
	branchWeightInner `serix:"0"`
}

type branchWeightInner struct {
	BranchID ledgerstate.BranchID
	Weight   float64 `serix:"0"`

	weightMutex sync.RWMutex

	objectstorage.StorableObjectFlags
}

// NewBranchWeight creates a new BranchWeight.
func NewBranchWeight(branchID ledgerstate.BranchID) (branchWeight *BranchWeight) {
	branchWeight = &BranchWeight{
		branchWeightInner{
			BranchID: branchID,
		},
	}

	branchWeight.Persist()
	branchWeight.SetModified()

	return
}

// FromObjectStorage creates an BranchWeight from sequences of key and bytes.
func (b *BranchWeight) FromObjectStorage(key, value []byte) (objectstorage.StorableObject, error) {
	result, err := b.FromBytes(byteutils.ConcatBytes(key, value))
	if err != nil {
		err = errors.Errorf("failed to parse BranchWeight from bytes: %w", err)
	}
	return result, err
}

// FromBytes unmarshals a BranchWeight object from a sequence of bytes.
func (b *BranchWeight) FromBytes(data []byte) (branchWeight *BranchWeight, err error) {
	bw := new(BranchWeight)
	if b != nil {
		bw = b
	}
	branchID := new(ledgerstate.BranchID)
	bytesRead, err := serix.DefaultAPI.Decode(context.Background(), data, branchID, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse BranchWeight.BranchID: %w", err)
		return bw, err
	}

	_, err = serix.DefaultAPI.Decode(context.Background(), data[bytesRead:], bw, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse BranchWeight: %w", err)
		return bw, err
	}
	bw.branchWeightInner.BranchID = *branchID
	return bw, err
}

// BranchID returns the BranchID that is being tracked.
func (b *BranchWeight) BranchID() (branchID ledgerstate.BranchID) {
	return b.branchWeightInner.BranchID
}

// Weight returns the weight of the BranchID.
func (b *BranchWeight) Weight() (weight float64) {
	b.weightMutex.RLock()
	defer b.weightMutex.RUnlock()

	return b.branchWeightInner.Weight
}

// SetWeight sets the weight for the BranchID and returns true if it was modified.
func (b *BranchWeight) SetWeight(weight float64) (modified bool) {
	b.weightMutex.Lock()
	defer b.weightMutex.Unlock()

	if weight == b.branchWeightInner.Weight {
		return false
	}

	b.branchWeightInner.Weight = weight
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
		stringify.StructField("BranchID", b.BranchID()),
		stringify.StructField("Weight", b.Weight()),
	)
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (b *BranchWeight) ObjectStorageKey() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), b.BranchID(), serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// ObjectStorageValue marshals the BranchWeight into a sequence of bytes. The ID is not serialized here as it is only used as
// a key in the ObjectStorage.
func (b *BranchWeight) ObjectStorageValue() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), b, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// code contract (make sure the struct implements all required methods).
var _ objectstorage.StorableObject = new(BranchWeight)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
// region SerializableSet ////////////////////////////////////////////////////////////////////////////////////////////////////////

// Voter is a type wrapper for identity.ID and defines a node that supports a branch or marker.
type Voter = identity.ID

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Voters ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Voters is a set of node identities that votes for a particular Branch.
type Voters struct {
	set.Set[Voter]
}

// NewVoters is the constructor of the Voters type.
func NewVoters() (voters *Voters) {
	return &Voters{set.New[Voter]()}
}

// AddAll adds all new Voters to the Set.
func (v *Voters) AddAll(voters *Voters) {
	voters.Set.ForEach(func(voter Voter) {
		v.Set.Add(voter)
	})
}

// Clone returns a copy of the Voters.
func (v *Voters) Clone() (clonedVoters *Voters) {
	clonedVoters = NewVoters()
	v.Set.ForEach(func(voter Voter) {
		clonedVoters.Set.Add(voter)
	})

	return
}

// Encode returns a serialized byte slice of the object.
func (v *Voters) Encode() ([]byte, error) {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), v.Set, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes, nil
}

// Decode deserializes bytes into a valid object.
func (v *Voters) Decode(data []byte) (bytesRead int, err error) {

	v.Set = set.New[Voter]()
	bytesRead, err = serix.DefaultAPI.Decode(context.Background(), data, &v.Set, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse Voters: %w", err)
		return
	}
	return
}

// Intersect creates an intersection of two set of Voters.
func (v *Voters) Intersect(other *Voters) (intersection *Voters) {
	intersection = NewVoters()
	v.Set.ForEach(func(voter Voter) {
		if other.Set.Has(voter) {
			intersection.Set.Add(voter)
		}
	})
	return
}

// String returns a human-readable version of the Voters.
func (v *Voters) String() string {
	structBuilder := stringify.StructBuilder("Voters")
	v.Set.ForEach(func(voter Voter) {
		structBuilder.AddField(stringify.StructField(voter.String(), "true"))
	})

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchVoters /////////////////////////////////////////////////////////////////////////////////////////////////

// BranchVoters is a data structure that tracks which nodes support a branch.
type BranchVoters struct {
	branchVotersInner `serix:"0"`
}

type branchVotersInner struct {
	BranchID ledgerstate.BranchID
	Voters   *Voters `serix:"1"`

	votersMutex sync.RWMutex

	objectstorage.StorableObjectFlags
}

// NewBranchVoters is the constructor for the BranchVoters object.
func NewBranchVoters(branchID ledgerstate.BranchID) (branchVoters *BranchVoters) {
	branchVoters = &BranchVoters{
		branchVotersInner{
			BranchID: branchID,
			Voters:   NewVoters(),
		},
	}

	branchVoters.Persist()
	branchVoters.SetModified()

	return
}

// FromObjectStorage creates an BranchVoters from sequences of key and bytes.
func (b *BranchVoters) FromObjectStorage(key, value []byte) (objectstorage.StorableObject, error) {
	result, err := b.FromBytes(byteutils.ConcatBytes(key, value))
	if err != nil {
		err = errors.Errorf("failed to parse BranchVoters from bytes: %w", err)
	}
	return result, err
}

// FromBytes unmarshals a BranchVoters object from a sequence of bytes.
func (b *BranchVoters) FromBytes(data []byte) (branchVoters *BranchVoters, err error) {
	votes := new(BranchVoters)
	if b != nil {
		votes = b
	}
	branchID := new(ledgerstate.BranchID)

	bytesRead, err := serix.DefaultAPI.Decode(context.Background(), data, branchID, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse BranchVoters.BranchID: %w", err)
		return votes, err
	}

	_, err = serix.DefaultAPI.Decode(context.Background(), data[bytesRead:], votes, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse BranchVoters: %w", err)
		return votes, err
	}
	votes.branchVotersInner.BranchID = *branchID
	return votes, err
}

// BranchID returns the BranchID that is being tracked.
func (b *BranchVoters) BranchID() (branchID ledgerstate.BranchID) {
	return b.branchVotersInner.BranchID
}

// Has returns true if the given Voter is currently supporting this Branch.
func (b *BranchVoters) Has(voter Voter) bool {
	b.votersMutex.RLock()
	defer b.votersMutex.RUnlock()

	return b.branchVotersInner.Voters.Set.Has(voter)
}

// AddVoter adds a new Voter to the tracked BranchID.
func (b *BranchVoters) AddVoter(voter Voter) (added bool) {
	b.votersMutex.Lock()
	defer b.votersMutex.Unlock()

	if added = b.branchVotersInner.Voters.Set.Add(voter); !added {
		return
	}
	b.SetModified()

	return
}

// AddVoters adds the Voters set to the tracked BranchID.
func (b *BranchVoters) AddVoters(voters *Voters) (added bool) {
	voters.Set.ForEach(func(voter Voter) {
		if b.branchVotersInner.Voters.Set.Add(voter) {
			added = true
		}
	})

	if added {
		b.SetModified()
	}

	return
}

// DeleteVoter deletes a Voter from the tracked BranchID.
func (b *BranchVoters) DeleteVoter(voter Voter) (deleted bool) {
	b.votersMutex.Lock()
	defer b.votersMutex.Unlock()

	if deleted = b.branchVotersInner.Voters.Set.Delete(voter); !deleted {
		return
	}
	b.SetModified()

	return
}

// Voters returns the set of Voters that are supporting the given BranchID.
func (b *BranchVoters) Voters() (voters *Voters) {
	b.votersMutex.RLock()
	defer b.votersMutex.RUnlock()

	return b.branchVotersInner.Voters.Clone()
}

// Bytes returns a marshaled version of the BranchVoters.
func (b *BranchVoters) Bytes() (marshaledBranchVoters []byte) {
	return byteutils.ConcatBytes(b.ObjectStorageKey(), b.ObjectStorageValue())
}

// String returns a human-readable version of the BranchVoters.
func (b *BranchVoters) String() string {
	return stringify.Struct("BranchVoters",
		stringify.StructField("branchID", b.BranchID()),
		stringify.StructField("voters", b.Voters()),
	)
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (b *BranchVoters) ObjectStorageKey() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), b.branchVotersInner.BranchID, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// ObjectStorageValue marshals the BranchVoters into a sequence of bytes. The ID is not serialized here as it is only used as
// a key in the ObjectStorage.
func (b *BranchVoters) ObjectStorageValue() []byte {
	b.votersMutex.RLock()
	defer b.votersMutex.RUnlock()
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), b, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// code contract (make sure the struct implements all required methods).
var _ objectstorage.StorableObject = new(BranchVoters)

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

// region latestMarkerVotesMap /////////////////////////////////////////////////////////////////////////////////////////

type latestMarkerVotesMap struct {
	*thresholdmap.ThresholdMap[markers.Index, VotePower]
}

func newLatestMarkerVotesMap() *latestMarkerVotesMap {
	return &latestMarkerVotesMap{thresholdmap.New[markers.Index, VotePower](thresholdmap.UpperThresholdMode, markers.IndexComparator)}
}

// Encode returns a serialized byte slice of the object.
func (l *latestMarkerVotesMap) Encode() ([]byte, error) {
	return l.ThresholdMap.Encode()
}

// Decode deserializes bytes into a valid object.
func (l *latestMarkerVotesMap) Decode(b []byte) (bytesRead int, err error) {
	l.ThresholdMap = thresholdmap.New[markers.Index, VotePower](thresholdmap.UpperThresholdMode, markers.IndexComparator)
	return l.ThresholdMap.Decode(b)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region LatestMarkerVotes ////////////////////////////////////////////////////////////////////////////////////////////

// VotePower is used to establish an absolute order of votes, regarldless of their arrival order.
// Currently, the used VotePower is the SequenceNumber embedded in the Message Layout, so that, regardless
// of the order in which votes are received, the same conclusion is computed.
// Alternatively, the objective timestamp of a Message could be used.
type VotePower = uint64

// LatestMarkerVotesKeyPartition defines the partition of the storage key of the LastMarkerVotes model.
var LatestMarkerVotesKeyPartition = objectstorage.PartitionKey(markers.SequenceIDLength, identity.IDLength)

// LatestMarkerVotes keeps track of the most up-to-date for a certain Voter casted on a specific Marker SequenceID.
// Votes can be casted on Markers (SequenceID, Index), but can arrive in any arbitrary order.
// Due to the nature of a Sequence, a vote casted for a certain Index clobbers votes for every lower index.
// Similarly, if a vote for an Index is casted and an existing vote for an higher Index exists, the operation has no effect.
type LatestMarkerVotes struct {
	latestMarkerVotesInner `serix:"0"`
}

type latestMarkerVotesInner struct {
	SequenceID        markers.SequenceID
	Voter             Voter
	LatestMarkerVotes *latestMarkerVotesMap `serix:"0"`

	sync.RWMutex
	objectstorage.StorableObjectFlags
}

// NewLatestMarkerVotes creates a new NewLatestMarkerVotes instance associated with the given details.
func NewLatestMarkerVotes(sequenceID markers.SequenceID, voter Voter) (newLatestMarkerVotes *LatestMarkerVotes) {
	newLatestMarkerVotes = &LatestMarkerVotes{
		latestMarkerVotesInner{
			SequenceID:        sequenceID,
			Voter:             voter,
			LatestMarkerVotes: newLatestMarkerVotesMap(),
		},
	}

	newLatestMarkerVotes.SetModified()
	newLatestMarkerVotes.Persist()

	return
}

// FromObjectStorage creates an LatestMarkerVotes from sequences of key and bytes.
func (l *LatestMarkerVotes) FromObjectStorage(key, value []byte) (objectstorage.StorableObject, error) {
	result, err := l.FromBytes(byteutils.ConcatBytes(key, value))
	if err != nil {
		err = errors.Errorf("failed to parse LatestMarkerVotes from bytes: %w", err)
	}
	return result, err
}

// FromBytes unmarshals a LatestMarkerVotes from a sequence of bytes.
func (l *LatestMarkerVotes) FromBytes(data []byte) (latestMarkerVotes *LatestMarkerVotes, err error) {
	votes := new(LatestMarkerVotes)
	if l != nil {
		votes = l
	}

	sequenceID := new(markers.SequenceID)
	bytesReadSequenceID, err := serix.DefaultAPI.Decode(context.Background(), data, sequenceID, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse LatestMarkerVotes.SequenceID: %w", err)
		return votes, err
	}

	voter := new(Voter)
	bytesReadVoter, err := serix.DefaultAPI.Decode(context.Background(), data[bytesReadSequenceID:], voter, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse LatestMarkerVotes.Voter: %w", err)
		return votes, err
	}

	_, err = serix.DefaultAPI.Decode(context.Background(), data[bytesReadSequenceID+bytesReadVoter:], votes, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse LatestMarkerVotes: %w", err)
		return votes, err
	}
	votes.latestMarkerVotesInner.SequenceID = *sequenceID
	votes.latestMarkerVotesInner.Voter = *voter
	return votes, err
}

// Voter returns the Voter for the LatestMarkerVotes.
func (l *LatestMarkerVotes) Voter() Voter {
	return l.latestMarkerVotesInner.Voter
}

// Power returns the power of the vote for the given marker Index.
func (l *LatestMarkerVotes) Power(index markers.Index) (power VotePower, exists bool) {
	l.RLock()
	defer l.RUnlock()

	key, exists := l.latestMarkerVotesInner.LatestMarkerVotes.Get(index)
	if !exists {
		return 0, exists
	}

	return key, exists
}

// Store stores the vote with the given marker Index and votePower.
// The votePower parameter is used to determine the order of the vote.
func (l *LatestMarkerVotes) Store(index markers.Index, power VotePower) (stored bool, previousHighestIndex markers.Index) {
	l.Lock()
	defer l.Unlock()

	if maxElement := l.latestMarkerVotesInner.LatestMarkerVotes.MaxElement(); maxElement != nil {
		previousHighestIndex = maxElement.Key()
	}

	// abort if we already have a higher value on an Index that is larger or equal
	_, ceilingValue, ceilingExists := l.latestMarkerVotesInner.LatestMarkerVotes.Ceiling(index)
	if ceilingExists && power < ceilingValue {
		return false, previousHighestIndex
	}

	// set the new value
	l.latestMarkerVotesInner.LatestMarkerVotes.Set(index, power)

	// remove all predecessors that are lower than the newly set value
	floorKey, floorValue, floorExists := l.latestMarkerVotesInner.LatestMarkerVotes.Floor(index - 1)
	for floorExists && floorValue < power {
		l.latestMarkerVotesInner.LatestMarkerVotes.Delete(floorKey)

		floorKey, floorValue, floorExists = l.latestMarkerVotesInner.LatestMarkerVotes.Floor(index - 1)
	}

	l.SetModified()

	return true, previousHighestIndex
}

// String returns a human-readable version of the LatestMarkerVotes.
func (l *LatestMarkerVotes) String() string {
	builder := stringify.StructBuilder("LatestMarkerVotes")

	l.latestMarkerVotesInner.LatestMarkerVotes.ForEach(func(node *thresholdmap.Element[markers.Index, VotePower]) bool {
		builder.AddField(stringify.StructField(node.Key().String(), node.Value()))
		return true
	})

	return builder.String()
}

// Bytes returns a marshaled version of the LatestMarkerVotes.
func (l *LatestMarkerVotes) Bytes() []byte {
	return byteutils.ConcatBytes(l.ObjectStorageKey(), l.ObjectStorageValue())
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (l *LatestMarkerVotes) ObjectStorageKey() []byte {
	objSeqIDBytes, err := serix.DefaultAPI.Encode(context.Background(), l.SequenceID, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}

	objVoterBytes, err := serix.DefaultAPI.Encode(context.Background(), l.latestMarkerVotesInner.Voter, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return byteutils.ConcatBytes(objSeqIDBytes, objVoterBytes)
}

// ObjectStorageValue marshals the LatestMarkerVotes into a sequence of bytes. The ID is not serialized here as it is only used as
// a key in the ObjectStorage.
func (l *LatestMarkerVotes) ObjectStorageValue() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), l, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

var _ objectstorage.StorableObject = new(LatestMarkerVotes)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedLatestMarkerVotesByVoter ///////////////////////////////////////////////////////////////////////////////

// CachedLatestMarkerVotesByVoter represents a cached LatestMarkerVotesByVoter mapped by Voter.
type CachedLatestMarkerVotesByVoter map[Voter]*objectstorage.CachedObject[*LatestMarkerVotes]

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
	latestBranchVotesInner `serix:"0"`
}

type latestBranchVotesInner struct {
	Voter             Voter
	LatestBranchVotes map[ledgerstate.BranchID]*BranchVote `serix:"0,lengthPrefixType=uint32"`
	sync.RWMutex
	objectstorage.StorableObjectFlags
}

// Vote returns the Vote for the LatestBranchVotes.
func (l *LatestBranchVotes) Vote(branchID ledgerstate.BranchID) (vote *BranchVote, exists bool) {
	l.RLock()
	defer l.RUnlock()

	vote, exists = l.latestBranchVotesInner.LatestBranchVotes[branchID]

	return
}

// Store stores the vote for the LatestBranchVotes.
func (l *LatestBranchVotes) Store(vote *BranchVote) (stored bool) {
	l.Lock()
	defer l.Unlock()

	if currentVote, exists := l.latestBranchVotesInner.LatestBranchVotes[vote.BranchID]; exists && currentVote.VotePower >= vote.VotePower {
		return false
	}

	l.latestBranchVotesInner.LatestBranchVotes[vote.BranchID] = vote
	l.SetModified()

	return true
}

// NewLatestBranchVotes creates a new LatestBranchVotes.
func NewLatestBranchVotes(voter Voter) (latestBranchVotes *LatestBranchVotes) {
	latestBranchVotes = &LatestBranchVotes{
		latestBranchVotesInner{
			Voter:             voter,
			LatestBranchVotes: make(map[ledgerstate.BranchID]*BranchVote),
		},
	}

	latestBranchVotes.Persist()
	latestBranchVotes.SetModified()

	return
}

// FromObjectStorage creates an LatestBranchVotes from sequences of key and bytes.
func (l *LatestBranchVotes) FromObjectStorage(key, value []byte) (objectstorage.StorableObject, error) {
	result, err := l.FromBytes(byteutils.ConcatBytes(key, value))
	if err != nil {
		err = errors.Errorf("failed to parse LatestBranchVotes from bytes: %w", err)
	}
	return result, err
}

// FromBytes unmarshals a LatestBranchVotes object from a sequence of bytes.
func (l *LatestBranchVotes) FromBytes(data []byte) (latestBranchVotes *LatestBranchVotes, err error) {
	votes := new(LatestBranchVotes)
	if l != nil {
		votes = l
	}
	voter := new(Voter)

	bytesRead, err := serix.DefaultAPI.Decode(context.Background(), data, voter, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse LatestBranchVotes.Voter: %w", err)
		return votes, err
	}

	_, err = serix.DefaultAPI.Decode(context.Background(), data[bytesRead:], votes, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse LatestBranchVotes: %w", err)
		return votes, err
	}
	votes.latestBranchVotesInner.Voter = *voter
	return votes, err
}

// Bytes returns a marshaled version of the LatestBranchVotes.
func (l *LatestBranchVotes) Bytes() []byte {
	return byteutils.ConcatBytes(l.ObjectStorageKey(), l.ObjectStorageValue())
}

// String returns a human-readable version of the LatestBranchVotes.
func (l *LatestBranchVotes) String() string {
	return stringify.Struct("LatestBranchVotes",
		stringify.StructField("Voter", l.latestBranchVotesInner.Voter),
	)
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (l *LatestBranchVotes) ObjectStorageKey() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), l.latestBranchVotesInner.Voter, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// ObjectStorageValue marshals the LatestBranchVotes into a sequence of bytes. The ID is not serialized here as it is only used as
// a key in the ObjectStorage.
func (l *LatestBranchVotes) ObjectStorageValue() []byte {
	l.RLock()
	defer l.RUnlock()
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), l, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// code contract (make sure the struct implements all required methods).
var _ objectstorage.StorableObject = new(LatestBranchVotes)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Vote /////////////////////////////////////////////////////////////////////////////////////////////////////////

// BranchVote represents a struct that holds information about what Opinion a certain Voter has on a Branch.
type BranchVote struct {
	Voter     Voter                `serix:"0"`
	BranchID  ledgerstate.BranchID `serix:"1"`
	Opinion   Opinion              `serix:"2"`
	VotePower VotePower            `serix:"3"`
}

// WithOpinion derives a vote for the given Opinion.
func (v *BranchVote) WithOpinion(opinion Opinion) (voteWithOpinion *BranchVote) {
	return &BranchVote{
		Voter:     v.Voter,
		BranchID:  v.BranchID,
		Opinion:   opinion,
		VotePower: v.VotePower,
	}
}

// WithBranchID derives a vote for the given BranchID.
func (v *BranchVote) WithBranchID(branchID ledgerstate.BranchID) (rejectedVote *BranchVote) {
	return &BranchVote{
		Voter:     v.Voter,
		BranchID:  branchID,
		Opinion:   v.Opinion,
		VotePower: v.VotePower,
	}
}

// String returns a human-readable version of the Vote.
func (v *BranchVote) String() string {
	return stringify.Struct("Vote",
		stringify.StructField("Voter", v.Voter),
		stringify.StructField("BranchID", v.BranchID),
		stringify.StructField("Opinion", int(v.Opinion)),
		stringify.StructField("VotePower", v.VotePower),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
