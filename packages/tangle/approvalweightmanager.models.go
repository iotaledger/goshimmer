package tangle

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/model"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/generics/thresholdmap"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/serix"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/markers"
)

// region ApprovalWeightManager Models /////////////////////////////////////////////////////////////////////////////////

// region BranchWeight /////////////////////////////////////////////////////////////////////////////////////////////////

// BranchWeight is a data structure that tracks the weight of a BranchID.
type BranchWeight struct {
	model.Storable[utxo.TransactionID, BranchWeight, *BranchWeight, float64] `serix:"0"`
}

// NewBranchWeight creates a new BranchWeight.
func NewBranchWeight(branchID utxo.TransactionID) (branchWeight *BranchWeight) {
	weight := 0.0
	branchWeight = model.NewStorable[utxo.TransactionID, BranchWeight](&weight)
	branchWeight.SetID(branchID)
	return
}

// BranchID returns the BranchID that is being tracked.
func (b *BranchWeight) BranchID() (branchID utxo.TransactionID) {
	return b.ID()
}

// Weight returns the weight of the BranchID.
func (b *BranchWeight) Weight() (weight float64) {
	b.RLock()
	defer b.RUnlock()

	return b.M
}

// SetWeight sets the weight for the BranchID and returns true if it was modified.
func (b *BranchWeight) SetWeight(weight float64) bool {
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
	model.Storable[utxo.TransactionID, BranchVoters, *BranchVoters, Voters] `serix:"0"`
}

// NewBranchVoters is the constructor for the BranchVoters object.
func NewBranchVoters(branchID utxo.TransactionID) (branchVoters *BranchVoters) {
	branchVoters = model.NewStorable[utxo.TransactionID, BranchVoters](NewVoters())
	branchVoters.SetID(branchID)
	return
}

// BranchID returns the BranchID that is being tracked.
func (b *BranchVoters) BranchID() (branchID utxo.TransactionID) {
	return b.ID()
}

// Has returns true if the given Voter is currently supporting this Branch.
func (b *BranchVoters) Has(voter Voter) bool {
	b.RLock()
	defer b.RUnlock()

	return b.M.Set.Has(voter)
}

// AddVoter adds a new Voter to the tracked BranchID.
func (b *BranchVoters) AddVoter(voter Voter) (added bool) {
	b.Lock()
	defer b.Unlock()

	if added = b.M.Set.Add(voter); !added {
		return
	}
	b.SetModified()

	return
}

// AddVoters adds the Voters set to the tracked BranchID.
func (b *BranchVoters) AddVoters(voters *Voters) (added bool) {
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

// DeleteVoter deletes a Voter from the tracked BranchID.
func (b *BranchVoters) DeleteVoter(voter Voter) (deleted bool) {
	b.Lock()
	defer b.Unlock()

	if deleted = b.M.Set.Delete(voter); !deleted {
		return
	}
	b.SetModified()

	return
}

// Voters returns the set of Voters that are supporting the given BranchID.
func (b *BranchVoters) Voters() (voters *Voters) {
	b.RLock()
	defer b.RUnlock()

	return b.M.Clone()
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

// VotePower is used to establish an absolute order of votes, regardless of their arrival order.
// Currently, the used VotePower is the SequenceNumber embedded in the Message Layout, so that, regardless
// of the order in which votes are received, the same conclusion is computed.
// Alternatively, the objective timestamp of a Message could be used.
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

// region LatestBranchVotes ////////////////////////////////////////////////////////////////////////////////////////////

// LatestBranchVotes represents the branch supported from an Issuer.
type LatestBranchVotes struct {
	model.Storable[Voter, LatestBranchVotes, *LatestBranchVotes, latestBranchVotesModel] `serix:"0"`
}

type latestBranchVotesModel struct {
	LatestBranchVotes map[utxo.TransactionID]*BranchVote `serix:"0,lengthPrefixType=uint32"`
}

// NewLatestBranchVotes creates a new LatestBranchVotes.
func NewLatestBranchVotes(voter Voter) (latestBranchVotes *LatestBranchVotes) {
	latestBranchVotes = model.NewStorable[Voter, LatestBranchVotes](
		&latestBranchVotesModel{
			LatestBranchVotes: make(map[utxo.TransactionID]*BranchVote),
		},
	)
	latestBranchVotes.SetID(voter)
	return
}

// Vote returns the Vote for the LatestBranchVotes.
func (l *LatestBranchVotes) Vote(branchID utxo.TransactionID) (vote *BranchVote, exists bool) {
	l.RLock()
	defer l.RUnlock()

	vote, exists = l.M.LatestBranchVotes[branchID]

	return
}

// Store stores the vote for the LatestBranchVotes.
func (l *LatestBranchVotes) Store(vote *BranchVote) (stored bool) {
	l.Lock()
	defer l.Unlock()

	if currentVote, exists := l.M.LatestBranchVotes[vote.M.BranchID]; exists && currentVote.M.VotePower >= vote.M.VotePower {
		return false
	}

	l.M.LatestBranchVotes[vote.M.BranchID] = vote
	l.SetModified()

	return true
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Vote /////////////////////////////////////////////////////////////////////////////////////////////////////////

// BranchVote represents a struct that holds information about what Opinion a certain Voter has on a Branch.
type BranchVote struct {
	model.Mutable[BranchVote, *BranchVote, branchVoteModel] `serix:"0"`
}

type branchVoteModel struct {
	Voter     Voter              `serix:"0"`
	BranchID  utxo.TransactionID `serix:"1"`
	Opinion   Opinion            `serix:"2"`
	VotePower VotePower          `serix:"3"`
}

// NewBranchVote derives a vote for th.
func NewBranchVote(voter Voter, votePower VotePower, branchID utxo.TransactionID, opinion Opinion) (voteWithOpinion *BranchVote) {
	return model.NewMutable[BranchVote](
		&branchVoteModel{
			Voter:     voter,
			VotePower: votePower,
			BranchID:  branchID,
			Opinion:   opinion,
		},
	)
}

// WithOpinion derives a vote for the given Opinion.
func (v *BranchVote) WithOpinion(opinion Opinion) (voteWithOpinion *BranchVote) {
	v.RLock()
	defer v.RUnlock()
	return model.NewMutable[BranchVote](
		&branchVoteModel{
			Voter:     v.M.Voter,
			BranchID:  v.M.BranchID,
			Opinion:   opinion,
			VotePower: v.M.VotePower,
		},
	)
}

// WithBranchID derives a vote for the given BranchID.
func (v *BranchVote) WithBranchID(branchID utxo.TransactionID) (rejectedVote *BranchVote) {
	v.RLock()
	defer v.RUnlock()
	return model.NewMutable[BranchVote](
		&branchVoteModel{
			Voter:     v.M.Voter,
			BranchID:  branchID,
			Opinion:   v.M.Opinion,
			VotePower: v.M.VotePower,
		},
	)
}

func (v *BranchVote) Voter() Voter {
	v.RLock()
	defer v.RUnlock()
	return v.M.Voter
}

func (v *BranchVote) BranchID() utxo.TransactionID {
	v.RLock()
	defer v.RUnlock()
	return v.M.BranchID
}

func (v *BranchVote) Opinion() Opinion {
	v.RLock()
	defer v.RUnlock()
	return v.M.Opinion
}

func (v *BranchVote) VotePower() VotePower {
	v.RLock()
	defer v.RUnlock()
	return v.M.VotePower
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
