package ledgerstate

import (
	"strings"
	"sync"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/types"
	"github.com/mr-tron/base58"
	"golang.org/x/xerrors"
)

// region ConflictID ///////////////////////////////////////////////////////////////////////////////////////////////////

// ConflictIDLength contains the amount of bytes that a marshaled version of the ConflictID contains.
const ConflictIDLength = 32

// ConflictID is the data type that represents the identifier of a Conflict.
type ConflictID [ConflictIDLength]byte

// NewConflictID creates a new ConflictID from a OutputID.
func NewConflictID(outputID OutputID) (conflictID ConflictID) {
	copy(conflictID[:], outputID[:])

	return
}

// ConflictIDFromBytes unmarshals a ConflictID from a sequence of bytes.
func ConflictIDFromBytes(bytes []byte) (conflictID ConflictID, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if conflictID, err = ConflictIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse ConflictID from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ConflictIDFromBase58 creates a ConflictID from a base58 encoded string.
func ConflictIDFromBase58(base58String string) (conflictID ConflictID, err error) {
	bytes, err := base58.Decode(base58String)
	if err != nil {
		err = xerrors.Errorf("error while decoding base58 encoded ConflictID (%v): %w", err, ErrBase58DecodeFailed)
		return
	}

	if conflictID, _, err = ConflictIDFromBytes(bytes); err != nil {
		err = xerrors.Errorf("failed to parse ConflictID from bytes: %w", err)
		return
	}

	return
}

// ConflictIDFromMarshalUtil unmarshals a ConflictID using a MarshalUtil (for easier unmarshaling).
func ConflictIDFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (conflictID ConflictID, err error) {
	conflictIDBytes, err := marshalUtil.ReadBytes(ConflictIDLength)
	if err != nil {
		err = xerrors.Errorf("failed to parse ConflictID (%v): %w", err, ErrParseBytesFailed)
		return
	}
	copy(conflictID[:], conflictIDBytes)

	return
}

// Bytes returns a marshaled version of this ConflictID.
func (c ConflictID) Bytes() []byte {
	return c[:]
}

// Base58 returns a base58 encoded version of the ConflictID.
func (c ConflictID) Base58() string {
	return base58.Encode(c.Bytes())
}

// String returns a human readable version of the ConflictID.
func (c ConflictID) String() string {
	return "ConflictID(" + c.Base58() + ")"
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ConflictIDs //////////////////////////////////////////////////////////////////////////////////////////////////

// ConflictIDs represents a collection of ConflictIDs.
type ConflictIDs map[ConflictID]types.Empty

// NewConflictIDs creates a new collection of ConflictIDs from the given ConflictIDs.
func NewConflictIDs(branches ...ConflictID) (conflictIDs ConflictIDs) {
	conflictIDs = make(ConflictIDs)
	for _, conflictID := range branches {
		conflictIDs[conflictID] = types.Void
	}

	return
}

// ConflictIDsFromBytes unmarshals a collection of ConflictIDs from a sequence of bytes.
func ConflictIDsFromBytes(bytes []byte) (conflictIDs ConflictIDs, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if conflictIDs, err = ConflictIDsFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse ConflictIDs from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ConflictIDsFromMarshalUtil unmarshals a collection of ConflictIDs using a MarshalUtil (for easier unmarshaling).
func ConflictIDsFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (conflictIDs ConflictIDs, err error) {
	conflictIDsCount, err := marshalUtil.ReadUint64()
	if err != nil {
		err = xerrors.Errorf("failed to parse ConflictIDs count (%v): %w", err, ErrParseBytesFailed)
		return
	}

	conflictIDs = make(ConflictIDs)
	for i := uint64(0); i < conflictIDsCount; i++ {
		conflictID, conflictIDErr := ConflictIDFromMarshalUtil(marshalUtil)
		if conflictIDErr != nil {
			err = xerrors.Errorf("failed to parse ConflictID: %w", conflictIDErr)
			return
		}

		conflictIDs[conflictID] = types.Void
	}

	return
}

// Slice creates a slice of ConflictIDs from the collection.
func (c ConflictIDs) Slice() (list []ConflictID) {
	list = make([]ConflictID, len(c))
	i := 0
	for conflictID := range c {
		list[i] = conflictID
		i++
	}

	return
}

// Bytes returns a marshaled version of the ConflictIDs.
func (c ConflictIDs) Bytes() []byte {
	marshalUtil := marshalutil.New(marshalutil.INT64_SIZE + len(c)*ConflictIDLength)
	marshalUtil.WriteUint64(uint64(len(c)))
	for conflictID := range c {
		marshalUtil.WriteBytes(conflictID.Bytes())
	}

	return marshalUtil.Bytes()
}

// String returns a human readable version of the ConflictIDs.
func (c ConflictIDs) String() string {
	if len(c) == 0 {
		return "ConflictIDs{}"
	}

	result := "ConflictIDs{\n"
	for conflictID := range c {
		result += strings.Repeat(" ", stringify.INDENTATION_SIZE) + conflictID.String() + ",\n"
	}
	result += "}"

	return result
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Conflict /////////////////////////////////////////////////////////////////////////////////////////////////////

// Conflict represents a set of Branches that are conflicting with each other.
type Conflict struct {
	id               ConflictID
	memberCount      int
	memberCountMutex sync.RWMutex

	objectstorage.StorableObjectFlags
}

// NewConflict is the constructor for new Conflicts.
func NewConflict(conflictID ConflictID) *Conflict {
	return &Conflict{
		id: conflictID,
	}
}

// ConflictFromBytes unmarshals a Conflict from a sequence of bytes.
func ConflictFromBytes(bytes []byte) (conflict *Conflict, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if conflict, err = ConflictFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Conflict from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ConflictFromMarshalUtil unmarshals a Conflict using a MarshalUtil (for easier unmarshaling).
func ConflictFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (conflict *Conflict, err error) {
	conflict = &Conflict{}
	if conflict.id, err = ConflictIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse ConflictID from MarshalUtil: %w", err)
		return
	}
	memberCount, err := marshalUtil.ReadUint64()
	if err != nil {
		err = xerrors.Errorf("failed to parse member count (%v): %w", err, ErrParseBytesFailed)
		return
	}
	conflict.memberCount = int(memberCount)

	return
}

// ConflictFromObjectStorage restores a Conflict object that was stored in the ObjectStorage.
func ConflictFromObjectStorage(key []byte, data []byte) (outputMetadata *Conflict, err error) {
	if outputMetadata, _, err = ConflictFromBytes(byteutils.ConcatBytes(key, data)); err != nil {
		err = xerrors.Errorf("failed to parse Conflict from bytes: %w", err)
		return
	}

	return
}

// ID returns the identifier of this Conflict.
func (c *Conflict) ID() ConflictID {
	return c.id
}

// MemberCount returns the amount of Branches that are part of this Conflict.
func (c *Conflict) MemberCount() int {
	c.memberCountMutex.RLock()
	defer c.memberCountMutex.RLock()

	return c.memberCount
}

// IncreaseMemberCount increase the MemberCount of this Conflict.
func (c *Conflict) IncreaseMemberCount(optionalDelta ...int) int {
	delta := 1
	if len(optionalDelta) >= 1 {
		delta = optionalDelta[0]
	}

	c.memberCountMutex.Lock()
	defer c.memberCountMutex.Unlock()

	c.memberCount = c.memberCount + delta
	c.SetModified()

	return c.memberCount
}

// DecreaseMemberCount decreases the MemberCount of this Conflict.
func (c *Conflict) DecreaseMemberCount(optionalDelta ...int) (newMemberCount int) {
	delta := 1
	if len(optionalDelta) >= 1 {
		delta = optionalDelta[0]
	}

	c.memberCountMutex.Lock()
	defer c.memberCountMutex.Unlock()

	c.memberCount = c.memberCount - delta
	c.SetModified()
	newMemberCount = c.memberCount

	return
}

// Bytes returns a marshaled version of the Conflict.
func (c *Conflict) Bytes() []byte {
	return byteutils.ConcatBytes(c.ObjectStorageKey(), c.ObjectStorageValue())
}

// String returns a human readable version of the Conflict.
func (c *Conflict) String() string {
	return stringify.Struct("Conflict",
		stringify.StructField("id", c.ID()),
		stringify.StructField("memberCount", c.MemberCount()),
	)
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (c *Conflict) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (c *Conflict) ObjectStorageKey() []byte {
	return c.id.Bytes()
}

// ObjectStorageValue marshals the Output into a sequence of bytes. The ID is not serialized here as it is only used as
// a key in the ObjectStorage.
func (c *Conflict) ObjectStorageValue() []byte {
	return marshalutil.New(marshalutil.UINT64_SIZE).
		WriteUint64(uint64(c.MemberCount())).
		Bytes()
}

// code contract (make sure the type implements all required methods)
var _ objectstorage.StorableObject = &Conflict{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedConflict ///////////////////////////////////////////////////////////////////////////////////////////////

// CachedConflict is a wrapper for the generic CachedObject returned by the objectstorage that overrides the accessor
// methods with a type-casted one.
type CachedConflict struct {
	objectstorage.CachedObject
}

// Retain marks this CachedObject to still be in use by the program.
func (cachedConflict *CachedConflict) Retain() *CachedConflict {
	return &CachedConflict{cachedConflict.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (cachedConflict *CachedConflict) Unwrap() *Conflict {
	untypedObject := cachedConflict.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*Conflict)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer. It automatically releases the
// object when the consumer finishes and returns true of there was at least one object that was consumed.
func (cachedConflict *CachedConflict) Consume(consumer func(conflict *Conflict), forceRelease ...bool) (consumed bool) {
	return cachedConflict.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Conflict))
	}, forceRelease...)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ConflictMember ///////////////////////////////////////////////////////////////////////////////////////////////

// ConflictMember represents the relationship between a Conflict and its Branches. Since a Conflict can have a
// potentially unbounded amount of conflicting Consumers, we store this as a separate k/v pair instead of a marshaled
// ist of members inside the Branch.
type ConflictMember struct {
	conflictID ConflictID
	branchID   BranchID

	objectstorage.StorableObjectFlags
}

// NewConflictMember is the constructor of the ConflictMember reference.
func NewConflictMember(conflictID ConflictID, branchID BranchID) *ConflictMember {
	return &ConflictMember{
		conflictID: conflictID,
		branchID:   branchID,
	}
}

// ConflictID returns the identifier of the Conflict that this ConflictMember belongs to.
func (c *ConflictMember) ConflictID() ConflictID {
	return c.conflictID
}

// BranchID returns the identifier of the Branch that this ConflictMember references.
func (c *ConflictMember) BranchID() BranchID {
	return c.branchID
}

// Bytes returns a marshaled version of this ConflictMember.
func (c *ConflictMember) Bytes() []byte {
	return c.ObjectStorageKey()
}

// String returns a human readable version of this ConflictMember.
func (c *ConflictMember) String() string {
	return stringify.Struct("ConflictMember",
		stringify.StructField("conflictID", c.conflictID),
		stringify.StructField("branchID", c.branchID),
	)
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (c *ConflictMember) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (c *ConflictMember) ObjectStorageKey() []byte {
	return byteutils.ConcatBytes(c.conflictID.Bytes(), c.branchID.Bytes())
}

// ObjectStorageValue marshals the Output into a sequence of bytes. The ID is not serialized here as it is only used as
// a key in the ObjectStorage.
func (c *ConflictMember) ObjectStorageValue() []byte {
	return nil
}

// code contract (make sure the type implements all required methods)
var _ objectstorage.StorableObject = &ConflictMember{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
