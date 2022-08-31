package tangleold

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/model"
	"github.com/iotaledger/hive.go/core/generics/objectstorage"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/generics/thresholdmap"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/serix"
	"github.com/iotaledger/hive.go/core/stringify"

	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/markers"
)

// region ApprovalWeightManager Models /////////////////////////////////////////////////////////////////////////////////

// region ConflictWeight /////////////////////////////////////////////////////////////////////////////////////////////////

// ConflictWeight is a data structure that tracks the weight of a ConflictID.
type ConflictWeight struct {
	model.Storable[utxo.TransactionID, ConflictWeight, *ConflictWeight, float64] `serix:"0"`
}

// NewConflictWeight creates a new ConflictWeight.
func NewConflictWeight(conflictID utxo.TransactionID) (conflictWeight *ConflictWeight) {
	weight := 0.0
	conflictWeight = model.NewStorable[utxo.TransactionID, ConflictWeight](&weight)
	conflictWeight.SetID(conflictID)
	return
}

// ConflictID returns the ConflictID that is being tracked.
func (b *ConflictWeight) ConflictID() (conflictID utxo.TransactionID) {
	return b.ID()
}

// Weight returns the weight of the ConflictID.
func (b *ConflictWeight) Weight() (weight float64) {
	b.RLock()
	defer b.RUnlock()

	return b.M
}

// SetWeight sets the weight for the ConflictID and returns true if it was modified.
func (b *ConflictWeight) SetWeight(weight float64) bool {
	b.Lock()
	defer b.Unlock()

	if weight == b.M {
		return false
	}

	b.M = weight
	b.SetModified()

	return true
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
// region SerializableSet ////////////////////////////////////////////////////////////////////////////////////////////////////////

// Voter is a type wrapper for identity.ID and defines a node that supports a conflict or marker.
type Voter = identity.ID

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Voters ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Voters is a set of node identities that votes for a particular Conflict.
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
	structBuilder := stringify.NewStructBuilder("Voters")
	v.Set.ForEach(func(voter Voter) {
		structBuilder.AddField(stringify.NewStructField(voter.String(), "true"))
	})

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ConflictVoters /////////////////////////////////////////////////////////////////////////////////////////////////

// ConflictVoters is a data structure that tracks which nodes support a conflict.
type ConflictVoters struct {
	model.Storable[utxo.TransactionID, ConflictVoters, *ConflictVoters, Voters] `serix:"0"`
}

// NewConflictVoters is the constructor for the ConflictVoters object.
func NewConflictVoters(conflictID utxo.TransactionID) (conflictVoters *ConflictVoters) {
	conflictVoters = model.NewStorable[utxo.TransactionID, ConflictVoters](NewVoters())
	conflictVoters.SetID(conflictID)
	return
}

// ConflictID returns the ConflictID that is being tracked.
func (b *ConflictVoters) ConflictID() (conflictID utxo.TransactionID) {
	return b.ID()
}

// Has returns true if the given Voter is currently supporting this Conflict.
func (b *ConflictVoters) Has(voter Voter) bool {
	b.RLock()
	defer b.RUnlock()

	return b.M.Set.Has(voter)
}

// AddVoter adds a new Voter to the tracked ConflictID.
func (b *ConflictVoters) AddVoter(voter Voter) (added bool) {
	b.Lock()
	defer b.Unlock()

	if added = b.M.Set.Add(voter); !added {
		return
	}
	b.SetModified()

	return
}

// AddVoters adds the Voters set to the tracked ConflictID.
func (b *ConflictVoters) AddVoters(voters *Voters) (added bool) {
	voters.Set.ForEach(func(voter Voter) {
		if b.AddVoter(voter) {
			added = true
		}
	})

	if added {
		b.SetModified()
	}

	return
}

// DeleteVoter deletes a Voter from the tracked ConflictID.
func (b *ConflictVoters) DeleteVoter(voter Voter) (deleted bool) {
	b.Lock()
	defer b.Unlock()

	if deleted = b.M.Set.Delete(voter); !deleted {
		return
	}
	b.SetModified()

	return
}

// Voters returns the set of Voters that are supporting the given ConflictID.
func (b *ConflictVoters) Voters() (voters *Voters) {
	b.RLock()
	defer b.RUnlock()

	return b.M.Clone()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Opinion //////////////////////////////////////////////////////////////////////////////////////////////////////

// Opinion is a type that represents the Opinion of a node on a certain Conflict.
type Opinion uint8

const (
	// UndefinedOpinion represents the zero value of the Opinion type.
	UndefinedOpinion Opinion = iota

	// Confirmed represents the Opinion that a given Conflict is the winning one.
	Confirmed

	// Rejected represents the Opinion that a given Conflict is the loosing one.
	Rejected
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region LatestMarkerVotes ////////////////////////////////////////////////////////////////////////////////////////////

// VotePower is used to establish an absolute order of votes, regardless of their arrival order.
// Currently, the used VotePower is the SequenceNumber embedded in the Block Layout, so that, regardless
// of the order in which votes are received, the same conclusion is computed.
// Alternatively, the objective timestamp of a Block could be used.
type VotePower = uint64

// LatestMarkerVotesKeyPartition defines the partition of the storage key of the LastMarkerVotes model.
var LatestMarkerVotesKeyPartition = objectstorage.PartitionKey(markers.SequenceID(0).Length(), identity.IDLength)

// LatestMarkerVotes keeps track of the most up-to-date for a certain Voter casted on a specific Marker SequenceID.
// Votes can be casted on Markers (SequenceID, Index), but can arrive in any arbitrary order.
// Due to the nature of a Sequence, a vote casted for a certain Index clobbers votes for every lower index.
// Similarly, if a vote for an Index is casted and an existing vote for an higher Index exists, the operation has no effect.
type LatestMarkerVotes struct {
	model.StorableReferenceWithMetadata[LatestMarkerVotes, *LatestMarkerVotes, markers.SequenceID, Voter, thresholdmap.ThresholdMap[markers.Index, VotePower]] `serix:"0"`
}

// NewLatestMarkerVotes creates a new NewLatestMarkerVotes instance associated with the given details.
func NewLatestMarkerVotes(sequenceID markers.SequenceID, voter Voter) (newLatestMarkerVotes *LatestMarkerVotes) {
	newLatestMarkerVotes = model.NewStorableReferenceWithMetadata[LatestMarkerVotes](
		sequenceID, voter, thresholdmap.New[markers.Index, VotePower](thresholdmap.UpperThresholdMode),
	)

	return
}

// Voter returns the Voter for the LatestMarkerVotes.
func (l *LatestMarkerVotes) Voter() Voter {
	l.RLock()
	defer l.RUnlock()
	return l.TargetID()
}

// Power returns the power of the vote for the given marker Index.
func (l *LatestMarkerVotes) Power(index markers.Index) (power VotePower, exists bool) {
	l.RLock()
	defer l.RUnlock()

	key, exists := l.M.Get(index)
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

	if maxElement := l.M.MaxElement(); maxElement != nil {
		previousHighestIndex = maxElement.Key()
	}

	// abort if we already have a higher value on an Index that is larger or equal
	_, ceilingValue, ceilingExists := l.M.Ceiling(index)
	if ceilingExists && power < ceilingValue {
		return false, previousHighestIndex
	}

	// set the new value
	l.M.Set(index, power)

	// remove all predecessors that are lower than the newly set value
	floorKey, floorValue, floorExists := l.M.Floor(index - 1)
	for floorExists && floorValue < power {
		l.M.Delete(floorKey)

		floorKey, floorValue, floorExists = l.M.Floor(index - 1)
	}

	l.SetModified()

	return true, previousHighestIndex
}

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

// region LatestConflictVotes ////////////////////////////////////////////////////////////////////////////////////////////

// LatestConflictVotes represents the conflict supported from an Issuer.
type LatestConflictVotes struct {
	model.Storable[Voter, LatestConflictVotes, *LatestConflictVotes, latestConflictVotesModel] `serix:"0"`
}

type latestConflictVotesModel struct {
	LatestConflictVotes map[utxo.TransactionID]*ConflictVote `serix:"0,lengthPrefixType=uint32"`
}

// NewLatestConflictVotes creates a new LatestConflictVotes.
func NewLatestConflictVotes(voter Voter) (latestConflictVotes *LatestConflictVotes) {
	latestConflictVotes = model.NewStorable[Voter, LatestConflictVotes](
		&latestConflictVotesModel{
			LatestConflictVotes: make(map[utxo.TransactionID]*ConflictVote),
		},
	)
	latestConflictVotes.SetID(voter)
	return
}

// Vote returns the Vote for the LatestConflictVotes.
func (l *LatestConflictVotes) Vote(conflictID utxo.TransactionID) (vote *ConflictVote, exists bool) {
	l.RLock()
	defer l.RUnlock()

	vote, exists = l.M.LatestConflictVotes[conflictID]

	return
}

// Store stores the vote for the LatestConflictVotes.
func (l *LatestConflictVotes) Store(vote *ConflictVote) (stored bool) {
	l.Lock()
	defer l.Unlock()

	if currentVote, exists := l.M.LatestConflictVotes[vote.M.ConflictID]; exists && currentVote.M.VotePower >= vote.M.VotePower {
		return false
	}

	l.M.LatestConflictVotes[vote.M.ConflictID] = vote
	l.SetModified()

	return true
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Vote /////////////////////////////////////////////////////////////////////////////////////////////////////////

// ConflictVote represents a struct that holds information about what Opinion a certain Voter has on a Conflict.
type ConflictVote struct {
	model.Mutable[ConflictVote, *ConflictVote, conflictVoteModel] `serix:"0"`
}

type conflictVoteModel struct {
	Voter      Voter              `serix:"0"`
	ConflictID utxo.TransactionID `serix:"1"`
	Opinion    Opinion            `serix:"2"`
	VotePower  VotePower          `serix:"3"`
}

// NewConflictVote derives a vote for th.
func NewConflictVote(voter Voter, votePower VotePower, conflictID utxo.TransactionID, opinion Opinion) (voteWithOpinion *ConflictVote) {
	return model.NewMutable[ConflictVote](
		&conflictVoteModel{
			Voter:      voter,
			VotePower:  votePower,
			ConflictID: conflictID,
			Opinion:    opinion,
		},
	)
}

// WithOpinion derives a vote for the given Opinion.
func (v *ConflictVote) WithOpinion(opinion Opinion) (voteWithOpinion *ConflictVote) {
	v.RLock()
	defer v.RUnlock()
	return model.NewMutable[ConflictVote](
		&conflictVoteModel{
			Voter:      v.M.Voter,
			ConflictID: v.M.ConflictID,
			Opinion:    opinion,
			VotePower:  v.M.VotePower,
		},
	)
}

// WithConflictID derives a vote for the given ConflictID.
func (v *ConflictVote) WithConflictID(conflictID utxo.TransactionID) (rejectedVote *ConflictVote) {
	v.RLock()
	defer v.RUnlock()
	return model.NewMutable[ConflictVote](
		&conflictVoteModel{
			Voter:      v.M.Voter,
			ConflictID: conflictID,
			Opinion:    v.M.Opinion,
			VotePower:  v.M.VotePower,
		},
	)
}

func (v *ConflictVote) Voter() Voter {
	v.RLock()
	defer v.RUnlock()
	return v.M.Voter
}

func (v *ConflictVote) ConflictID() utxo.TransactionID {
	v.RLock()
	defer v.RUnlock()
	return v.M.ConflictID
}

func (v *ConflictVote) Opinion() Opinion {
	v.RLock()
	defer v.RUnlock()
	return v.M.Opinion
}

func (v *ConflictVote) VotePower() VotePower {
	v.RLock()
	defer v.RUnlock()
	return v.M.VotePower
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
