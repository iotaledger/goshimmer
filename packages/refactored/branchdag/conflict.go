package branchdag

import (
	"fmt"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
)

// region InclusionState ///////////////////////////////////////////////////////////////////////////////////////////////

// InclusionState represents the confirmation status of elements in the ledger.
type InclusionState uint8

const (
	// Pending represents elements that have neither been confirmed nor rejected.
	Pending InclusionState = iota

	// Confirmed represents elements that have been confirmed and will stay part of the ledger state forever.
	Confirmed

	// Rejected represents elements that have been rejected and will not be included in the ledger state.
	Rejected
)

// InclusionStateFromBytes unmarshals an InclusionState from a sequence of bytes.
func InclusionStateFromBytes(bytes []byte) (inclusionState InclusionState, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if inclusionState, err = InclusionStateFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse InclusionState from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// InclusionStateFromMarshalUtil unmarshals an InclusionState from using a MarshalUtil (for easier unmarshalling).
func InclusionStateFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (inclusionState InclusionState, err error) {
	untypedInclusionState, err := marshalUtil.ReadUint8()
	if err != nil {
		err = errors.Errorf("failed to parse InclusionState (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	if inclusionState = InclusionState(untypedInclusionState); inclusionState > Rejected {
		err = errors.Errorf("invalid %s: %w", inclusionState, cerrors.ErrParseBytesFailed)
	}

	return
}

// String returns a human-readable representation of the InclusionState.
func (i InclusionState) String() string {
	switch i {
	case Pending:
		return "InclusionState(Pending)"
	case Confirmed:
		return "InclusionState(Confirmed)"
	case Rejected:
		return "InclusionState(Rejected)"
	default:
		return fmt.Sprintf("InclusionState(%X)", uint8(i))
	}
}

// Bytes returns a marshaled representation of the InclusionState.
func (i InclusionState) Bytes() []byte {
	return marshalutil.New(marshalutil.Uint8Size).WriteUint8(uint8(i)).Bytes()
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

// FromObjectStorage creates a Conflict from sequences of key and bytes.
func (c *Conflict) FromObjectStorage(key, bytes []byte) (conflict objectstorage.StorableObject, err error) {
	conflict, err = c.FromBytes(byteutils.ConcatBytes(key, bytes))
	if err != nil {
		err = errors.Errorf("failed to parse Conflict from bytes: %w", err)
	}
	return
}

// FromBytes unmarshals a Conflict from a sequence of bytes.
func (c *Conflict) FromBytes(bytes []byte) (conflict *Conflict, err error) {
	marshalUtil := marshalutil.New(bytes)
	if conflict, err = c.FromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse Conflict from MarshalUtil: %w", err)
		return
	}

	return
}

// FromMarshalUtil unmarshals a Conflict using a MarshalUtil (for easier unmarshalling).
func (c *Conflict) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (conflict *Conflict, err error) {
	if conflict = c; conflict == nil {
		conflict = &Conflict{}
	}

	if err = conflict.id.FromMarshalUtil(marshalUtil); err != nil {
		return nil, errors.Errorf("failed to parse ConflictID from MarshalUtil: %w", err)
	}
	memberCount, err := marshalUtil.ReadUint64()
	if err != nil {
		return nil, errors.Errorf("failed to parse member count (%v): %w", err, cerrors.ErrParseBytesFailed)
	}
	conflict.memberCount = int(memberCount)

	return
}

// ID returns the identifier of this Conflict.
func (c *Conflict) ID() ConflictID {
	return c.id
}

// MemberCount returns the amount of Branches that are part of this Conflict.
func (c *Conflict) MemberCount() int {
	c.memberCountMutex.RLock()
	defer c.memberCountMutex.RUnlock()

	return c.memberCount
}

// IncreaseMemberCount increase the MemberCount of this Conflict.
func (c *Conflict) IncreaseMemberCount(optionalDelta ...int) (newMemberCount int) {
	delta := 1
	if len(optionalDelta) >= 1 {
		delta = optionalDelta[0]
	}

	c.memberCountMutex.Lock()
	defer c.memberCountMutex.Unlock()

	c.memberCount += delta
	c.SetModified()
	newMemberCount = c.memberCount

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

	c.memberCount -= delta
	c.SetModified()
	newMemberCount = c.memberCount

	return
}

// Bytes returns a marshaled version of the Conflict.
func (c *Conflict) Bytes() []byte {
	return byteutils.ConcatBytes(c.ObjectStorageKey(), c.ObjectStorageValue())
}

// String returns a human-readable version of the Conflict.
func (c *Conflict) String() string {
	return stringify.Struct("Conflict",
		stringify.StructField("id", c.ID()),
		stringify.StructField("memberCount", c.MemberCount()),
	)
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (c *Conflict) ObjectStorageKey() []byte {
	return c.id.Bytes()
}

// ObjectStorageValue marshals the Conflict into a sequence of bytes. The ID is not serialized here as it is only used as
// a key in the ObjectStorage.
func (c *Conflict) ObjectStorageValue() []byte {
	return marshalutil.New(marshalutil.Uint64Size).
		WriteUint64(uint64(c.MemberCount())).
		Bytes()
}

// code contract (make sure the type implements all required methods)
var _ objectstorage.StorableObject = &Conflict{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ConflictMember ///////////////////////////////////////////////////////////////////////////////////////////////

// ConflictMemberKeyPartition defines the partition of the storage key of the ConflictMember model.
var ConflictMemberKeyPartition = objectstorage.PartitionKey(ConflictIDLength, BranchIDLength)

// ConflictMember represents the relationship between a Conflict and its Branches. Since an Output can have a
// potentially unbounded amount of conflicting Consumers, we store the membership of the Branches in the corresponding
// Conflicts as a separate k/v pair instead of a marshaled list of members inside the Branch.
type ConflictMember struct {
	conflictID ConflictID
	branchID   *BranchID

	objectstorage.StorableObjectFlags
}

// NewConflictMember is the constructor of the ConflictMember reference.
func NewConflictMember(conflictID ConflictID, branchID *BranchID) *ConflictMember {
	return &ConflictMember{
		conflictID: conflictID,
		branchID:   branchID,
	}
}

// FromObjectStorage creates an ConflictMember from sequences of key and bytes.
func (c *ConflictMember) FromObjectStorage(key, bytes []byte) (conflictMember objectstorage.StorableObject, err error) {
	conflictMember, err = c.FromBytes(byteutils.ConcatBytes(key, bytes))
	if err != nil {
		err = errors.Errorf("failed to parse ConflictMember from bytes: %w", err)
	}
	return
}

// FromBytes unmarshals a ConflictMember from a sequence of bytes.
func (c *ConflictMember) FromBytes(bytes []byte) (conflictMember *ConflictMember, err error) {
	marshalUtil := marshalutil.New(bytes)
	if conflictMember, err = c.FromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse ConflictMember from MarshalUtil: %w", err)
		return
	}

	return
}

// FromMarshalUtil unmarshals an ConflictMember using a MarshalUtil (for easier unmarshalling).
func (c *ConflictMember) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (conflictMember *ConflictMember, err error) {
	if conflictMember = c; conflictMember == nil {
		conflictMember = &ConflictMember{}
	}

	if err = conflictMember.conflictID.FromMarshalUtil(marshalUtil); err != nil {
		return nil, errors.Errorf("failed to parse ConflictID from MarshalUtil: %w", err)
	}
	if err = conflictMember.branchID.FromMarshalUtil(marshalUtil); err != nil {
		return nil, errors.Errorf("failed to parse BranchID: %w", err)
	}

	return
}

// ConflictID returns the identifier of the Conflict that this ConflictMember belongs to.
func (c *ConflictMember) ConflictID() ConflictID {
	return c.conflictID
}

// BranchID returns the identifier of the Branch that this ConflictMember references.
func (c *ConflictMember) BranchID() *BranchID {
	return c.branchID
}

// Bytes returns a marshaled version of this ConflictMember.
func (c *ConflictMember) Bytes() []byte {
	return c.ObjectStorageKey()
}

// String returns a human-readable version of this ConflictMember.
func (c *ConflictMember) String() string {
	return stringify.Struct("ConflictMember",
		stringify.StructField("conflictID", c.conflictID),
		stringify.StructField("branchID", c.branchID),
	)
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
