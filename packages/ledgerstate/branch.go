package ledgerstate

import (
	"bytes"
	"sort"
	"strings"
	"sync"

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
		err = xerrors.Errorf("error while decoding base58 encoded BranchID (%v): %w", err, cerrors.ErrBase58DecodeFailed)
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
	if conflictBranch.preferred, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse preferred flag (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if conflictBranch.liked, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse liked flag (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if conflictBranch.finalized, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse finalized flag (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if conflictBranch.confirmed, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse confirmed flag (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if conflictBranch.rejected, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse rejected flag (%v): %w", err, cerrors.ErrParseBytesFailed)
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
func (c *ConflictBranch) SetParents(parentBranches ...BranchID) (modified bool, err error) {
	c.parentsMutex.Lock()
	defer c.parentsMutex.Unlock()

	c.parents = NewBranchIDs(parentBranches...)
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

// Preferred returns true if the branch is "liked within it's scope" (ignoring monotonicity).
func (c *ConflictBranch) Preferred() bool {
	c.preferredMutex.RLock()
	defer c.preferredMutex.RUnlock()

	return c.preferred
}

// SetPreferred sets the preferred property to the given value. It returns true if the value has been updated.
func (c *ConflictBranch) SetPreferred(preferred bool) (modified bool) {
	c.preferredMutex.Lock()
	defer c.preferredMutex.Unlock()

	if c.preferred == preferred {
		return
	}

	c.preferred = preferred
	c.SetModified()
	modified = true

	return
}

// Liked returns true if the branch is liked (taking monotonicity in account - i.e. all parents are also liked).
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

// Finalized returns true if the decision whether it is preferred has been finalized.
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

// Confirmed returns true if the decision that the Branch is liked has been finalized and all of its parents have
// been confirmed.
func (c *ConflictBranch) Confirmed() bool {
	c.confirmedMutex.RLock()
	defer c.confirmedMutex.RUnlock()

	return c.confirmed
}

// SetConfirmed is the setter for the confirmed flag. It returns true if the value of the flag has been updated.
func (c *ConflictBranch) SetConfirmed(confirmed bool) (modified bool) {
	c.confirmedMutex.Lock()
	defer c.confirmedMutex.Unlock()

	if c.confirmed == confirmed {
		return
	}

	c.confirmed = confirmed
	c.SetModified()
	modified = true

	return
}

// Rejected returns true if either a decision that the Branch is not liked has been finalized or any of its
// parents are rejected.
func (c *ConflictBranch) Rejected() bool {
	c.rejectedMutex.RLock()
	defer c.rejectedMutex.RUnlock()

	return c.confirmed
}

// SetRejected sets the rejected property to the given value. It returns true if the value has been updated.
func (c *ConflictBranch) SetRejected(rejected bool) (modified bool) {
	c.rejectedMutex.Lock()
	defer c.rejectedMutex.Unlock()

	if c.rejected == rejected {
		return
	}

	c.rejected = rejected
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
		stringify.StructField("preferred", c.Preferred()),
		stringify.StructField("liked", c.Liked()),
		stringify.StructField("finalized", c.Finalized()),
		stringify.StructField("confirmed", c.Confirmed()),
		stringify.StructField("rejected", c.Rejected()),
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
		WriteBool(c.Preferred()).
		WriteBool(c.Liked()).
		WriteBool(c.Finalized()).
		WriteBool(c.Confirmed()).
		WriteBool(c.Rejected()).
		Bytes()
}

// code contract (make sure the struct implements all required methods)
var _ Branch = &ConflictBranch{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region AggregatedBranch /////////////////////////////////////////////////////////////////////////////////////////////

// AggregatedBranch represents a container for Transactions and Outputs representing a certain perception of the ledger
// state.
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
	if aggregatedBranch.preferred, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse preferred flag (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if aggregatedBranch.liked, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse liked flag (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if aggregatedBranch.finalized, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse finalized flag (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if aggregatedBranch.confirmed, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse confirmed flag (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if aggregatedBranch.rejected, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse rejected flag (%v): %w", err, cerrors.ErrParseBytesFailed)
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

// Preferred returns true if the branch is "liked within it's scope" (ignoring monotonicity).
func (a *AggregatedBranch) Preferred() bool {
	a.preferredMutex.RLock()
	defer a.preferredMutex.RUnlock()

	return a.preferred
}

// SetPreferred sets the preferred property to the given value. It returns true if the value has been updated.
func (a *AggregatedBranch) SetPreferred(preferred bool) (modified bool) {
	a.preferredMutex.Lock()
	defer a.preferredMutex.Unlock()

	if a.preferred == preferred {
		return
	}

	a.preferred = preferred
	a.SetModified()
	modified = true

	return
}

// Liked returns true if the branch is liked (taking monotonicity in account - i.e. all parents are also liked).
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

// Finalized returns true if the decision whether it is preferred has been finalized.
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

// Confirmed returns true if the decision that the Branch is liked has been finalized and all of its parents have
// been confirmed.
func (a *AggregatedBranch) Confirmed() bool {
	a.confirmedMutex.RLock()
	defer a.confirmedMutex.RUnlock()

	return a.confirmed
}

// SetConfirmed is the setter for the confirmed flag. It returns true if the value of the flag has been updated.
func (a *AggregatedBranch) SetConfirmed(confirmed bool) (modified bool) {
	a.confirmedMutex.Lock()
	defer a.confirmedMutex.Unlock()

	if a.confirmed == confirmed {
		return
	}

	a.confirmed = confirmed
	a.SetModified()
	modified = true

	return
}

// Rejected returns true if either a decision that the Branch is not liked has been finalized or any of its
// parents are rejected.
func (a *AggregatedBranch) Rejected() bool {
	a.rejectedMutex.RLock()
	defer a.rejectedMutex.RUnlock()

	return a.confirmed
}

// SetRejected sets the rejected property to the given value. It returns true if the value has been updated.
func (a *AggregatedBranch) SetRejected(rejected bool) (modified bool) {
	a.rejectedMutex.Lock()
	defer a.rejectedMutex.Unlock()

	if a.rejected == rejected {
		return
	}

	a.rejected = rejected
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
		stringify.StructField("preferred", a.Preferred()),
		stringify.StructField("liked", a.Liked()),
		stringify.StructField("finalized", a.Finalized()),
		stringify.StructField("confirmed", a.Confirmed()),
		stringify.StructField("rejected", a.Rejected()),
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
		WriteBool(a.Preferred()).
		WriteBool(a.Liked()).
		WriteBool(a.Finalized()).
		WriteBool(a.Confirmed()).
		WriteBool(a.Rejected()).
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
	parentBranchID BranchID
	childBranchID  BranchID

	objectstorage.StorableObjectFlags
}

// NewChildBranch is the constructor of the ChildBranch reference.
func NewChildBranch(parentBranchID BranchID, childBranchID BranchID) *ChildBranch {
	return &ChildBranch{
		parentBranchID: parentBranchID,
		childBranchID:  childBranchID,
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
		err = xerrors.Errorf("failed to parse child BranchID: %w", err)
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

// Bytes returns a marshaled version of the ChildBranch.
func (c *ChildBranch) Bytes() (marshaledChildBranch []byte) {
	return c.ObjectStorageKey()
}

// String returns a human readable version of the ChildBranch.
func (c *ChildBranch) String() (humanReadableChildBranch string) {
	return stringify.Struct("ChildBranch",
		stringify.StructField("parentBranchID", c.ParentBranchID()),
		stringify.StructField("childBranchID", c.ChildBranchID()),
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
	return nil
}

// code contract (make sure the struct implements all required methods)
var _ objectstorage.StorableObject = &ChildBranch{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
