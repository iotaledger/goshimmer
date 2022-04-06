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

// Branch represents a container for transactions and outputs spawning off from a conflicting transaction.
type Branch struct {
	// id contains the identifier of the Branch.
	id BranchID
	// parents contains the parent BranchIDs that this Branch depends on.
	parents BranchIDs
	// parentsMutex contains a mutex that is used to synchronize parallel access to the parents.
	parentsMutex sync.RWMutex
	// conflictIDs contains the identifiers of the conflicts that this Branch is part of.
	conflictIDs ConflictIDs
	// conflictIDsMutex contains a mutex that is used to synchronize parallel access to the conflictIDs.
	conflictIDsMutex sync.RWMutex
	// inclusionState contains the InclusionState of the Branch.
	inclusionState InclusionState
	// inclusionStateMutex contains a mutex that is used to synchronize parallel access to the inclusionState.
	inclusionStateMutex sync.RWMutex
	// StorableObjectFlags embeds the properties and methods required to manage to object storage related flags.
	objectstorage.StorableObjectFlags
}

// NewBranch return a new Branch from the given details.
func NewBranch(id BranchID, parents BranchIDs, conflicts ConflictIDs) (new *Branch) {
	new = &Branch{
		id:          id,
		parents:     parents.Clone(),
		conflictIDs: conflicts.Clone(),
	}
	new.SetModified()
	new.Persist()

	return new
}

// FromObjectStorage un-serializes a Branch from an object storage.
func (b *Branch) FromObjectStorage(key, bytes []byte) (conflictBranch objectstorage.StorableObject, err error) {
	result := new(Branch)
	if err = result.FromBytes(byteutils.ConcatBytes(key, bytes)); err != nil {
		return nil, errors.Errorf("failed to parse Branch from bytes: %w", err)
	}

	return result, nil
}

// FromBytes un-serializes a Branch from a sequence of bytes.
func (b *Branch) FromBytes(bytes []byte) (err error) {
	if err = b.FromMarshalUtil(marshalutil.New(bytes)); err != nil {
		return errors.Errorf("failed to parse Branch from MarshalUtil: %w", err)
	}

	return nil
}

// FromMarshalUtil un-serializes a Branch using a MarshalUtil.
func (b *Branch) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (err error) {
	if err = b.id.FromMarshalUtil(marshalUtil); err != nil {
		return errors.Errorf("failed to parse id: %w", err)
	}
	if err = b.parents.FromMarshalUtil(marshalUtil); err != nil {
		return errors.Errorf("failed to parse Branch parents: %w", err)
	}
	if err = b.conflictIDs.FromMarshalUtil(marshalUtil); err != nil {
		return errors.Errorf("failed to parse conflicts: %w", err)
	}
	if err = b.inclusionState.FromMarshalUtil(marshalUtil); err != nil {
		return errors.Errorf("failed to parse inclusionState: %w", err)
	}

	return nil
}

// ID returns the identifier of the Branch.
func (b *Branch) ID() BranchID {
	return b.id
}

// Parents returns the parent BranchIDs that this Branch depends on.
func (b *Branch) Parents() BranchIDs {
	b.parentsMutex.RLock()
	defer b.parentsMutex.RUnlock()

	return b.parents.Clone()
}

// SetParents updates the parent BranchIDs that this Branch depends on. It returns true if the Branch was modified.
func (b *Branch) SetParents(parents BranchIDs) (modified bool) {
	b.parentsMutex.Lock()
	defer b.parentsMutex.Unlock()

	b.parents = parents
	b.SetModified()
	modified = true

	return
}

// ConflictIDs returns the identifiers of the conflicts that this Branch is part of.
func (b *Branch) ConflictIDs() (conflicts ConflictIDs) {
	b.conflictIDsMutex.RLock()
	defer b.conflictIDsMutex.RUnlock()

	conflicts = b.conflictIDs.Clone()

	return
}

// InclusionState returns the InclusionState of the Branch.
func (b *Branch) InclusionState() (inclusionState InclusionState) {
	b.inclusionStateMutex.RLock()
	defer b.inclusionStateMutex.RUnlock()

	return b.inclusionState
}

// Bytes returns a serialized version of the Branch.
func (b *Branch) Bytes() []byte {
	return b.ObjectStorageValue()
}

// String returns a human-readable version of the Branch.
func (b *Branch) String() string {
	return stringify.Struct("Branch",
		stringify.StructField("id", b.ID()),
		stringify.StructField("parents", b.Parents()),
		stringify.StructField("conflictIDs", b.ConflictIDs()),
		stringify.StructField("inclusionState", b.InclusionState()),
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
		Write(b.ConflictIDs()).
		Write(b.InclusionState()).
		Bytes()
}

// addConflict registers the membership of the Branch in the given Conflict.
func (b *Branch) addConflict(conflictID ConflictID) (added bool) {
	b.conflictIDsMutex.Lock()
	defer b.conflictIDsMutex.Unlock()

	if added = b.conflictIDs.Add(conflictID); added {
		b.SetModified()
	}

	return added
}

// setInclusionState sets the InclusionState of the Branch.
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

// code contract (make sure the struct implements all required methods)
var _ objectstorage.StorableObject = new(Branch)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ChildBranch //////////////////////////////////////////////////////////////////////////////////////////////////

// ChildBranch represents the reference between a Branch and its children.
type ChildBranch struct {
	// parentBranchID contains the identifier of the parent Branch.
	parentBranchID BranchID
	// childBranchID contains the identifier of the child Branch.
	childBranchID BranchID
	// StorableObjectFlags embeds the properties and methods required to manage to object storage related flags.
	objectstorage.StorableObjectFlags
}

// NewChildBranch return a new ChildBranch reference from the named parent to the named child.
func NewChildBranch(parentBranchID, childBranchID BranchID) (new *ChildBranch) {
	new = &ChildBranch{
		parentBranchID: parentBranchID,
		childBranchID:  childBranchID,
	}
	new.Persist()
	new.SetModified()

	return new
}

// FromObjectStorage un-serializes a ChildBranch from an object storage.
func (c *ChildBranch) FromObjectStorage(key, bytes []byte) (childBranch objectstorage.StorableObject, err error) {
	result := new(Branch)
	if err = result.FromBytes(byteutils.ConcatBytes(key, bytes)); err != nil {
		return nil, errors.Errorf("failed to parse ChildBranch from bytes: %w", err)
	}

	return result, nil
}

// FromBytes un-serializes a ChildBranch from a sequence of bytes.
func (c *ChildBranch) FromBytes(bytes []byte) (err error) {
	if err = c.FromMarshalUtil(marshalutil.New(bytes)); err != nil {
		return errors.Errorf("failed to parse ChildBranch from MarshalUtil: %w", err)
	}

	return nil
}

// FromMarshalUtil un-serializes a ChildBranch using a MarshalUtil.
func (c *ChildBranch) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (err error) {
	if err = c.parentBranchID.FromMarshalUtil(marshalUtil); err != nil {
		return errors.Errorf("failed to parse parent BranchID from MarshalUtil: %w", err)
	}
	if err = c.childBranchID.FromMarshalUtil(marshalUtil); err != nil {
		return errors.Errorf("failed to parse child BranchID from MarshalUtil: %w", err)
	}

	return nil
}

// ParentBranchID returns the identifier of the parent Branch.
func (c *ChildBranch) ParentBranchID() (parentBranchID BranchID) {
	return c.parentBranchID
}

// ChildBranchID returns the identifier of the child Branch.
func (c *ChildBranch) ChildBranchID() (childBranchID BranchID) {
	return c.childBranchID
}

// Bytes returns a serialized version of the ChildBranch.
func (c *ChildBranch) Bytes() (marshaledChildBranch []byte) {
	return byteutils.ConcatBytes(c.ObjectStorageKey(), c.ObjectStorageValue())
}

// String returns a human-readable version of the ChildBranch.
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
	return nil
}

// code contract (make sure the struct implements all required methods)
var _ objectstorage.StorableObject = new(ChildBranch)

// childBranchKeyPartition defines the partition of the storage key of the ChildBranch model.
var childBranchKeyPartition = objectstorage.PartitionKey(BranchIDLength, BranchIDLength)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Conflict /////////////////////////////////////////////////////////////////////////////////////////////////////

// Conflict represents a set of branches that are conflicting with each other.
type Conflict struct {
	// id contains the identifier of the Conflict.
	id ConflictID
	// memberCount contains the amount of branches contained in this Conflict.
	memberCount int
	// memberCountMutex contains a mutex that is used to synchronize parallel access to the memberCount.
	memberCountMutex sync.RWMutex
	// StorableObjectFlags embeds the properties and methods required to manage to object storage related flags.
	objectstorage.StorableObjectFlags
}

// NewConflict returns a new Conflict with the given identifier.
func NewConflict(conflictID ConflictID) (new *Conflict) {
	new = &Conflict{
		id: conflictID,
	}
	new.Persist()
	new.SetModified()

	return new
}

// FromObjectStorage un-serializes a Conflict from an object storage.
func (c *Conflict) FromObjectStorage(key, bytes []byte) (conflict objectstorage.StorableObject, err error) {
	result := new(Conflict)
	if err = result.FromBytes(byteutils.ConcatBytes(key, bytes)); err != nil {
		return nil, errors.Errorf("failed to parse Conflict from bytes: %w", err)
	}

	return result, nil
}

// FromBytes un-serializes a Conflict from a sequence of bytes.
func (c *Conflict) FromBytes(bytes []byte) (err error) {
	if err = c.FromMarshalUtil(marshalutil.New(bytes)); err != nil {
		return errors.Errorf("failed to parse Conflict from MarshalUtil: %w", err)
	}

	return nil
}

// FromMarshalUtil un-serializes a Branch using a MarshalUtil.
func (c *Conflict) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (err error) {
	if err = c.id.FromMarshalUtil(marshalUtil); err != nil {
		return errors.Errorf("failed to parse ConflictID from MarshalUtil: %w", err)
	}
	memberCount, err := marshalUtil.ReadUint64()
	if err != nil {
		return errors.Errorf("failed to parse member count (%v): %w", err, cerrors.ErrParseBytesFailed)
	}
	c.memberCount = int(memberCount)

	return nil
}

// ID returns the identifier of the Conflict.
func (c *Conflict) ID() ConflictID {
	return c.id
}

// MemberCount returns the amount of Branches that are part of this Conflict.
func (c *Conflict) MemberCount() (memberCount int) {
	c.memberCountMutex.RLock()
	defer c.memberCountMutex.RUnlock()

	return c.memberCount
}

// IncreaseMemberCount increases the member count of this Conflict.
func (c *Conflict) IncreaseMemberCount(optionalDelta ...int) (newCount int) {
	delta := 1
	if len(optionalDelta) >= 1 {
		delta = optionalDelta[0]
	}

	c.memberCountMutex.Lock()
	defer c.memberCountMutex.Unlock()

	c.memberCount += delta
	c.SetModified()
	newCount = c.memberCount

	return c.memberCount
}

// DecreaseMemberCount decreases the member count of this Conflict.
func (c *Conflict) DecreaseMemberCount(optionalDelta ...int) (newCount int) {
	delta := 1
	if len(optionalDelta) >= 1 {
		delta = optionalDelta[0]
	}

	c.memberCountMutex.Lock()
	defer c.memberCountMutex.Unlock()

	c.memberCount -= delta
	c.SetModified()
	newCount = c.memberCount

	return
}

// Bytes returns a serialized version of the Conflict.
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

// ObjectStorageValue marshals the Conflict into a sequence of bytes. The id is not serialized here as it is only used
// as a key in the ObjectStorage.
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
