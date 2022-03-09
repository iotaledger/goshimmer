package tangle

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/generics/thresholdmap"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/marshalutil"
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

// FromObjectStorage creates an BranchWeight from sequences of key and bytes.
func (b *BranchWeight) FromObjectStorage(key, bytes []byte) (objectstorage.StorableObject, error) {
	result, err := b.FromBytes(byteutils.ConcatBytes(key, bytes))
	if err != nil {
		err = errors.Errorf("failed to parse BranchWeight from bytes: %w", err)
	}
	return result, err
}

// FromBytes unmarshals a BranchWeight object from a sequence of bytes.
func (b *BranchWeight) FromBytes(bytes []byte) (branchWeight *BranchWeight, err error) {
	marshalUtil := marshalutil.New(bytes)
	if branchWeight, err = b.FromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse BranchWeight from MarshalUtil: %w", err)
		return
	}
	return
}

// FromMarshalUtil unmarshals a BranchWeight object using a MarshalUtil (for easier unmarshalling).
func (b *BranchWeight) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (branchWeight *BranchWeight, err error) {
	if branchWeight = b; branchWeight == nil {
		branchWeight = new(BranchWeight)
	}
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
var _ objectstorage.StorableObject = new(BranchWeight)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Voter ////////////////////////////////////////////////////////////////////////////////////////////////////////

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
	return &Voters{
		Set: set.New[Voter](),
	}
}

// AddAll adds all new Voters to the Set.
func (v *Voters) AddAll(voters *Voters) {
	voters.ForEach(func(voter Voter) {
		v.Set.Add(voter)
	})
}

// Clone returns a copy of the Voters.
func (v *Voters) Clone() (clonedVoters *Voters) {
	clonedVoters = NewVoters()
	v.ForEach(func(voter Voter) {
		clonedVoters.Add(voter)
	})

	return
}

// Intersect creates an intersection of two set of Voters.
func (v *Voters) Intersect(other *Voters) (intersection *Voters) {
	intersection = NewVoters()
	v.ForEach(func(voter Voter) {
		if other.Has(voter) {
			intersection.Add(voter)
		}
	})
	return
}

// String returns a human-readable version of the Voters.
func (v *Voters) String() string {
	structBuilder := stringify.StructBuilder("Voters")
	v.ForEach(func(voter Voter) {
		structBuilder.AddField(stringify.StructField(voter.String(), "true"))
	})

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchVoters /////////////////////////////////////////////////////////////////////////////////////////////////

// BranchVoters is a data structure that tracks which nodes support a branch.
type BranchVoters struct {
	branchID ledgerstate.BranchID
	voters   *Voters

	votersMutex sync.RWMutex

	objectstorage.StorableObjectFlags
}

// NewBranchVoters is the constructor for the BranchVoters object.
func NewBranchVoters(branchID ledgerstate.BranchID) (branchVoters *BranchVoters) {
	branchVoters = &BranchVoters{
		branchID: branchID,
		voters:   NewVoters(),
	}

	branchVoters.Persist()
	branchVoters.SetModified()

	return
}

// FromObjectStorage creates an BranchVoters from sequences of key and bytes.
func (b *BranchVoters) FromObjectStorage(key, bytes []byte) (objectstorage.StorableObject, error) {
	result, err := b.FromBytes(byteutils.ConcatBytes(key, bytes))
	if err != nil {
		err = errors.Errorf("failed to parse BranchVoters from bytes: %w", err)
	}
	return result, err
}

// FromBytes unmarshals a BranchVoters object from a sequence of bytes.
func (b *BranchVoters) FromBytes(bytes []byte) (branchVoters *BranchVoters, err error) {
	marshalUtil := marshalutil.New(bytes)
	if branchVoters, err = b.FromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse SequenceVoters from MarshalUtil: %w", err)
		return
	}
	return
}

// FromMarshalUtil unmarshals a BranchVoters object using a MarshalUtil (for easier unmarshalling).
func (b *BranchVoters) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (branchVoters *BranchVoters, err error) {
	if branchVoters = b; branchVoters == nil {
		branchVoters = new(BranchVoters)
	}
	if branchVoters.branchID, err = ledgerstate.BranchIDFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse BranchID from MarshalUtil: %w", err)
		return
	}

	votersCount, err := marshalUtil.ReadUint64()
	if err != nil {
		err = errors.Errorf("failed to parse voters count (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	branchVoters.voters = NewVoters()
	for i := uint64(0); i < votersCount; i++ {
		voter, voterErr := identity.IDFromMarshalUtil(marshalUtil)
		if voterErr != nil {
			err = errors.Errorf("failed to parse Voter (%v): %w", voterErr, cerrors.ErrParseBytesFailed)
			return
		}

		branchVoters.voters.Add(voter)
	}

	return
}

// BranchID returns the BranchID that is being tracked.
func (b *BranchVoters) BranchID() (branchID ledgerstate.BranchID) {
	return b.branchID
}

// Has returns true if the given Voter is currently supporting this Branch.
func (b *BranchVoters) Has(voter Voter) bool {
	b.votersMutex.RLock()
	defer b.votersMutex.RUnlock()

	return b.voters.Has(voter)
}

// AddVoter adds a new Voter to the tracked BranchID.
func (b *BranchVoters) AddVoter(voter Voter) (added bool) {
	b.votersMutex.Lock()
	defer b.votersMutex.Unlock()

	if added = b.voters.Add(voter); !added {
		return
	}
	b.SetModified()

	return
}

// AddVoters adds the Voters set to the tracked BranchID.
func (b *BranchVoters) AddVoters(voters *Voters) (added bool) {
	voters.ForEach(func(voter Voter) {
		if b.voters.Add(voter) {
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

	if deleted = b.voters.Delete(voter); !deleted {
		return
	}
	b.SetModified()

	return
}

// Voters returns the set of Voters that are supporting the given BranchID.
func (b *BranchVoters) Voters() (voters *Voters) {
	b.votersMutex.RLock()
	defer b.votersMutex.RUnlock()

	return b.voters.Clone()
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
	return b.BranchID().Bytes()
}

// ObjectStorageValue marshals the BranchVoters into a sequence of bytes that are used as the value part in the
// object storage.
func (b *BranchVoters) ObjectStorageValue() []byte {
	b.votersMutex.RLock()
	defer b.votersMutex.RUnlock()

	marshalUtil := marshalutil.New(marshalutil.Uint64Size + b.voters.Size()*identity.IDLength)
	marshalUtil.WriteUint64(uint64(b.voters.Size()))

	b.voters.ForEach(func(voter Voter) {
		marshalUtil.WriteBytes(voter.Bytes())
	})

	return marshalUtil.Bytes()
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
	sequenceID        markers.SequenceID
	voter             Voter
	latestMarkerVotes *thresholdmap.ThresholdMap[markers.Index, VotePower]

	sync.RWMutex
	objectstorage.StorableObjectFlags
}

// NewLatestMarkerVotes creates a new NewLatestMarkerVotes instance associated with the given details.
func NewLatestMarkerVotes(sequenceID markers.SequenceID, voter Voter) (newLatestMarkerVotes *LatestMarkerVotes) {
	newLatestMarkerVotes = &LatestMarkerVotes{
		sequenceID:        sequenceID,
		voter:             voter,
		latestMarkerVotes: thresholdmap.New[markers.Index, VotePower](thresholdmap.UpperThresholdMode, markers.IndexComparator),
	}

	newLatestMarkerVotes.SetModified()
	newLatestMarkerVotes.Persist()

	return
}

// FromObjectStorage creates an LatestMarkerVotes from sequences of key and bytes.
func (l *LatestMarkerVotes) FromObjectStorage(key, bytes []byte) (objectstorage.StorableObject, error) {
	result, err := l.FromBytes(byteutils.ConcatBytes(key, bytes))
	if err != nil {
		err = errors.Errorf("failed to parse LatestMarkerVotes from bytes: %w", err)
	}
	return result, err
}

// FromBytes unmarshals a LatestMarkerVotes from a sequence of bytes.
func (l *LatestMarkerVotes) FromBytes(bytes []byte) (latestMarkerVotes *LatestMarkerVotes, err error) {
	marshalUtil := marshalutil.New(bytes)
	if latestMarkerVotes, err = l.FromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse LatestBranchVotes from MarshalUtil: %w", err)
		return
	}
	return
}

// FromMarshalUtil unmarshals a LatestMarkerVotes using a MarshalUtil (for easier unmarshalling).
func (l *LatestMarkerVotes) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (latestMarkerVotes *LatestMarkerVotes, err error) {
	if latestMarkerVotes = l; latestMarkerVotes == nil {
		latestMarkerVotes = new(LatestMarkerVotes)
	}

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

	latestMarkerVotes.latestMarkerVotes = thresholdmap.New[markers.Index, VotePower](thresholdmap.UpperThresholdMode, markers.IndexComparator)
	for i := uint64(0); i < mapSize; i++ {
		markerIndex, markerIndexErr := markers.IndexFromMarshalUtil(marshalUtil)
		if markerIndexErr != nil {
			return nil, errors.Errorf("failed to read Index from MarshalUtil: %w", markerIndexErr)
		}

		votePower, votePowerErr := marshalUtil.ReadUint64()
		if markerIndexErr != nil {
			return nil, errors.Errorf("failed to read sequence number from MarshalUtil: %w", votePowerErr)
		}

		latestMarkerVotes.latestMarkerVotes.Set(markerIndex, votePower)
	}

	return latestMarkerVotes, nil
}

// Voter returns the Voter for the LatestMarkerVotes.
func (l *LatestMarkerVotes) Voter() Voter {
	return l.voter
}

// Power returns the power of the vote for the given marker Index.
func (l *LatestMarkerVotes) Power(index markers.Index) (power VotePower, exists bool) {
	l.RLock()
	defer l.RUnlock()

	key, exists := l.latestMarkerVotes.Get(index)
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

	if maxElement := l.latestMarkerVotes.MaxElement(); maxElement != nil {
		previousHighestIndex = maxElement.Key()
	}

	// abort if we already have a higher value on an Index that is larger or equal
	_, ceilingValue, ceilingExists := l.latestMarkerVotes.Ceiling(index)
	if ceilingExists && power < ceilingValue {
		return false, previousHighestIndex
	}

	// set the new value
	l.latestMarkerVotes.Set(index, power)

	// remove all predecessors that are lower than the newly set value
	floorKey, floorValue, floorExists := l.latestMarkerVotes.Floor(index - 1)
	for floorExists && floorValue < power {
		l.latestMarkerVotes.Delete(floorKey)

		floorKey, floorValue, floorExists = l.latestMarkerVotes.Floor(index - 1)
	}

	l.SetModified()

	return true, previousHighestIndex
}

// String returns a human-readable version of the LatestMarkerVotes.
func (l *LatestMarkerVotes) String() string {
	builder := stringify.StructBuilder("LatestMarkerVotes")

	l.latestMarkerVotes.ForEach(func(node *thresholdmap.Element[markers.Index, VotePower]) bool {
		builder.AddField(stringify.StructField(node.Key().String(), node.Value()))

		return true
	})

	return builder.String()
}

// Bytes returns a marshaled version of the LatestMarkerVotes.
func (l *LatestMarkerVotes) Bytes() []byte {
	return byteutils.ConcatBytes(l.ObjectStorageKey(), l.ObjectStorageValue())
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
	l.latestMarkerVotes.ForEach(func(node *thresholdmap.Element[markers.Index, VotePower]) bool {
		marshalUtil.Write(node.Key())
		marshalUtil.WriteUint64(node.Value())

		return true
	})

	return marshalUtil.Bytes()
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
	voter             Voter
	latestBranchVotes map[ledgerstate.BranchID]*BranchVote

	sync.RWMutex
	objectstorage.StorableObjectFlags
}

// Vote returns the Vote for the LatestBranchVotes.
func (l *LatestBranchVotes) Vote(branchID ledgerstate.BranchID) (vote *BranchVote, exists bool) {
	l.RLock()
	defer l.RUnlock()

	vote, exists = l.latestBranchVotes[branchID]

	return
}

// Store stores the vote for the LatestBranchVotes.
func (l *LatestBranchVotes) Store(vote *BranchVote) (stored bool) {
	l.Lock()
	defer l.Unlock()

	if currentVote, exists := l.latestBranchVotes[vote.BranchID]; exists && currentVote.VotePower >= vote.VotePower {
		return false
	}

	l.latestBranchVotes[vote.BranchID] = vote
	l.SetModified()

	return true
}

// NewLatestBranchVotes creates a new LatestBranchVotes.
func NewLatestBranchVotes(voter Voter) (latestBranchVotes *LatestBranchVotes) {
	latestBranchVotes = &LatestBranchVotes{
		voter:             voter,
		latestBranchVotes: make(map[ledgerstate.BranchID]*BranchVote),
	}

	latestBranchVotes.Persist()
	latestBranchVotes.SetModified()

	return
}

// FromObjectStorage creates an LatestBranchVotes from sequences of key and bytes.
func (l *LatestBranchVotes) FromObjectStorage(key, bytes []byte) (objectstorage.StorableObject, error) {
	result, err := l.FromBytes(byteutils.ConcatBytes(key, bytes))
	if err != nil {
		err = errors.Errorf("failed to parse LatestBranchVotes from bytes: %w", err)
	}
	return result, err
}

// FromBytes unmarshals a LatestBranchVotes object from a sequence of bytes.
func (l *LatestBranchVotes) FromBytes(bytes []byte) (latestBranchVotes *LatestBranchVotes, err error) {
	marshalUtil := marshalutil.New(bytes)
	if latestBranchVotes, err = l.FromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse LatestBranchVotes from MarshalUtil: %w", err)
		return
	}

	return
}

// FromMarshalUtil unmarshals a LatestBranchVotes object using a MarshalUtil (for easier unmarshalling).
func (l *LatestBranchVotes) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (latestBranchVotes *LatestBranchVotes, err error) {
	if latestBranchVotes = l; l == nil {
		latestBranchVotes = new(LatestBranchVotes)
	}
	if latestBranchVotes.voter, err = identity.IDFromMarshalUtil(marshalUtil); err != nil {
		return nil, errors.Errorf("failed to parse Voter from MarshalUtil: %w", err)
	}

	mapSize, err := marshalUtil.ReadUint64()
	if err != nil {
		return nil, errors.Errorf("failed to parse map size (%v): %w", err, cerrors.ErrParseBytesFailed)
	}

	latestBranchVotes.latestBranchVotes = make(map[ledgerstate.BranchID]*BranchVote, int(mapSize))

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

// Bytes returns a marshaled version of the LatestBranchVotes.
func (l *LatestBranchVotes) Bytes() []byte {
	return byteutils.ConcatBytes(l.ObjectStorageKey(), l.ObjectStorageValue())
}

// String returns a human-readable version of the LatestBranchVotes.
func (l *LatestBranchVotes) String() string {
	return stringify.Struct("LatestBranchVotes",
		stringify.StructField("voter", l.voter),
	)
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
var _ objectstorage.StorableObject = new(LatestBranchVotes)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Vote /////////////////////////////////////////////////////////////////////////////////////////////////////////

// BranchVote represents a struct that holds information about what Opinion a certain Voter has on a Branch.
type BranchVote struct {
	Voter     Voter
	BranchID  ledgerstate.BranchID
	Opinion   Opinion
	VotePower VotePower
}

// VoteFromMarshalUtil unmarshals a Vote structure using a MarshalUtil (for easier unmarshalling).
func VoteFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (vote *BranchVote, err error) {
	vote = new(BranchVote)

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

	if vote.VotePower, err = marshalUtil.ReadUint64(); err != nil {
		return nil, errors.Errorf("failed to parse VotePower from MarshalUtil: %w", err)
	}

	return
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

// Bytes returns the bytes of the Vote.
func (v *BranchVote) Bytes() []byte {
	return marshalutil.New().
		Write(v.Voter).
		Write(v.BranchID).
		WriteUint8(uint8(v.Opinion)).
		WriteUint64(v.VotePower).
		Bytes()
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
