package conflictdag

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/generics/model"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
)

// region Conflict /////////////////////////////////////////////////////////////////////////////////////////////////////

// Conflict represents a container for transactions and outputs spawning off from a conflicting transaction.
type Conflict[ConflictID set.AdvancedSetElement[ConflictID], ConflictSetID set.AdvancedSetElement[ConflictSetID]] struct {
	model.Model[ConflictID, conflict[ConflictID, ConflictSetID]]
}

type conflict[ConflictID set.AdvancedSetElement[ConflictID], ConflictSetID set.AdvancedSetElement[ConflictSetID]] struct {
	// parents contains the parent BranchIDs that this Conflict depends on.
	Parents *set.AdvancedSet[ConflictID]

	// conflictIDs contains the identifiers of the conflicts that this Conflict is part of.
	ConflictIDs *set.AdvancedSet[ConflictSetID]

	// inclusionState contains the InclusionState of the Conflict.
	InclusionState InclusionState
}

func NewConflict[ConflictID set.AdvancedSetElement[ConflictID], ConflictSetID set.AdvancedSetElement[ConflictSetID]](id ConflictID, parents *set.AdvancedSet[ConflictID], conflicts *set.AdvancedSet[ConflictSetID]) (new *Conflict[ConflictID, ConflictSetID]) {
	new = &Conflict[ConflictID, ConflictSetID]{model.NewModel[ConflictID](conflict[ConflictID, ConflictSetID]{
		Parents:        parents,
		ConflictIDs:    conflicts,
		InclusionState: Pending,
	})}
	new.SetID(id)

	return new
}

// Parents returns the parent BranchIDs that this Conflict depends on.
func (b *Conflict[ConflictID, ConflictSetID]) Parents() (parents *set.AdvancedSet[ConflictID]) {
	b.RLock()
	defer b.RUnlock()

	return b.M.Parents.Clone()
}

// SetParents updates the parent BranchIDs that this Conflict depends on. It returns true if the Conflict was modified.
func (b *Conflict[ConflictID, ConflictSetID]) SetParents(parents *set.AdvancedSet[ConflictID]) {
	b.Lock()
	defer b.Unlock()

	b.M.Parents = parents
	b.SetModified()

	return
}

// ConflictIDs returns the identifiers of the conflicts that this Conflict is part of.
func (b *Conflict[ConflictID, ConflictSetID]) ConflictIDs() (conflictIDs *set.AdvancedSet[ConflictSetID]) {
	b.RLock()
	defer b.RUnlock()

	return b.M.ConflictIDs.Clone()
}

// InclusionState returns the InclusionState of the Conflict.
func (b *Conflict[ConflictID, ConflictSetID]) InclusionState() (inclusionState InclusionState) {
	b.RLock()
	defer b.RUnlock()

	return b.M.InclusionState
}

// addConflict registers the membership of the Conflict in the given Conflict.
func (b *Conflict[ConflictID, ConflictSetID]) addConflict(conflictID ConflictSetID) (added bool) {
	b.Lock()
	defer b.Unlock()

	if added = b.M.ConflictIDs.Add(conflictID); added {
		b.SetModified()
	}

	return added
}

// setInclusionState sets the InclusionState of the Conflict.
func (b *Conflict[ConflictID, ConflictSetID]) setInclusionState(inclusionState InclusionState) (modified bool) {
	b.Lock()
	defer b.Unlock()

	if modified = b.M.InclusionState != inclusionState; !modified {
		return
	}

	b.M.InclusionState = inclusionState
	b.SetModified()

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ChildBranch //////////////////////////////////////////////////////////////////////////////////////////////////

// ChildBranch represents the reference between a Conflict and its children.
type ChildBranch[ConflictID set.AdvancedSetElement[ConflictID]] struct {
	// parentBranchID contains the identifier of the parent Conflict.
	parentBranchID ConflictID

	// childBranchID contains the identifier of the child Conflict.
	childBranchID ConflictID

	// StorableObjectFlags embeds the properties and methods required to manage the object storage related flags.
	objectstorage.StorableObjectFlags
}

// NewChildBranch return a new ChildBranch reference from the named parent to the named child.
func NewChildBranch[ConflictID set.AdvancedSetElement[ConflictID]](parentBranchID, childBranchID ConflictID) (new *ChildBranch[ConflictID]) {
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

// ParentBranchID returns the identifier of the parent Conflict.
func (c *ChildBranch[ConflictID]) ParentBranchID() (parentBranchID ConflictID) {
	return c.parentBranchID
}

// ChildBranchID returns the identifier of the child Conflict.
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

// ConflictMember represents the reference between a Conflict and its contained Conflict.
type ConflictMember[ConflictID set.AdvancedSetElement[ConflictID], ConflictSetID set.AdvancedSetElement[ConflictSetID]] struct {
	// conflictID contains the identifier of the conflict.
	conflictID ConflictSetID

	// branchID contains the identifier of the Conflict.
	branchID ConflictID

	// StorableObjectFlags embeds the properties and methods required to manage the object storage related flags.
	objectstorage.StorableObjectFlags
}

// NewConflictMember return a new ConflictMember reference from the named conflict to the named Conflict.
func NewConflictMember[ConflictID set.AdvancedSetElement[ConflictID], ConflictSetID set.AdvancedSetElement[ConflictSetID]](conflictID ConflictSetID, branchID ConflictID) (new *ConflictMember[ConflictID, ConflictSetID]) {
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

// BranchID returns the identifier of the Conflict.
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
