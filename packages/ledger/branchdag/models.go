package branchdag

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
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

	// StorableObjectFlags embeds the properties and methods required to manage the object storage related flags.
	objectstorage.StorableObjectFlags
}

// NewBranch returns a new Branch from the given details.
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
func (b *Branch) FromObjectStorage(key, bytes []byte) (branch objectstorage.StorableObject, err error) {
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
	b.parents = NewBranchIDs()
	b.conflictIDs = NewConflictIDs()

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
func (b *Branch) ID() (id BranchID) {
	return b.id
}

// Parents returns the parent BranchIDs that this Branch depends on.
func (b *Branch) Parents() (parents BranchIDs) {
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
func (b *Branch) ConflictIDs() (conflictIDs ConflictIDs) {
	b.conflictIDsMutex.RLock()
	defer b.conflictIDsMutex.RUnlock()

	conflictIDs = b.conflictIDs.Clone()

	return
}

// InclusionState returns the InclusionState of the Branch.
func (b *Branch) InclusionState() (inclusionState InclusionState) {
	b.inclusionStateMutex.RLock()
	defer b.inclusionStateMutex.RUnlock()

	return b.inclusionState
}

// Bytes returns a serialized version of the Branch.
func (b *Branch) Bytes() (serialized []byte) {
	return b.ObjectStorageValue()
}

// String returns a human-readable version of the Branch.
func (b *Branch) String() (humanReadable string) {
	return stringify.Struct("Branch",
		stringify.StructField("id", b.ID()),
		stringify.StructField("parents", b.Parents()),
		stringify.StructField("conflictIDs", b.ConflictIDs()),
		stringify.StructField("inclusionState", b.InclusionState()),
	)
}

// ObjectStorageKey serializes the part of the object that is stored in the key part of the object storage.
func (b *Branch) ObjectStorageKey() (key []byte) {
	return b.ID().Bytes()
}

// ObjectStorageValue serializes the part of the object that is stored in the value part of the object storage.
func (b *Branch) ObjectStorageValue() (value []byte) {
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

	// StorableObjectFlags embeds the properties and methods required to manage the object storage related flags.
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
	result := new(ChildBranch)
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
func (c *ChildBranch) Bytes() (serialized []byte) {
	return byteutils.ConcatBytes(c.ObjectStorageKey(), c.ObjectStorageValue())
}

// String returns a human-readable version of the ChildBranch.
func (c *ChildBranch) String() (humanReadable string) {
	return stringify.Struct("ChildBranch",
		stringify.StructField("parentBranchID", c.ParentBranchID()),
		stringify.StructField("childBranchID", c.ChildBranchID()),
	)
}

// ObjectStorageKey serializes the part of the object that is stored in the key part of the object storage.
func (c *ChildBranch) ObjectStorageKey() (key []byte) {
	return marshalutil.New(BranchIDLength + BranchIDLength).
		WriteBytes(c.parentBranchID.Bytes()).
		WriteBytes(c.childBranchID.Bytes()).
		Bytes()
}

// ObjectStorageValue serializes the part of the object that is stored in the value part of the object storage.
func (c *ChildBranch) ObjectStorageValue() (value []byte) {
	return nil
}

// code contract (make sure the struct implements all required methods)
var _ objectstorage.StorableObject = new(ChildBranch)

// childBranchKeyPartition defines the partition of the storage key of the ChildBranch model.
var childBranchKeyPartition = objectstorage.PartitionKey(BranchIDLength, BranchIDLength)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ConflictMember ///////////////////////////////////////////////////////////////////////////////////////////////

// ConflictMember represents the reference between a Conflict and its contained Branch.
type ConflictMember struct {
	// conflictID contains the identifier of the conflict.
	conflictID ConflictID

	// branchID contains the identifier of the Branch.
	branchID BranchID

	// StorableObjectFlags embeds the properties and methods required to manage the object storage related flags.
	objectstorage.StorableObjectFlags
}

// NewConflictMember return a new ConflictMember reference from the named conflict to the named Branch.
func NewConflictMember(conflictID ConflictID, branchID BranchID) (new *ConflictMember) {
	new = &ConflictMember{
		conflictID: conflictID,
		branchID:   branchID,
	}
	new.Persist()
	new.SetModified()

	return new
}

// FromObjectStorage un-serializes a ConflictMember from an object storage.
func (c *ConflictMember) FromObjectStorage(key, bytes []byte) (conflictMember objectstorage.StorableObject, err error) {
	result := new(ConflictMember)
	if err = result.FromBytes(byteutils.ConcatBytes(key, bytes)); err != nil {
		return nil, errors.Errorf("failed to parse ConflictMember from bytes: %w", err)
	}

	return result, nil
}

// FromBytes un-serializes a ConflictMember from a sequence of bytes.
func (c *ConflictMember) FromBytes(bytes []byte) (err error) {
	if err = c.FromMarshalUtil(marshalutil.New(bytes)); err != nil {
		return errors.Errorf("failed to parse ConflictMember from MarshalUtil: %w", err)
	}

	return nil
}

// FromMarshalUtil un-serializes a ConflictMember using a MarshalUtil.
func (c *ConflictMember) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (err error) {
	if err = c.conflictID.FromMarshalUtil(marshalUtil); err != nil {
		return errors.Errorf("failed to parse ConflictID from MarshalUtil: %w", err)
	}
	if err = c.branchID.FromMarshalUtil(marshalUtil); err != nil {
		return errors.Errorf("failed to parse BranchID from MarshalUtil: %w", err)
	}

	return nil
}

// ConflictID returns the identifier of the Conflict.
func (c *ConflictMember) ConflictID() (conflictID ConflictID) {
	return c.conflictID
}

// BranchID returns the identifier of the Branch.
func (c *ConflictMember) BranchID() (branchID BranchID) {
	return c.branchID
}

// Bytes returns a serialized version of the ConflictMember.
func (c *ConflictMember) Bytes() (serialized []byte) {
	return c.ObjectStorageKey()
}

// String returns a human-readable version of the ConflictMember.
func (c *ConflictMember) String() (humanReadable string) {
	return stringify.Struct("ConflictMember",
		stringify.StructField("conflictID", c.conflictID),
		stringify.StructField("branchID", c.branchID),
	)
}

// ObjectStorageKey serializes the part of the object that is stored in the key part of the object storage.
func (c *ConflictMember) ObjectStorageKey() (key []byte) {
	return byteutils.ConcatBytes(c.conflictID.Bytes(), c.branchID.Bytes())
}

// ObjectStorageValue serializes the part of the object that is stored in the value part of the object storage.
func (c *ConflictMember) ObjectStorageValue() (value []byte) {
	return nil
}

// code contract (make sure the type implements all required methods)
var _ objectstorage.StorableObject = new(ConflictMember)

// conflictMemberKeyPartition defines the partition of the storage key of the ConflictMember model.
var conflictMemberKeyPartition = objectstorage.PartitionKey(ConflictIDLength, BranchIDLength)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
