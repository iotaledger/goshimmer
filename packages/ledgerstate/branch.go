package ledgerstate

import (
	"sort"
	"strings"
	"sync"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/types"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"
	"golang.org/x/xerrors"
)

// region BranchID /////////////////////////////////////////////////////////////////////////////////////////////////////

var (
	// UndefinedBranchID is the zero value of a BranchID and represents a branch that has not been set.
	UndefinedBranchID = BranchID{}

	// MasterBranchID is the identifier of the MasterBranch (root of the ConflictBranch DAG).
	MasterBranchID = BranchID{1}
)

// BranchIDLength contains the amount of bytes that a marshaled version of the BranchID contains.
const BranchIDLength = 32

// BranchID is the data type that represents the identifier of a ConflictBranch.
type BranchID [BranchIDLength]byte

// NewBranchID creates a new BranchID from a TransactionID.
func NewBranchID(transactionID TransactionID) (branchID BranchID) {
	copy(branchID[:], transactionID[:])

	return
}

// BranchIDFromBytes unmarshals a BranchID from a sequence of bytes.
func BranchIDFromBytes(bytes []byte) (branchID BranchID, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if branchID, err = BranchIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse BranchID from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// BranchIDFromBase58 creates a BranchID from a base58 encoded string.
func BranchIDFromBase58(base58String string) (branchID BranchID, err error) {
	bytes, err := base58.Decode(base58String)
	if err != nil {
		err = xerrors.Errorf("error while decoding base58 encoded BranchID (%v): %w", err, ErrBase58DecodeFailed)
		return
	}

	if branchID, _, err = BranchIDFromBytes(bytes); err != nil {
		err = xerrors.Errorf("failed to parse BranchID from bytes: %w", err)
		return
	}

	return
}

// BranchIDFromMarshalUtil unmarshals a BranchID using a MarshalUtil (for easier unmarshaling).
func BranchIDFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (branchID BranchID, err error) {
	branchIDBytes, err := marshalUtil.ReadBytes(BranchIDLength)
	if err != nil {
		err = xerrors.Errorf("failed to parse BranchID (%v): %w", err, ErrParseBytesFailed)
		return
	}
	copy(branchID[:], branchIDBytes)

	return
}

// Bytes returns a marshaled version of the BranchID.
func (b BranchID) Bytes() []byte {
	return b[:]
}

// Base58 returns a base58 encoded version of the BranchID.
func (b BranchID) Base58() string {
	return base58.Encode(b.Bytes())
}

// String returns a human readable version of the BranchID.
func (b BranchID) String() string {
	switch b {
	case UndefinedBranchID:
		return "BranchID(UndefinedBranchID)"
	case MasterBranchID:
		return "BranchID(MasterBranchID)"
	default:
		return "BranchID(" + b.Base58() + ")"
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchIDs ////////////////////////////////////////////////////////////////////////////////////////////////////

// BranchIDs represents a collection of BranchIDs.
type BranchIDs map[BranchID]types.Empty

// NewBranchIDs creates a new collection of BranchIDs from the given BranchIDs.
func NewBranchIDs(branches ...BranchID) (branchIDs BranchIDs) {
	branchIDs = make(BranchIDs)
	for _, branchID := range branches {
		branchIDs[branchID] = types.Void
	}

	return
}

// BranchIDsFromBytes unmarshals a collection of BranchIDs from a sequence of bytes.
func BranchIDsFromBytes(bytes []byte) (branchIDs BranchIDs, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if branchIDs, err = BranchIDsFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse BranchIDs from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// BranchIDsFromMarshalUtil unmarshals a collection of BranchIDs using a MarshalUtil (for easier unmarshaling).
func BranchIDsFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (branchIDs BranchIDs, err error) {
	branchIDsCount, err := marshalUtil.ReadUint64()
	if err != nil {
		err = xerrors.Errorf("failed to parse BranchIDs count (%v): %w", err, ErrParseBytesFailed)
		return
	}

	branchIDs = make(BranchIDs)
	for i := uint64(0); i < branchIDsCount; i++ {
		branchID, branchIDErr := BranchIDFromMarshalUtil(marshalUtil)
		if branchIDErr != nil {
			err = xerrors.Errorf("failed to parse BranchID: %w", branchIDErr)
			return
		}

		branchIDs[branchID] = types.Void
	}

	return
}

// Slice creates a slice of BranchIDs from the collection.
func (b BranchIDs) Slice() (list []BranchID) {
	list = make([]BranchID, len(b))
	i := 0
	for branchID := range b {
		list[i] = branchID
		i++
	}

	return
}

// Bytes returns a marshaled version of the BranchIDs.
func (b BranchIDs) Bytes() []byte {
	marshalUtil := marshalutil.New(marshalutil.INT64_SIZE + len(b)*BranchIDLength)
	marshalUtil.WriteUint64(uint64(len(b)))
	for branchID := range b {
		marshalUtil.WriteBytes(branchID.Bytes())
	}

	return marshalUtil.Bytes()
}

// String returns a human readable version of the BranchIDs.
func (b BranchIDs) String() string {
	if len(b) == 0 {
		return "BranchIDs{}"
	}

	result := "BranchIDs{\n"
	for branchID := range b {
		result += strings.Repeat(" ", stringify.INDENTATION_SIZE) + branchID.String() + ",\n"
	}
	result += "}"

	return result
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BranchType ///////////////////////////////////////////////////////////////////////////////////////////////////

// BranchType represents the type of a Branch which can either be a ConflictBranch or an AggregatedBranch.
type BranchType uint8

const (
	// ConflictBranchType represents the type of a Branch that was created by a Transaction spending conflicting Outputs.
	ConflictBranchType BranchType = iota

	// AggregatedBranchType represents the type of a Branch that was created by combining Outputs of multiple
	// non-conflicting Branches.
	AggregatedBranchType
)

// String returns a human readable representation of the BranchType.
func (b BranchType) String() string {
	return [...]string{
		"ConflictBranchType",
		"AggregatedBranchType",
	}[b]
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Branch ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Branch is an interface for a container for Transactions and Outputs representing a certain perception of the ledger
// state.
type Branch interface {
	// ID returns the identifier of the Branch.
	ID() BranchID

	// Type returns the type of the Branch.
	Type() BranchType

	// Parents returns the BranchIDs of the Branches parents in the BranchDAG.
	Parents() BranchIDs

	// Preferred returns true if the branch is "liked within it's scope" (ignoring monotonicity).
	Preferred() bool

	// SetPreferred sets the preferred property to the given value. It returns true if the value has been updated.
	SetPreferred(preferred bool) (modified bool)

	// Liked returns true if the branch is liked (taking monotonicity in account - i.e. all parents are also liked).
	Liked() bool

	// SetLiked sets the liked property to the given value. It returns true if the value has been updated.
	SetLiked(liked bool) (modified bool)

	// Finalized returns true if the decision whether it is preferred has been finalized.
	Finalized() bool

	// SetFinalized sets the finalized property to the given value. It returns true if the value has been updated.
	SetFinalized(finalized bool) (modified bool)

	// Confirmed returns true if the decision that the Branch is liked has been finalized and all of its parents have
	// been confirmed.
	Confirmed() bool

	// SetConfirmed sets the confirmed property to the given value. It returns true if the value has been updated.
	SetConfirmed(confirmed bool) (modified bool)

	// Rejected returns true if either a decision that the Branch is not liked has been finalized or any of its
	// parents are rejected.
	Rejected() bool

	// SetRejected sets the rejected property to the given value. It returns true if the value has been updated.
	SetRejected(rejected bool) (modified bool)

	// Bytes returns a marshaled version of the Branch.
	Bytes() []byte

	// String returns a human readable version of the Branch.
	String() string

	// StorableObject enables the Branch to be stored in the ObjectStorage.
	objectstorage.StorableObject
}

// BranchFromBytes unmarshals a Branch from a sequence of bytes.
func BranchFromBytes(bytes []byte) (branch Branch, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if branch, err = BranchFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Branch from MarshalUtil: %w", err)
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// BranchFromMarshalUtil unmarshals a Branch using a MarshalUtil (for easier unmarshaling).
func BranchFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (branch Branch, err error) {
	branchType, err := marshalUtil.ReadByte()
	if err != nil {
		err = xerrors.Errorf("failed to parse BranchType (%v): %w", err, ErrParseBytesFailed)
		return
	}
	marshalUtil.ReadSeek(-1)

	switch BranchType(branchType) {
	case ConflictBranchType:
		if branch, err = ConflictBranchFromMarshalUtil(marshalUtil); err != nil {
			err = xerrors.Errorf("failed to parse ConflictBranch: %w", err)
			return
		}
	case AggregatedBranchType:
		if branch, err = AggregatedBranchFromMarshalUtil(marshalUtil); err != nil {
			err = xerrors.Errorf("failed to parse AggregatedBranch: %w", err)
			return
		}
	default:
		err = xerrors.Errorf("unsupported BranchType (%X): %w", branchType, ErrParseBytesFailed)
		return
	}

	return
}

// BranchFromObjectStorage restores a Branch that was stored in the ObjectStorage.
func BranchFromObjectStorage(_ []byte, data []byte) (branch objectstorage.StorableObject, err error) {
	if branch, _, err = BranchFromBytes(data); err != nil {
		err = xerrors.Errorf("failed to parse Branch from bytes: %w", err)
		return
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ConflictBranch ///////////////////////////////////////////////////////////////////////////////////////////////

// ConflictBranch represents a container for Transactions and Outputs representing a certain perception of the ledger state.
type ConflictBranch struct {
	id             BranchID
	parents        BranchIDs
	parentsMutex   sync.RWMutex
	conflicts      ConflictIDs
	conflictsMutex sync.RWMutex
	preferred      bool
	preferredMutex sync.RWMutex
	liked          bool
	likedMutex     sync.RWMutex
	finalized      bool
	finalizedMutex sync.RWMutex
	confirmed      bool
	confirmedMutex sync.RWMutex
	rejected       bool
	rejectedMutex  sync.RWMutex

	objectstorage.StorableObjectFlags
}

// NewConflictBranch creates a new ConflictBranch from the given details.
func NewConflictBranch(id BranchID, parents BranchIDs, conflicts ConflictIDs) *ConflictBranch {
	return &ConflictBranch{
		id:        id,
		parents:   parents,
		conflicts: conflicts,
	}
}

// ConflictBranchFromBytes unmarshals an ConflictBranch from a sequence of bytes.
func ConflictBranchFromBytes(bytes []byte) (conflictBranch *ConflictBranch, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if conflictBranch, err = ConflictBranchFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse ConflictBranch from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ConflictBranchFromMarshalUtil unmarshals an ConflictBranch using a MarshalUtil (for easier unmarshaling).
func ConflictBranchFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (conflictBranch *ConflictBranch, err error) {
	branchType, err := marshalUtil.ReadByte()
	if err != nil {
		err = xerrors.Errorf("failed to parse BranchType (%v): %w", err, ErrParseBytesFailed)
		return
	}
	if BranchType(branchType) != ConflictBranchType {
		err = xerrors.Errorf("invalid BranchType (%X): %w", branchType, ErrParseBytesFailed)
		return
	}

	conflictBranch = &ConflictBranch{}
	if conflictBranch.id, err = BranchIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse id: %w", err)
		return
	}
	if conflictBranch.parents, err = BranchIDsFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse parents: %w", err)
		return
	}
	if conflictBranch.conflicts, err = ConflictIDsFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse conflicts: %w", err)
		return
	}
	if conflictBranch.preferred, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse preferred flag (%v): %w", err, ErrParseBytesFailed)
		return
	}
	if conflictBranch.liked, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse liked flag (%v): %w", err, ErrParseBytesFailed)
		return
	}
	if conflictBranch.finalized, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse finalized flag (%v): %w", err, ErrParseBytesFailed)
		return
	}
	if conflictBranch.confirmed, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse confirmed flag (%v): %w", err, ErrParseBytesFailed)
		return
	}
	if conflictBranch.rejected, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse rejected flag (%v): %w", err, ErrParseBytesFailed)
		return
	}

	return
}

// ID returns the identifier of the Branch.
func (b *ConflictBranch) ID() BranchID {
	return b.id
}

// Type returns the type of the Branch.
func (b *ConflictBranch) Type() BranchType {
	return ConflictBranchType
}

// Parents returns the BranchIDs of the Branches parents in the BranchDAG.
func (b *ConflictBranch) Parents() BranchIDs {
	b.parentsMutex.RLock()
	defer b.parentsMutex.RUnlock()

	return b.parents
}

// SetParents updates the parents of the ConflictBranch.
func (b *ConflictBranch) SetParents(parentBranches ...BranchID) (modified bool, err error) {
	b.parentsMutex.Lock()
	defer b.parentsMutex.Unlock()

	b.parents = NewBranchIDs(parentBranches...)
	b.SetModified()
	modified = true

	return
}

// Conflicts returns the Conflicts that the ConflictBranch is part of.
func (b *ConflictBranch) Conflicts() (conflicts ConflictIDs) {
	b.conflictsMutex.RLock()
	defer b.conflictsMutex.RUnlock()

	conflicts = make(ConflictIDs)
	for conflict := range b.conflicts {
		conflicts[conflict] = types.Void
	}

	return
}

// AddConflict registers the membership of the ConflictBranch in the given Conflict.
func (b *ConflictBranch) AddConflict(conflictID ConflictID) (added bool) {
	b.conflictsMutex.Lock()
	defer b.conflictsMutex.Unlock()

	if _, exists := b.conflicts[conflictID]; exists {
		return
	}

	b.conflicts[conflictID] = types.Void
	b.SetModified()
	added = true

	return
}

// Preferred returns true if the branch is "liked within it's scope" (ignoring monotonicity).
func (b *ConflictBranch) Preferred() bool {
	b.preferredMutex.RLock()
	defer b.preferredMutex.RUnlock()

	return b.preferred
}

// SetPreferred sets the preferred property to the given value. It returns true if the value has been updated.
func (b *ConflictBranch) SetPreferred(preferred bool) (modified bool) {
	b.preferredMutex.Lock()
	defer b.preferredMutex.Unlock()

	if b.preferred == preferred {
		return
	}

	b.preferred = preferred
	b.SetModified()
	modified = true

	return
}

// Liked returns true if the branch is liked (taking monotonicity in account - i.e. all parents are also liked).
func (b *ConflictBranch) Liked() bool {
	b.likedMutex.RLock()
	defer b.likedMutex.RUnlock()

	return b.liked
}

// SetLiked sets the liked property to the given value. It returns true if the value has been updated.
func (b *ConflictBranch) SetLiked(liked bool) (modified bool) {
	b.likedMutex.Lock()
	defer b.likedMutex.Unlock()

	if b.liked == liked {
		return
	}

	b.liked = liked
	b.SetModified()
	modified = true

	return
}

// SetFinalized sets the finalized property to the given value. It returns true if the value has been updated.
func (b *ConflictBranch) Finalized() bool {
	b.finalizedMutex.RLock()
	defer b.finalizedMutex.RUnlock()

	return b.finalized
}

// SetFinalized is the setter for the finalized flag. It returns true if the value of the flag has been updated.
func (b *ConflictBranch) SetFinalized(finalized bool) (modified bool) {
	b.finalizedMutex.Lock()
	defer b.finalizedMutex.Unlock()

	if b.finalized == finalized {
		return
	}

	b.finalized = finalized
	b.SetModified()
	modified = true

	return
}

// Confirmed returns true if the decision that the Branch is liked has been finalized and all of its parents have
// been confirmed.
func (b *ConflictBranch) Confirmed() bool {
	b.confirmedMutex.RLock()
	defer b.confirmedMutex.RUnlock()

	return b.confirmed
}

// SetConfirmed is the setter for the confirmed flag. It returns true if the value of the flag has been updated.
func (b *ConflictBranch) SetConfirmed(confirmed bool) (modified bool) {
	b.confirmedMutex.Lock()
	defer b.confirmedMutex.Unlock()

	if b.confirmed == confirmed {
		return
	}

	b.confirmed = confirmed
	b.SetModified()
	modified = true

	return
}

// Rejected returns true if either a decision that the Branch is not liked has been finalized or any of its
// parents are rejected.
func (b *ConflictBranch) Rejected() bool {
	b.confirmedMutex.RLock()
	defer b.confirmedMutex.RUnlock()

	return b.confirmed
}

// SetRejected sets the rejected property to the given value. It returns true if the value has been updated.
func (b *ConflictBranch) SetRejected(rejected bool) (modified bool) {
	b.rejectedMutex.Lock()
	defer b.rejectedMutex.Unlock()

	if b.rejected == rejected {
		return
	}

	b.rejected = rejected
	b.SetModified()
	modified = true

	return
}

// Bytes returns a marshaled version of the Branch.
func (b *ConflictBranch) Bytes() []byte {
	return b.ObjectStorageValue()
}

// String returns a human readable version of the Branch.
func (b *ConflictBranch) String() string {
	return stringify.Struct("ConflictBranch",
		stringify.StructField("id", b.ID()),
		stringify.StructField("parents", b.Parents()),
		stringify.StructField("conflicts", b.Conflicts()),
		stringify.StructField("preferred", b.Preferred()),
		stringify.StructField("liked", b.Liked()),
		stringify.StructField("finalized", b.Finalized()),
		stringify.StructField("confirmed", b.Confirmed()),
		stringify.StructField("rejected", b.Rejected()),
	)
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (b *ConflictBranch) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (b *ConflictBranch) ObjectStorageKey() []byte {
	return b.ID().Bytes()
}

// ObjectStorageValue marshals the ConflictBranch into a sequence of bytes that are used as the value part in the
// ObjectStorage.
func (b *ConflictBranch) ObjectStorageValue() []byte {
	return marshalutil.New().
		WriteByte(byte(b.Type())).
		WriteBytes(b.ID().Bytes()).
		WriteBytes(b.Parents().Bytes()).
		WriteBytes(b.Conflicts().Bytes()).
		WriteBool(b.Preferred()).
		WriteBool(b.Liked()).
		WriteBool(b.Finalized()).
		WriteBool(b.Confirmed()).
		WriteBool(b.Rejected()).
		Bytes()
}

// code contract (make sure the struct implements all required methods)
var _ Branch = &ConflictBranch{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region AggregatedBranch ///////////////////////////////////////////////////////////////////////////////////////////////

// AggregatedBranch represents a container for Transactions and Outputs representing a certain perception of the ledger state.
type AggregatedBranch struct {
	id             BranchID
	parents        BranchIDs
	parentsMutex   sync.RWMutex
	preferred      bool
	preferredMutex sync.RWMutex
	liked          bool
	likedMutex     sync.RWMutex
	finalized      bool
	finalizedMutex sync.RWMutex
	confirmed      bool
	confirmedMutex sync.RWMutex
	rejected       bool
	rejectedMutex  sync.RWMutex

	objectstorage.StorableObjectFlags
}

// NewAggregatedBranch creates a new AggregatedBranch from the given details.
func NewAggregatedBranch(parents BranchIDs) *AggregatedBranch {
	// sort parents
	parentBranchIDs := parents.Slice()
	sort.Slice(parentBranchIDs, func(i, j int) bool {
		for k := 0; k < len(parentBranchIDs[k]); k++ {
			if parentBranchIDs[i][k] < parentBranchIDs[j][k] {
				return true
			} else if parentBranchIDs[i][k] > parentBranchIDs[j][k] {
				return false
			}
		}

		return false
	})

	// concatenate sorted parent bytes
	marshalUtil := marshalutil.New(BranchIDLength * len(parentBranchIDs))
	for _, branchID := range parentBranchIDs {
		marshalUtil.WriteBytes(branchID.Bytes())
	}

	// return result
	return &AggregatedBranch{
		id:      blake2b.Sum256(marshalUtil.Bytes()),
		parents: parents,
	}
}

// AggregatedBranchFromBytes unmarshals an AggregatedBranch from a sequence of bytes.
func AggregatedBranchFromBytes(bytes []byte) (conflictBranch *AggregatedBranch, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if conflictBranch, err = AggregatedBranchFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse AggregatedBranch from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// AggregatedBranchFromMarshalUtil unmarshals an AggregatedBranch using a MarshalUtil (for easier unmarshaling).
func AggregatedBranchFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (conflictBranch *AggregatedBranch, err error) {
	branchType, err := marshalUtil.ReadByte()
	if err != nil {
		err = xerrors.Errorf("failed to parse BranchType (%v): %w", err, ErrParseBytesFailed)
		return
	}
	if BranchType(branchType) != AggregatedBranchType {
		err = xerrors.Errorf("invalid BranchType (%X): %w", branchType, ErrParseBytesFailed)
		return
	}

	conflictBranch = &AggregatedBranch{}
	if conflictBranch.id, err = BranchIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse id: %w", err)
		return
	}
	if conflictBranch.parents, err = BranchIDsFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse parents: %w", err)
		return
	}
	if conflictBranch.preferred, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse preferred flag (%v): %w", err, ErrParseBytesFailed)
		return
	}
	if conflictBranch.liked, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse liked flag (%v): %w", err, ErrParseBytesFailed)
		return
	}
	if conflictBranch.finalized, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse finalized flag (%v): %w", err, ErrParseBytesFailed)
		return
	}
	if conflictBranch.confirmed, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse confirmed flag (%v): %w", err, ErrParseBytesFailed)
		return
	}
	if conflictBranch.rejected, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse rejected flag (%v): %w", err, ErrParseBytesFailed)
		return
	}

	return
}

// ID returns the identifier of the Branch.
func (b *AggregatedBranch) ID() BranchID {
	return b.id
}

// Type returns the type of the Branch.
func (b *AggregatedBranch) Type() BranchType {
	return AggregatedBranchType
}

// Parents returns the BranchIDs of the Branches parents in the BranchDAG.
func (b *AggregatedBranch) Parents() BranchIDs {
	b.parentsMutex.RLock()
	defer b.parentsMutex.RUnlock()

	return b.parents
}

// Preferred returns true if the branch is "liked within it's scope" (ignoring monotonicity).
func (b *AggregatedBranch) Preferred() bool {
	b.preferredMutex.RLock()
	defer b.preferredMutex.RUnlock()

	return b.preferred
}

// SetPreferred sets the preferred property to the given value. It returns true if the value has been updated.
func (b *AggregatedBranch) SetPreferred(preferred bool) (modified bool) {
	b.preferredMutex.Lock()
	defer b.preferredMutex.Unlock()

	if b.preferred == preferred {
		return
	}

	b.preferred = preferred
	b.SetModified()
	modified = true

	return
}

// Liked returns true if the branch is liked (taking monotonicity in account - i.e. all parents are also liked).
func (b *AggregatedBranch) Liked() bool {
	b.likedMutex.RLock()
	defer b.likedMutex.RUnlock()

	return b.liked
}

// SetLiked sets the liked property to the given value. It returns true if the value has been updated.
func (b *AggregatedBranch) SetLiked(liked bool) (modified bool) {
	b.likedMutex.Lock()
	defer b.likedMutex.Unlock()

	if b.liked == liked {
		return
	}

	b.liked = liked
	b.SetModified()
	modified = true

	return
}

// SetFinalized sets the finalized property to the given value. It returns true if the value has been updated.
func (b *AggregatedBranch) Finalized() bool {
	b.finalizedMutex.RLock()
	defer b.finalizedMutex.RUnlock()

	return b.finalized
}

// SetFinalized is the setter for the finalized flag. It returns true if the value of the flag has been updated.
func (b *AggregatedBranch) SetFinalized(finalized bool) (modified bool) {
	b.finalizedMutex.Lock()
	defer b.finalizedMutex.Unlock()

	if b.finalized == finalized {
		return
	}

	b.finalized = finalized
	b.SetModified()
	modified = true

	return
}

// Confirmed returns true if the decision that the Branch is liked has been finalized and all of its parents have
// been confirmed.
func (b *AggregatedBranch) Confirmed() bool {
	b.confirmedMutex.RLock()
	defer b.confirmedMutex.RUnlock()

	return b.confirmed
}

// SetConfirmed is the setter for the confirmed flag. It returns true if the value of the flag has been updated.
func (b *AggregatedBranch) SetConfirmed(confirmed bool) (modified bool) {
	b.confirmedMutex.Lock()
	defer b.confirmedMutex.Unlock()

	if b.confirmed == confirmed {
		return
	}

	b.confirmed = confirmed
	b.SetModified()
	modified = true

	return
}

// Rejected returns true if either a decision that the Branch is not liked has been finalized or any of its
// parents are rejected.
func (b *AggregatedBranch) Rejected() bool {
	b.confirmedMutex.RLock()
	defer b.confirmedMutex.RUnlock()

	return b.confirmed
}

// SetRejected sets the rejected property to the given value. It returns true if the value has been updated.
func (b *AggregatedBranch) SetRejected(rejected bool) (modified bool) {
	b.rejectedMutex.Lock()
	defer b.rejectedMutex.Unlock()

	if b.rejected == rejected {
		return
	}

	b.rejected = rejected
	b.SetModified()
	modified = true

	return
}

// Bytes returns a marshaled version of the Branch.
func (b *AggregatedBranch) Bytes() []byte {
	return b.ObjectStorageValue()
}

// String returns a human readable version of the Branch.
func (b *AggregatedBranch) String() string {
	return stringify.Struct("AggregatedBranch",
		stringify.StructField("id", b.ID()),
		stringify.StructField("parents", b.Parents()),
		stringify.StructField("preferred", b.Preferred()),
		stringify.StructField("liked", b.Liked()),
		stringify.StructField("finalized", b.Finalized()),
		stringify.StructField("confirmed", b.Confirmed()),
		stringify.StructField("rejected", b.Rejected()),
	)
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (b *AggregatedBranch) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (b *AggregatedBranch) ObjectStorageKey() []byte {
	return b.ID().Bytes()
}

// ObjectStorageValue marshals the AggregatedBranch into a sequence of bytes that are used as the value part in the
// ObjectStorage.
func (b *AggregatedBranch) ObjectStorageValue() []byte {
	return marshalutil.New().
		WriteByte(byte(b.Type())).
		WriteBytes(b.ID().Bytes()).
		WriteBytes(b.Parents().Bytes()).
		WriteBool(b.Preferred()).
		WriteBool(b.Liked()).
		WriteBool(b.Finalized()).
		WriteBool(b.Confirmed()).
		WriteBool(b.Rejected()).
		Bytes()
}

// code contract (make sure the struct implements all required methods)
var _ Branch = &AggregatedBranch{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
