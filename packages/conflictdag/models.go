package conflictdag

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
)

// region Branch ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Branch represents a container for transactions and outputs spawning off from a conflicting transaction.
type Branch[ConflictID ConflictIDType[ConflictID], ConflictSet ConflictSetIDType[ConflictSet]] struct {
	// id contains the identifier of the Branch.
	id ConflictID

	// parents contains the parent BranchIDs that this Branch depends on.
	parents *set.AdvancedSet[ConflictID]

	// parentsMutex contains a mutex that is used to synchronize parallel access to the parents.
	parentsMutex sync.RWMutex

	// conflictIDs contains the identifiers of the conflicts that this Branch is part of.
	conflictIDs *set.AdvancedSet[ConflictSet]

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
func NewBranch[ConflictID ConflictIDType[ConflictID], ConflictSetID ConflictSetIDType[ConflictSetID]](id ConflictID, parents *set.AdvancedSet[ConflictID], conflicts *set.AdvancedSet[ConflictSetID]) (new *Branch[ConflictID, ConflictSetID]) {
	new = &Branch[ConflictID, ConflictSetID]{
		id:          id,
		parents:     parents.Clone(),
		conflictIDs: conflicts.Clone(),
	}
	new.SetModified()
	new.Persist()

	return new
}

// FromObjectStorage un-serializes a Branch from an object storage.
func (b *Branch[ConflictID, ConflictSetID]) FromObjectStorage(key, bytes []byte) (branch objectstorage.StorableObject, err error) {
	result := new(Branch[ConflictID, ConflictSetID])
	if err = result.FromBytes(byteutils.ConcatBytes(key, bytes)); err != nil {
		return nil, errors.Errorf("failed to parse Branch from bytes: %w", err)
	}

	return result, nil
}

// FromBytes un-serializes a Branch from a sequence of bytes.
func (b *Branch[ConflictID, ConflictSetID]) FromBytes(bytes []byte) (err error) {
	if err = b.FromMarshalUtil(marshalutil.New(bytes)); err != nil {
		return errors.Errorf("failed to parse Branch from MarshalUtil: %w", err)
	}

	return nil
}

// FromMarshalUtil un-serializes a Branch using a MarshalUtil.
func (b *Branch[ConflictID, ConflictSetID]) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (err error) {
	b.parents = set.NewAdvancedSet[ConflictID]()
	b.conflictIDs = set.NewAdvancedSet[ConflictSetID]()

	if b.id, err = b.id.Unmarshal(marshalUtil); err != nil {
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
func (b *Branch[ConflictID, ConflictSetID]) ID() (id ConflictID) {
	return b.id
}

// Parents returns the parent BranchIDs that this Branch depends on.
func (b *Branch[ConflictID, ConflictSetID]) Parents() (parents *set.AdvancedSet[ConflictID]) {
	b.parentsMutex.RLock()
	defer b.parentsMutex.RUnlock()

	return b.parents.Clone()
}

// SetParents updates the parent BranchIDs that this Branch depends on. It returns true if the Branch was modified.
func (b *Branch[ConflictID, ConflictSetID]) SetParents(parents *set.AdvancedSet[ConflictID]) (modified bool) {
	b.parentsMutex.Lock()
	defer b.parentsMutex.Unlock()

	b.parents = parents
	b.SetModified()
	modified = true

	return
}

// ConflictIDs returns the identifiers of the conflicts that this Branch is part of.
func (b *Branch[ConflictID, ConflictSetID]) ConflictIDs() (conflictIDs *set.AdvancedSet[ConflictSetID]) {
	b.conflictIDsMutex.RLock()
	defer b.conflictIDsMutex.RUnlock()

	conflictIDs = b.conflictIDs.Clone()

	return
}

// InclusionState returns the InclusionState of the Branch.
func (b *Branch[ConflictID, ConflictSetID]) InclusionState() (inclusionState InclusionState) {
	b.inclusionStateMutex.RLock()
	defer b.inclusionStateMutex.RUnlock()

	return b.inclusionState
}

// Bytes returns a serialized version of the Branch.
func (b *Branch[ConflictID, ConflictSetID]) Bytes() (serialized []byte) {
	return b.ObjectStorageValue()
}

// String returns a human-readable version of the Branch.
func (b *Branch[ConflictID, ConflictSetID]) String() (humanReadable string) {
	return stringify.Struct("Branch",
		stringify.StructField("id", b.ID()),
		stringify.StructField("parents", b.Parents()),
		stringify.StructField("conflictIDs", b.ConflictIDs()),
		stringify.StructField("inclusionState", b.InclusionState()),
	)
}

// ObjectStorageKey serializes the part of the object that is stored in the key part of the object storage.
func (b *Branch[ConflictID, ConflictSetID]) ObjectStorageKey() (key []byte) {
	return b.ID().Bytes()
}

// ObjectStorageValue serializes the part of the object that is stored in the value part of the object storage.
func (b *Branch[ConflictID, ConflictSetID]) ObjectStorageValue() (value []byte) {
	return marshalutil.New().
		Write(b.Parents()).
		Write(b.ConflictIDs()).
		Write(b.InclusionState()).
		Bytes()
}

// addConflict registers the membership of the Branch in the given Conflict.
func (b *Branch[ConflictID, ConflictSetID]) addConflict(conflictID ConflictSetID) (added bool) {
	b.conflictIDsMutex.Lock()
	defer b.conflictIDsMutex.Unlock()

	if added = b.conflictIDs.Add(conflictID); added {
		b.SetModified()
	}

	return added
}

// setInclusionState sets the InclusionState of the Branch.
func (b *Branch[ConflictID, ConflictSetID]) setInclusionState(inclusionState InclusionState) (modified bool) {
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
var _ objectstorage.StorableObject = new(Branch[MockedConflictID, MockedConflictSetID])

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ChildBranch //////////////////////////////////////////////////////////////////////////////////////////////////

// ChildBranch represents the reference between a Branch and its children.
type ChildBranch[ConflictID ConflictIDType[ConflictID]] struct {
	// parentBranchID contains the identifier of the parent Branch.
	parentBranchID ConflictID

	// childBranchID contains the identifier of the child Branch.
	childBranchID ConflictID

	// StorableObjectFlags embeds the properties and methods required to manage the object storage related flags.
	objectstorage.StorableObjectFlags
}

// NewChildBranch return a new ChildBranch reference from the named parent to the named child.
func NewChildBranch[ConflictID ConflictIDType[ConflictID]](parentBranchID, childBranchID ConflictID) (new *ChildBranch[ConflictID]) {
	new = &ChildBranch[ConflictID]{
		parentBranchID: parentBranchID,
		childBranchID:  childBranchID,
	}
	new.Persist()
	new.SetModified()

	return new
}

// FromObjectStorage un-serializes a ChildBranch from an object storage.
func (c *ChildBranch[ConflictID]) FromObjectStorage(key, bytes []byte) (childBranch objectstorage.StorableObject, err error) {
	result := new(ChildBranch[ConflictID])
	if err = result.FromBytes(byteutils.ConcatBytes(key, bytes)); err != nil {
		return nil, errors.Errorf("failed to parse ChildBranch from bytes: %w", err)
	}

	return result, nil
}

// FromBytes un-serializes a ChildBranch from a sequence of bytes.
func (c *ChildBranch[ConflictID]) FromBytes(bytes []byte) (err error) {
	if err = c.FromMarshalUtil(marshalutil.New(bytes)); err != nil {
		return errors.Errorf("failed to parse ChildBranch from MarshalUtil: %w", err)
	}

	return nil
}

// FromMarshalUtil un-serializes a ChildBranch using a MarshalUtil.
func (c *ChildBranch[ConflictID]) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (err error) {
	if c.parentBranchID, err = c.parentBranchID.Unmarshal(marshalUtil); err != nil {
		return errors.Errorf("failed to parse parent BranchID from MarshalUtil: %w", err)
	}
	if c.childBranchID, err = c.childBranchID.Unmarshal(marshalUtil); err != nil {
		return errors.Errorf("failed to parse child BranchID from MarshalUtil: %w", err)
	}

	return nil
}

// ParentBranchID returns the identifier of the parent Branch.
func (c *ChildBranch[ConflictID]) ParentBranchID() (parentBranchID ConflictID) {
	return c.parentBranchID
}

// ChildBranchID returns the identifier of the child Branch.
func (c *ChildBranch[ConflictID]) ChildBranchID() (childBranchID ConflictID) {
	return c.childBranchID
}

// Bytes returns a serialized version of the ChildBranch.
func (c *ChildBranch[ConflictID]) Bytes() (serialized []byte) {
	return byteutils.ConcatBytes(c.ObjectStorageKey(), c.ObjectStorageValue())
}

// String returns a human-readable version of the ChildBranch.
func (c *ChildBranch[ConflictID]) String() (humanReadable string) {
	return stringify.Struct("ChildBranch",
		stringify.StructField("parentBranchID", c.ParentBranchID()),
		stringify.StructField("childBranchID", c.ChildBranchID()),
	)
}

// ObjectStorageKey serializes the part of the object that is stored in the key part of the object storage.
func (c *ChildBranch[ConflictID]) ObjectStorageKey() (key []byte) {
	return marshalutil.New().
		WriteBytes(c.parentBranchID.Bytes()).
		WriteBytes(c.childBranchID.Bytes()).
		Bytes()
}

// ObjectStorageValue serializes the part of the object that is stored in the value part of the object storage.
func (c *ChildBranch[ConflictID]) ObjectStorageValue() (value []byte) {
	return nil
}

// code contract (make sure the struct implements all required methods)
var _ objectstorage.StorableObject = new(ChildBranch[MockedConflictSetID])

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ConflictMember ///////////////////////////////////////////////////////////////////////////////////////////////

// ConflictMember represents the reference between a Conflict and its contained Branch.
type ConflictMember[ConflictID ConflictIDType[ConflictID], ConflictSetID ConflictSetIDType[ConflictSetID]] struct {
	// conflictID contains the identifier of the conflict.
	conflictID ConflictSetID

	// branchID contains the identifier of the Branch.
	branchID ConflictID

	// StorableObjectFlags embeds the properties and methods required to manage the object storage related flags.
	objectstorage.StorableObjectFlags
}

// NewConflictMember return a new ConflictMember reference from the named conflict to the named Branch.
func NewConflictMember[ConflictID ConflictIDType[ConflictID], ConflictSetID ConflictSetIDType[ConflictSetID]](conflictID ConflictSetID, branchID ConflictID) (new *ConflictMember[ConflictID, ConflictSetID]) {
	new = &ConflictMember[ConflictID, ConflictSetID]{
		conflictID: conflictID,
		branchID:   branchID,
	}
	new.Persist()
	new.SetModified()

	return new
}

// FromObjectStorage un-serializes a ConflictMember from an object storage.
func (c *ConflictMember[ConflictID, ConflictSetID]) FromObjectStorage(key, bytes []byte) (conflictMember objectstorage.StorableObject, err error) {
	result := new(ConflictMember[ConflictID, ConflictSetID])
	if err = result.FromBytes(byteutils.ConcatBytes(key, bytes)); err != nil {
		return nil, errors.Errorf("failed to parse ConflictMember from bytes: %w", err)
	}

	return result, nil
}

// FromBytes un-serializes a ConflictMember from a sequence of bytes.
func (c *ConflictMember[ConflictID, ConflictSetID]) FromBytes(bytes []byte) (err error) {
	if err = c.FromMarshalUtil(marshalutil.New(bytes)); err != nil {
		return errors.Errorf("failed to parse ConflictMember from MarshalUtil: %w", err)
	}

	return nil
}

// FromMarshalUtil un-serializes a ConflictMember using a MarshalUtil.
func (c *ConflictMember[ConflictID, ConflictSetID]) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (err error) {
	if c.conflictID, err = c.conflictID.Unmarshal(marshalUtil); err != nil {
		return errors.Errorf("failed to parse ConflictID from MarshalUtil: %w", err)
	}
	if c.branchID, err = c.branchID.Unmarshal(marshalUtil); err != nil {
		return errors.Errorf("failed to parse BranchID from MarshalUtil: %w", err)
	}

	return nil
}

// ConflictID returns the identifier of the Conflict.
func (c *ConflictMember[ConflictID, ConflictSetID]) ConflictID() (conflictID ConflictSetID) {
	return c.conflictID
}

// BranchID returns the identifier of the Branch.
func (c *ConflictMember[ConflictID, ConflictSetID]) BranchID() (branchID ConflictID) {
	return c.branchID
}

// Bytes returns a serialized version of the ConflictMember.
func (c *ConflictMember[ConflictID, ConflictSetID]) Bytes() (serialized []byte) {
	return c.ObjectStorageKey()
}

// String returns a human-readable version of the ConflictMember.
func (c *ConflictMember[ConflictID, ConflictSetID]) String() (humanReadable string) {
	return stringify.Struct("ConflictMember",
		stringify.StructField("conflictID", c.conflictID),
		stringify.StructField("branchID", c.branchID),
	)
}

// ObjectStorageKey serializes the part of the object that is stored in the key part of the object storage.
func (c *ConflictMember[ConflictID, ConflictSetID]) ObjectStorageKey() (key []byte) {
	return byteutils.ConcatBytes(c.conflictID.Bytes(), c.branchID.Bytes())
}

// ObjectStorageValue serializes the part of the object that is stored in the value part of the object storage.
func (c *ConflictMember[ConflictID, ConflictSetID]) ObjectStorageValue() (value []byte) {
	return nil
}

// code contract (make sure the type implements all required methods)
var _ objectstorage.StorableObject = new(ConflictMember[MockedConflictID, MockedConflictSetID])

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
