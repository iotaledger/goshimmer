package ledgerstate

import (
	"bytes"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
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

	// LazyBookedConflictsBranchID is the identifier of the Branch that is the root of all lazy booked ConflictBranches.
	LazyBookedConflictsBranchID = BranchID{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 254}

	// InvalidBranchID is the identifier of the Branch that contains the invalid Transactions.
	InvalidBranchID = BranchID{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255}
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
	decodedBytes, err := base58.Decode(base58String)
	if err != nil {
		err = xerrors.Errorf("error while decoding base58 encoded BranchID (%v): %w", err, cerrors.ErrBase58DecodeFailed)
		return
	}

	if branchID, _, err = BranchIDFromBytes(decodedBytes); err != nil {
		err = xerrors.Errorf("failed to parse BranchID from bytes: %w", err)
		return
	}

	return
}

// BranchIDFromMarshalUtil unmarshals a BranchID using a MarshalUtil (for easier unmarshaling).
func BranchIDFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (branchID BranchID, err error) {
	branchIDBytes, err := marshalUtil.ReadBytes(BranchIDLength)
	if err != nil {
		err = xerrors.Errorf("failed to parse BranchID (%v): %w", err, cerrors.ErrParseBytesFailed)
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
		err = xerrors.Errorf("failed to parse BranchIDs count (%v): %w", err, cerrors.ErrParseBytesFailed)
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

// Add adds a BranchID to the collection and returns the collection to enable chaining.
func (b BranchIDs) Add(branchID BranchID) BranchIDs {
	b[branchID] = types.Void

	return b
}

// Contains checks if the given target BranchID is part of the BranchIDs.
func (b BranchIDs) Contains(targetBranchID BranchID) (contains bool) {
	for branchID := range b {
		if contains = branchID == targetBranchID; contains {
			return
		}
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
	marshalUtil := marshalutil.New(marshalutil.Int64Size + len(b)*BranchIDLength)
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

// Clone creates a copy of the BranchIDs.
func (b BranchIDs) Clone() (clonedBranchIDs BranchIDs) {
	clonedBranchIDs = make(BranchIDs)
	for branchID := range b {
		clonedBranchIDs[branchID] = types.Void
	}

	return
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

// BranchTypeFromBytes unmarshals a BranchType from a sequence of bytes.
func BranchTypeFromBytes(branchTypeBytes []byte) (branchType BranchType, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(branchTypeBytes)
	if branchType, err = BranchTypeFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse BranchType from MarshalUtil: %w", err)
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// BranchTypeFromMarshalUtil unmarshals a BranchType using a MarshalUtil (for easier unmarshaling).
func BranchTypeFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (branchType BranchType, err error) {
	branchTypeByte, err := marshalUtil.ReadByte()
	if err != nil {
		err = xerrors.Errorf("failed to parse BranchType (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	switch branchType = BranchType(branchTypeByte); branchType {
	case ConflictBranchType:
		return
	case AggregatedBranchType:
		return
	default:
		err = xerrors.Errorf("invalid BranchType (%X): %w", branchTypeByte, cerrors.ErrParseBytesFailed)
		return
	}
}

// Bytes returns a marshaled version of the BranchType.
func (b BranchType) Bytes() []byte {
	return []byte{byte(b)}
}

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

	// Liked returns true if the branch is "liked within it's scope" (ignoring monotonicity).
	Liked() bool

	// SetLiked sets the liked property to the given value. It returns true if the value has been updated.
	SetLiked(liked bool) (modified bool)

	// MonotonicallyLiked returns true if the branch is monotonically liked (all parents are also liked).
	MonotonicallyLiked() bool

	// SetMonotonicallyLiked sets the monotonically liked property to the given value. It returns true if the value has
	// been updated.
	SetMonotonicallyLiked(monotonicallyLiked bool) (modified bool)

	// Finalized returns true if the decision whether it is liked has been finalized.
	Finalized() bool

	// SetFinalized sets the finalized property to the given value. It returns true if the value has been updated.
	SetFinalized(finalized bool) (modified bool)

	// InclusionState returns the InclusionState of the Branch which encodes if the Branch has been included in the
	// ledger state.
	InclusionState() InclusionState

	// SetInclusionState sets the InclusionState of the Branch which encodes if the Branch has been included in the
	// ledger state. It returns true if the value has been updated.
	SetInclusionState(inclusionState InclusionState) (modified bool)

	// Bytes returns a marshaled version of the Branch.
	Bytes() []byte

	// String returns a human readable version of the Branch.
	String() string

	// StorableObject enables the Branch to be stored in the object storage.
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
		err = xerrors.Errorf("failed to parse BranchType (%v): %w", err, cerrors.ErrParseBytesFailed)
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
		err = xerrors.Errorf("unsupported BranchType (%X): %w", branchType, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// BranchFromObjectStorage restores a Branch that was stored in the object storage.
func BranchFromObjectStorage(_ []byte, data []byte) (branch objectstorage.StorableObject, err error) {
	if branch, _, err = BranchFromBytes(data); err != nil {
		err = xerrors.Errorf("failed to parse Branch from bytes: %w", err)
		return
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedBranch /////////////////////////////////////////////////////////////////////////////////////////////////

// CachedBranch is a wrapper for the generic CachedObject returned by the object storage that overrides the accessor
// methods with a type-casted one.
type CachedBranch struct {
	objectstorage.CachedObject
}

// ID returns the BranchID of the requested Branch.
func (c *CachedBranch) ID() (branchID BranchID) {
	branchID, _, err := BranchIDFromBytes(c.Key())
	if err != nil {
		panic(err)
	}

	return
}

// Retain marks the CachedObject to still be in use by the program.
func (c *CachedBranch) Retain() *CachedBranch {
	return &CachedBranch{c.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (c *CachedBranch) Unwrap() Branch {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(Branch)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// UnwrapConflictBranch is a more specialized Unwrap method that returns a ConflictBranch instead of the more generic interface.
func (c *CachedBranch) UnwrapConflictBranch() (conflictBranch *ConflictBranch, err error) {
	branch := c.Unwrap()
	if branch == nil {
		return
	}

	conflictBranch, typeCastOK := branch.(*ConflictBranch)
	if !typeCastOK {
		err = xerrors.Errorf("CachedBranch does not contain a ConflictBranch: %w", cerrors.ErrFatal)
		return
	}

	return
}

// UnwrapAggregatedBranch is a more specialized Unwrap method that returns an AggregatedBranch instead of the more generic interface.
func (c *CachedBranch) UnwrapAggregatedBranch() (aggregatedBranch *AggregatedBranch, err error) {
	branch := c.Unwrap()
	if branch == nil {
		return
	}

	aggregatedBranch, typeCastOK := branch.(*AggregatedBranch)
	if !typeCastOK {
		err = xerrors.Errorf("CachedBranch does not contain an AggregatedBranch: %w", cerrors.ErrFatal)
		return
	}

	return
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (c *CachedBranch) Consume(consumer func(branch Branch), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(Branch))
	}, forceRelease...)
}

// String returns a human readable version of the CachedBranch.
func (c *CachedBranch) String() string {
	return stringify.Struct("CachedBranch",
		stringify.StructField("CachedObject", c.Unwrap()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ConflictBranch ///////////////////////////////////////////////////////////////////////////////////////////////

// ConflictBranch represents a container for Transactions and Outputs representing a certain perception of the ledger
// state.
type ConflictBranch struct {
	id                      BranchID
	parents                 BranchIDs
	parentsMutex            sync.RWMutex
	conflicts               ConflictIDs
	conflictsMutex          sync.RWMutex
	liked                   bool
	likedMutex              sync.RWMutex
	monotonicallyLiked      bool
	monotonicallyLikedMutex sync.RWMutex
	finalized               bool
	finalizedMutex          sync.RWMutex
	inclusionState          InclusionState
	inclusionStateMutex     sync.RWMutex

	objectstorage.StorableObjectFlags
}

// NewConflictBranch creates a new ConflictBranch from the given details.
func NewConflictBranch(id BranchID, parents BranchIDs, conflicts ConflictIDs) *ConflictBranch {
	return &ConflictBranch{
		id:        id,
		parents:   parents.Clone(),
		conflicts: conflicts.Clone(),
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
		err = xerrors.Errorf("failed to parse BranchType (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if BranchType(branchType) != ConflictBranchType {
		err = xerrors.Errorf("invalid BranchType (%X): %w", branchType, cerrors.ErrParseBytesFailed)
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
	if conflictBranch.liked, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse liked flag (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if conflictBranch.monotonicallyLiked, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse monotonicallyLiked flag (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if conflictBranch.finalized, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse finalized flag (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if conflictBranch.inclusionState, err = InclusionStateFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse InclusionState from MarshalUtil: %w", err)
		return
	}

	return
}

// ID returns the identifier of the Branch.
func (c *ConflictBranch) ID() BranchID {
	return c.id
}

// Type returns the type of the Branch.
func (c *ConflictBranch) Type() BranchType {
	return ConflictBranchType
}

// Parents returns the BranchIDs of the Branches parents in the BranchDAG.
func (c *ConflictBranch) Parents() BranchIDs {
	c.parentsMutex.RLock()
	defer c.parentsMutex.RUnlock()

	return c.parents
}

// SetParents updates the parents of the ConflictBranch.
func (c *ConflictBranch) SetParents(parentBranches BranchIDs) (modified bool) {
	c.parentsMutex.Lock()
	defer c.parentsMutex.Unlock()

	c.parents = parentBranches
	c.SetModified()
	modified = true

	return
}

// Conflicts returns the Conflicts that the ConflictBranch is part of.
func (c *ConflictBranch) Conflicts() (conflicts ConflictIDs) {
	c.conflictsMutex.RLock()
	defer c.conflictsMutex.RUnlock()

	conflicts = c.conflicts.Clone()

	return
}

// AddConflict registers the membership of the ConflictBranch in the given Conflict.
func (c *ConflictBranch) AddConflict(conflictID ConflictID) (added bool) {
	c.conflictsMutex.Lock()
	defer c.conflictsMutex.Unlock()

	if _, exists := c.conflicts[conflictID]; exists {
		return
	}

	c.conflicts[conflictID] = types.Void
	c.SetModified()
	added = true

	return
}

// Liked returns true if the branch is "liked within it's scope" (ignoring monotonicity).
func (c *ConflictBranch) Liked() bool {
	c.likedMutex.RLock()
	defer c.likedMutex.RUnlock()

	return c.liked
}

// SetLiked sets the liked property to the given value. It returns true if the value has been updated.
func (c *ConflictBranch) SetLiked(liked bool) (modified bool) {
	c.likedMutex.Lock()
	defer c.likedMutex.Unlock()

	if c.liked == liked {
		return
	}

	c.liked = liked
	c.SetModified()
	modified = true

	return
}

// MonotonicallyLiked returns true if the branch is monotonically liked (all parents are also liked).
func (c *ConflictBranch) MonotonicallyLiked() bool {
	c.monotonicallyLikedMutex.RLock()
	defer c.monotonicallyLikedMutex.RUnlock()

	return c.monotonicallyLiked
}

// SetMonotonicallyLiked sets the monotonically liked property to the given value. It returns true if the value has been
// updated.
func (c *ConflictBranch) SetMonotonicallyLiked(monotonicallyLiked bool) (modified bool) {
	c.monotonicallyLikedMutex.Lock()
	defer c.monotonicallyLikedMutex.Unlock()

	if c.monotonicallyLiked == monotonicallyLiked {
		return
	}

	c.monotonicallyLiked = monotonicallyLiked
	c.SetModified()
	modified = true

	return
}

// Finalized returns true if the decision whether it is liked has been finalized.
func (c *ConflictBranch) Finalized() bool {
	c.finalizedMutex.RLock()
	defer c.finalizedMutex.RUnlock()

	return c.finalized
}

// SetFinalized is the setter for the finalized flag. It returns true if the value of the flag has been updated.
func (c *ConflictBranch) SetFinalized(finalized bool) (modified bool) {
	c.finalizedMutex.Lock()
	defer c.finalizedMutex.Unlock()

	if c.finalized == finalized {
		return
	}

	c.finalized = finalized
	c.SetModified()
	modified = true

	return
}

// InclusionState returns the InclusionState of the Branch which encodes if the Branch has been included in the
// ledger state.
func (c *ConflictBranch) InclusionState() (inclusionState InclusionState) {
	c.inclusionStateMutex.RLock()
	defer c.inclusionStateMutex.RUnlock()

	return c.inclusionState
}

// SetInclusionState sets the InclusionState of the Branch which encodes if the Branch has been included in the
// ledger state. It returns true if the value has been updated.
func (c *ConflictBranch) SetInclusionState(inclusionState InclusionState) (modified bool) {
	c.inclusionStateMutex.Lock()
	defer c.inclusionStateMutex.Unlock()

	if c.inclusionState == inclusionState {
		return
	}

	c.inclusionState = inclusionState
	c.SetModified()
	modified = true

	return
}

// Bytes returns a marshaled version of the Branch.
func (c *ConflictBranch) Bytes() []byte {
	return c.ObjectStorageValue()
}

// String returns a human readable version of the Branch.
func (c *ConflictBranch) String() string {
	return stringify.Struct("ConflictBranch",
		stringify.StructField("id", c.ID()),
		stringify.StructField("parents", c.Parents()),
		stringify.StructField("conflicts", c.Conflicts()),
		stringify.StructField("liked", c.Liked()),
		stringify.StructField("monotonicallyLiked", c.MonotonicallyLiked()),
		stringify.StructField("finalized", c.Finalized()),
		stringify.StructField("inclusionState", c.InclusionState()),
	)
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (c *ConflictBranch) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (c *ConflictBranch) ObjectStorageKey() []byte {
	return c.ID().Bytes()
}

// ObjectStorageValue marshals the ConflictBranch into a sequence of bytes that are used as the value part in the
// object storage.
func (c *ConflictBranch) ObjectStorageValue() []byte {
	return marshalutil.New().
		WriteByte(byte(c.Type())).
		WriteBytes(c.ID().Bytes()).
		WriteBytes(c.Parents().Bytes()).
		WriteBytes(c.Conflicts().Bytes()).
		WriteBool(c.Liked()).
		WriteBool(c.MonotonicallyLiked()).
		WriteBool(c.Finalized()).
		Write(c.InclusionState()).
		Bytes()
}

// code contract (make sure the struct implements all required methods)
var _ Branch = &ConflictBranch{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region AggregatedBranch /////////////////////////////////////////////////////////////////////////////////////////////

// AggregatedBranch represents a container for Transactions and Outputs representing a certain perception of the ledger
// state.
type AggregatedBranch struct {
	id                      BranchID
	parents                 BranchIDs
	parentsMutex            sync.RWMutex
	liked                   bool
	likedMutex              sync.RWMutex
	monotonicallyLiked      bool
	monotonicallyLikedMutex sync.RWMutex
	finalized               bool
	finalizedMutex          sync.RWMutex
	inclusionState          InclusionState
	inclusionStateMutex     sync.RWMutex

	objectstorage.StorableObjectFlags
}

// NewAggregatedBranch creates a new AggregatedBranch from the given details.
func NewAggregatedBranch(parents BranchIDs) *AggregatedBranch {
	// sort parents
	parentBranchIDs := parents.Slice()
	sort.Slice(parentBranchIDs, func(i, j int) bool {
		return bytes.Compare(parentBranchIDs[i].Bytes(), parentBranchIDs[j].Bytes()) < 0
	})

	// concatenate sorted parent bytes
	marshalUtil := marshalutil.New(BranchIDLength * len(parentBranchIDs))
	for _, branchID := range parentBranchIDs {
		marshalUtil.WriteBytes(branchID.Bytes())
	}

	// return result
	return &AggregatedBranch{
		id:      blake2b.Sum256(marshalUtil.Bytes()),
		parents: parents.Clone(),
	}
}

// AggregatedBranchFromBytes unmarshals an AggregatedBranch from a sequence of bytes.
func AggregatedBranchFromBytes(bytes []byte) (aggregatedBranch *AggregatedBranch, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if aggregatedBranch, err = AggregatedBranchFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse AggregatedBranch from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// AggregatedBranchFromMarshalUtil unmarshals an AggregatedBranch using a MarshalUtil (for easier unmarshaling).
func AggregatedBranchFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (aggregatedBranch *AggregatedBranch, err error) {
	branchType, err := marshalUtil.ReadByte()
	if err != nil {
		err = xerrors.Errorf("failed to parse BranchType (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if BranchType(branchType) != AggregatedBranchType {
		err = xerrors.Errorf("invalid BranchType (%X): %w", branchType, cerrors.ErrParseBytesFailed)
		return
	}

	aggregatedBranch = &AggregatedBranch{}
	if aggregatedBranch.id, err = BranchIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse id: %w", err)
		return
	}
	if aggregatedBranch.parents, err = BranchIDsFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse parents: %w", err)
		return
	}
	if aggregatedBranch.liked, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse liked flag (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if aggregatedBranch.monotonicallyLiked, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse monotonicallyLiked flag (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if aggregatedBranch.finalized, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse finalized flag (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if aggregatedBranch.inclusionState, err = InclusionStateFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse InclusionState from MarshalUtil: %w", err)
		return
	}

	return
}

// ID returns the identifier of the Branch.
func (a *AggregatedBranch) ID() BranchID {
	return a.id
}

// Type returns the type of the Branch.
func (a *AggregatedBranch) Type() BranchType {
	return AggregatedBranchType
}

// Parents returns the BranchIDs of the Branches parents in the BranchDAG.
func (a *AggregatedBranch) Parents() BranchIDs {
	a.parentsMutex.RLock()
	defer a.parentsMutex.RUnlock()

	return a.parents
}

// Liked returns true if the branch is "liked within it's scope" (ignoring monotonicity).
func (a *AggregatedBranch) Liked() bool {
	a.likedMutex.RLock()
	defer a.likedMutex.RUnlock()

	return a.liked
}

// SetLiked sets the liked property to the given value. It returns true if the value has been updated.
func (a *AggregatedBranch) SetLiked(liked bool) (modified bool) {
	a.likedMutex.Lock()
	defer a.likedMutex.Unlock()

	if a.liked == liked {
		return
	}

	a.liked = liked
	a.SetModified()
	modified = true

	return
}

// MonotonicallyLiked returns true if the branch is monotonically liked (all parents are also liked).
func (a *AggregatedBranch) MonotonicallyLiked() bool {
	a.monotonicallyLikedMutex.RLock()
	defer a.monotonicallyLikedMutex.RUnlock()

	return a.monotonicallyLiked
}

// SetMonotonicallyLiked sets the monotonically liked property to the given value. It returns true if the value has been
// updated.
func (a *AggregatedBranch) SetMonotonicallyLiked(monotonicallyLiked bool) (modified bool) {
	a.monotonicallyLikedMutex.Lock()
	defer a.monotonicallyLikedMutex.Unlock()

	if a.monotonicallyLiked == monotonicallyLiked {
		return
	}

	a.monotonicallyLiked = monotonicallyLiked
	a.SetModified()
	modified = true

	return
}

// Finalized returns true if the decision whether it is liked has been finalized.
func (a *AggregatedBranch) Finalized() bool {
	a.finalizedMutex.RLock()
	defer a.finalizedMutex.RUnlock()

	return a.finalized
}

// SetFinalized is the setter for the finalized flag. It returns true if the value of the flag has been updated.
func (a *AggregatedBranch) SetFinalized(finalized bool) (modified bool) {
	a.finalizedMutex.Lock()
	defer a.finalizedMutex.Unlock()

	if a.finalized == finalized {
		return
	}

	a.finalized = finalized
	a.SetModified()
	modified = true

	return
}

// InclusionState returns the InclusionState of the Branch which encodes if the Branch has been included in the
// ledger state.
func (a *AggregatedBranch) InclusionState() (inclusionState InclusionState) {
	a.inclusionStateMutex.RLock()
	defer a.inclusionStateMutex.RUnlock()

	return a.inclusionState
}

// SetInclusionState sets the InclusionState of the Branch which encodes if the Branch has been included in the
// ledger state. It returns true if the value has been updated.
func (a *AggregatedBranch) SetInclusionState(inclusionState InclusionState) (modified bool) {
	a.inclusionStateMutex.Lock()
	defer a.inclusionStateMutex.Unlock()

	if a.inclusionState == inclusionState {
		return
	}

	a.inclusionState = inclusionState
	a.SetModified()
	modified = true

	return
}

// Bytes returns a marshaled version of the Branch.
func (a *AggregatedBranch) Bytes() []byte {
	return a.ObjectStorageValue()
}

// String returns a human readable version of the Branch.
func (a *AggregatedBranch) String() string {
	return stringify.Struct("AggregatedBranch",
		stringify.StructField("id", a.ID()),
		stringify.StructField("parents", a.Parents()),
		stringify.StructField("liked", a.Liked()),
		stringify.StructField("monotonicallyLiked", a.MonotonicallyLiked()),
		stringify.StructField("finalized", a.Finalized()),
		stringify.StructField("inclusionState", a.InclusionState()),
	)
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (a *AggregatedBranch) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (a *AggregatedBranch) ObjectStorageKey() []byte {
	return a.ID().Bytes()
}

// ObjectStorageValue marshals the AggregatedBranch into a sequence of bytes that are used as the value part in the
// object storage.
func (a *AggregatedBranch) ObjectStorageValue() []byte {
	return marshalutil.New().
		WriteByte(byte(a.Type())).
		WriteBytes(a.ID().Bytes()).
		WriteBytes(a.Parents().Bytes()).
		WriteBool(a.Liked()).
		WriteBool(a.MonotonicallyLiked()).
		WriteBool(a.Finalized()).
		Write(a.InclusionState()).
		Bytes()
}

// code contract (make sure the struct implements all required methods)
var _ Branch = &AggregatedBranch{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ChildBranch //////////////////////////////////////////////////////////////////////////////////////////////////

// ChildBranchKeyPartition defines the partition of the storage key of the ChildBranch model.
var ChildBranchKeyPartition = objectstorage.PartitionKey(BranchIDLength, BranchIDLength)

// ChildBranch represents the relationship between a Branch and its children. Since a Branch can have a potentially
// unbounded amount of child Branches, we store this as a separate k/v pair instead of a marshaled list of children
// inside the Branch.
type ChildBranch struct {
	parentBranchID  BranchID
	childBranchID   BranchID
	childBranchType BranchType

	objectstorage.StorableObjectFlags
}

// NewChildBranch is the constructor of the ChildBranch reference.
func NewChildBranch(parentBranchID BranchID, childBranchID BranchID, childBranchType BranchType) *ChildBranch {
	return &ChildBranch{
		parentBranchID:  parentBranchID,
		childBranchID:   childBranchID,
		childBranchType: childBranchType,
	}
}

// ChildBranchFromBytes unmarshals a ChildBranch from a sequence of bytes.
func ChildBranchFromBytes(bytes []byte) (childBranch *ChildBranch, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if childBranch, err = ChildBranchFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse ChildBranch from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ChildBranchFromMarshalUtil unmarshals an ChildBranch using a MarshalUtil (for easier unmarshaling).
func ChildBranchFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (childBranch *ChildBranch, err error) {
	childBranch = &ChildBranch{}
	if childBranch.parentBranchID, err = BranchIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse parent BranchID from MarshalUtil: %w", err)
		return
	}
	if childBranch.childBranchID, err = BranchIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse child BranchID from MarshalUtil: %w", err)
		return
	}
	if childBranch.childBranchType, err = BranchTypeFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse child BranchType from MarshalUtil: %w", err)
		return
	}

	return
}

// ChildBranchFromObjectStorage is a factory method that creates a new ChildBranch instance from a storage key of the
// object storage. It is used by the object storage, to create new instances of this entity.
func ChildBranchFromObjectStorage(key []byte, _ []byte) (result objectstorage.StorableObject, err error) {
	if result, _, err = ChildBranchFromBytes(key); err != nil {
		err = xerrors.Errorf("failed to parse ChildBranch from bytes: %w", err)
		return
	}

	return
}

// ParentBranchID returns the BranchID of the parent Branch in the BranchDAG.
func (c *ChildBranch) ParentBranchID() (parentBranchID BranchID) {
	return c.parentBranchID
}

// ChildBranchID returns the BranchID of the child Branch in the BranchDAG.
func (c *ChildBranch) ChildBranchID() (childBranchID BranchID) {
	return c.childBranchID
}

// ChildBranchType returns the BranchType of the child Branch in the BranchDAG.
func (c *ChildBranch) ChildBranchType() BranchType {
	return c.childBranchType
}

// Bytes returns a marshaled version of the ChildBranch.
func (c *ChildBranch) Bytes() (marshaledChildBranch []byte) {
	return byteutils.ConcatBytes(c.ObjectStorageKey(), c.ObjectStorageValue())
}

// String returns a human readable version of the ChildBranch.
func (c *ChildBranch) String() (humanReadableChildBranch string) {
	return stringify.Struct("ChildBranch",
		stringify.StructField("parentBranchID", c.ParentBranchID()),
		stringify.StructField("childBranchID", c.ChildBranchID()),
		stringify.StructField("childBranchType", c.ChildBranchType()),
	)
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (c *ChildBranch) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (c *ChildBranch) ObjectStorageKey() (objectStorageKey []byte) {
	return marshalutil.New(ConflictIDLength + BranchIDLength).
		WriteBytes(c.parentBranchID.Bytes()).
		WriteBytes(c.childBranchID.Bytes()).
		Bytes()
}

// ObjectStorageValue marshals the AggregatedBranch into a sequence of bytes that are used as the value part in the
// object storage.
func (c *ChildBranch) ObjectStorageValue() (objectStorageValue []byte) {
	return c.childBranchType.Bytes()
}

// code contract (make sure the struct implements all required methods)
var _ objectstorage.StorableObject = &ChildBranch{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedChildBranch ////////////////////////////////////////////////////////////////////////////////////////////

// CachedChildBranch is a wrapper for the generic CachedObject returned by the object storage that overrides the
// accessor methods with a type-casted one.
type CachedChildBranch struct {
	objectstorage.CachedObject
}

// Retain marks the CachedObject to still be in use by the program.
func (c *CachedChildBranch) Retain() *CachedChildBranch {
	return &CachedChildBranch{c.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (c *CachedChildBranch) Unwrap() *ChildBranch {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*ChildBranch)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (c *CachedChildBranch) Consume(consumer func(childBranch *ChildBranch), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*ChildBranch))
	}, forceRelease...)
}

// String returns a human readable version of the CachedChildBranch.
func (c *CachedChildBranch) String() string {
	return stringify.Struct("CachedChildBranch",
		stringify.StructField("CachedObject", c.Unwrap()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedChildBranches //////////////////////////////////////////////////////////////////////////////////////////

// CachedChildBranches represents a collection of CachedChildBranch objects.
type CachedChildBranches []*CachedChildBranch

// Unwrap is the type-casted equivalent of Get. It returns a slice of unwrapped objects with the object being nil if it
// does not exist.
func (c CachedChildBranches) Unwrap() (unwrappedChildBranches []*ChildBranch) {
	unwrappedChildBranches = make([]*ChildBranch, len(c))
	for i, cachedChildBranch := range c {
		untypedObject := cachedChildBranch.Get()
		if untypedObject == nil {
			continue
		}

		typedObject := untypedObject.(*ChildBranch)
		if typedObject == nil || typedObject.IsDeleted() {
			continue
		}

		unwrappedChildBranches[i] = typedObject
	}

	return
}

// Consume iterates over the CachedObjects, unwraps them and passes a type-casted version to the consumer (if the object
// is not empty - it exists). It automatically releases the object when the consumer finishes. It returns true, if at
// least one object was consumed.
func (c CachedChildBranches) Consume(consumer func(childBranch *ChildBranch), forceRelease ...bool) (consumed bool) {
	for _, cachedChildBranch := range c {
		consumed = cachedChildBranch.Consume(consumer, forceRelease...) || consumed
	}

	return
}

// Release is a utility function that allows us to release all CachedObjects in the collection.
func (c CachedChildBranches) Release(force ...bool) {
	for _, cachedChildBranch := range c {
		cachedChildBranch.Release(force...)
	}
}

// String returns a human readable version of the CachedChildBranches.
func (c CachedChildBranches) String() string {
	structBuilder := stringify.StructBuilder("CachedChildBranches")
	for i, cachedChildBranch := range c {
		structBuilder.AddField(stringify.StructField(strconv.Itoa(i), cachedChildBranch))
	}

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
