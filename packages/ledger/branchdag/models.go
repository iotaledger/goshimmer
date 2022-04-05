package branchdag

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
)

// region Branch ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Branch represents a container for Transactions and Outputs representing a certain perception of the ledger state.
type Branch struct {
	id                  BranchID
	parents             BranchIDs
	parentsMutex        sync.RWMutex
	conflicts           ConflictIDs
	conflictsMutex      sync.RWMutex
	inclusionState      InclusionState
	inclusionStateMutex sync.RWMutex
	objectstorage.StorableObjectFlags
}

// NewBranch creates a new Branch from the given details.
func NewBranch(id BranchID, parents BranchIDs, conflicts ConflictIDs) *Branch {
	c := &Branch{
		id:        id,
		parents:   parents.Clone(),
		conflicts: conflicts.Clone(),
	}

	c.SetModified()
	c.Persist()

	return c
}

// FromObjectStorage creates an Branch from sequences of key and bytes.
func (b *Branch) FromObjectStorage(key, bytes []byte) (conflictBranch objectstorage.StorableObject, err error) {
	result, err := b.FromBytes(byteutils.ConcatBytes(key, bytes))
	if err != nil {
		err = errors.Errorf("failed to parse Branch from bytes: %w", err)
	}
	return result, err
}

// FromBytes unmarshals a Branch from a sequence of bytes.
func (b *Branch) FromBytes(bytes []byte) (branch *Branch, err error) {
	marshalUtil := marshalutil.New(bytes)
	if branch, err = b.FromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse Branch from MarshalUtil: %w", err)
		return
	}

	return
}

// FromMarshalUtil unmarshals an Branch using a MarshalUtil (for easier unmarshalling).
func (b *Branch) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (branch *Branch, err error) {
	if branch = b; b == nil {
		branch = &Branch{
			parents:   NewBranchIDs(),
			conflicts: NewConflictIDs(),
		}
	}

	if err = branch.id.FromMarshalUtil(marshalUtil); err != nil {
		return nil, errors.Errorf("failed to parse id: %w", err)
	}
	if err = branch.parents.FromMarshalUtil(marshalUtil); err != nil {
		return nil, errors.Errorf("failed to parse Branch parents: %w", err)
	}
	if err = branch.conflicts.FromMarshalUtil(marshalUtil); err != nil {
		return nil, errors.Errorf("failed to parse conflicts: %w", err)
	}
	if branch.inclusionState, err = InclusionStateFromMarshalUtil(marshalUtil); err != nil {
		return nil, errors.Errorf("failed to parse inclusionState: %w", err)
	}

	return
}

// ID returns the identifier of the Branch.
func (b *Branch) ID() BranchID {
	return b.id
}

// InclusionState returns the InclusionState of the Branch.
func (b *Branch) InclusionState() (inclusionState InclusionState) {
	b.inclusionStateMutex.RLock()
	defer b.inclusionStateMutex.RUnlock()

	return b.inclusionState
}

// setInclusionState sets the InclusionState of the Branch (it is private because the InclusionState should be
// set through the corresponding method in the BranchDAG).
func (b *Branch) setInclusionState(inclusionState InclusionState) (modified bool) {
	b.inclusionStateMutex.Lock()
	defer b.inclusionStateMutex.Unlock()

	if modified = b.inclusionState != inclusionState; !modified {
		return
	}

	b.inclusionState = inclusionState
	b.SetModified()

	return
}

// Parents returns the BranchIDs of the Branches parents in the BranchDAG.
func (b *Branch) Parents() BranchIDs {
	b.parentsMutex.RLock()
	defer b.parentsMutex.RUnlock()

	return b.parents.Clone()
}

// SetParents updates the parents of the Branch.
func (b *Branch) SetParents(parentBranches BranchIDs) (modified bool) {
	b.parentsMutex.Lock()
	defer b.parentsMutex.Unlock()

	b.parents = parentBranches
	b.SetModified()
	modified = true

	return
}

// Conflicts returns the Conflicts that the Branch is part of.
func (b *Branch) Conflicts() (conflicts ConflictIDs) {
	b.conflictsMutex.RLock()
	defer b.conflictsMutex.RUnlock()

	conflicts = b.conflicts.Clone()

	return
}

// AddConflict registers the membership of the Branch in the given Conflict.
func (b *Branch) AddConflict(conflictID ConflictID) (added bool) {
	b.conflictsMutex.Lock()
	defer b.conflictsMutex.Unlock()

	if added = b.conflicts.Add(conflictID); added {
		b.SetModified()
	}

	return added
}

// Bytes returns a marshaled version of the Branch.
func (b *Branch) Bytes() []byte {
	return b.ObjectStorageValue()
}

// String returns a human-readable version of the Branch.
func (b *Branch) String() string {
	return stringify.Struct("Branch",
		stringify.StructField("id", b.ID()),
		stringify.StructField("parents", b.Parents()),
		stringify.StructField("conflicts", b.Conflicts()),
	)
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (b *Branch) ObjectStorageKey() []byte {
	return b.ID().Bytes()
}

// ObjectStorageValue marshals the Branch into a sequence of bytes that are used as the value part in the
// object storage.
func (b *Branch) ObjectStorageValue() []byte {
	return marshalutil.New().
		Write(b.Parents()).
		Write(b.Conflicts()).
		Write(b.InclusionState()).
		Bytes()
}

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
func NewChildBranch(parentBranchID, childBranchID BranchID) *ChildBranch {
	return &ChildBranch{
		parentBranchID: parentBranchID,
		childBranchID:  childBranchID,
	}
}

// FromObjectStorage creates an ChildBranch from sequences of key and bytes.
func (c *ChildBranch) FromObjectStorage(key, bytes []byte) (childBranch objectstorage.StorableObject, err error) {
	result, err := c.FromBytes(byteutils.ConcatBytes(key, bytes))
	if err != nil {
		err = errors.Errorf("failed to parse ChildBranch from bytes: %w", err)
		return result, err
	}
	return result, err
}

// FromBytes unmarshals a ChildBranch from a sequence of bytes.
func (c *ChildBranch) FromBytes(bytes []byte) (childBranch objectstorage.StorableObject, err error) {
	marshalUtil := marshalutil.New(bytes)
	if childBranch, err = c.FromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse ChildBranch from MarshalUtil: %w", err)
		return
	}

	return
}

// FromMarshalUtil unmarshals an ChildBranch using a MarshalUtil (for easier unmarshaling).
func (c *ChildBranch) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (childBranch *ChildBranch, err error) {
	if childBranch = c; childBranch == nil {
		childBranch = new(ChildBranch)
	}

	if err = childBranch.parentBranchID.FromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse parent BranchID from MarshalUtil: %w", err)
		return
	}
	if err = childBranch.childBranchID.FromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse child BranchID from MarshalUtil: %w", err)
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
	return byteutils.ConcatBytes(c.ObjectStorageKey(), c.ObjectStorageValue())
}

// String returns a human readable version of the ChildBranch.
func (c *ChildBranch) String() (humanReadableChildBranch string) {
	return stringify.Struct("ChildBranch",
		stringify.StructField("parentBranchID", c.ParentBranchID()),
		stringify.StructField("childBranchID", c.ChildBranchID()),
	)
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (c *ChildBranch) ObjectStorageKey() (objectStorageKey []byte) {
	return marshalutil.New(BranchIDLength + BranchIDLength).
		WriteBytes(c.parentBranchID.Bytes()).
		WriteBytes(c.childBranchID.Bytes()).
		Bytes()
}

// ObjectStorageValue marshals the AggregatedBranch into a sequence of bytes that are used as the value part in the
// object storage.
func (c *ChildBranch) ObjectStorageValue() (objectStorageValue []byte) {
	return []byte{}
}

// code contract (make sure the struct implements all required methods)
var _ objectstorage.StorableObject = new(ChildBranch)

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
func NewConflict(conflictID ConflictID) (new *Conflict) {
	new = &Conflict{
		id: conflictID,
	}
	new.Persist()
	new.SetModified()

	return new
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
	branchID   BranchID

	objectstorage.StorableObjectFlags
}

// NewConflictMember is the constructor of the ConflictMember reference.
func NewConflictMember(conflictID ConflictID, branchID BranchID) (new *ConflictMember) {
	new = &ConflictMember{
		conflictID: conflictID,
		branchID:   branchID,
	}
	new.Persist()
	new.SetModified()

	return new
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
func (c *ConflictMember) BranchID() BranchID {
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
