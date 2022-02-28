package ledgerstate

import (
	"strconv"
	"strings"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/crypto"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/types"
	"github.com/mr-tron/base58"
)

// region BranchID /////////////////////////////////////////////////////////////////////////////////////////////////////

var (
	// UndefinedBranchID is the zero value of a BranchID and represents a branch that has not been set.
	UndefinedBranchID = BranchID{}

	// MasterBranchID is the identifier of the MasterBranch (root of the Branch DAG).
	MasterBranchID = BranchID{1}
)

// BranchIDLength contains the amount of bytes that a marshaled version of the BranchID contains.
const BranchIDLength = 32

// BranchID is the data type that represents the identifier of a Branch.
type BranchID [BranchIDLength]byte

// NewBranchID creates a new BranchID from a TransactionID.
func NewBranchID(transactionID TransactionID) (branchID BranchID) {
	copy(branchID[:], transactionID[:])

	return
}

// BranchIDEventHandler is an event handler for an event with a BranchID.
func BranchIDEventHandler(handler interface{}, params ...interface{}) {
	handler.(func(BranchID))(params[0].(BranchID))
}

// BranchIDFromBytes unmarshals a BranchID from a sequence of bytes.
func BranchIDFromBytes(bytes []byte) (branchID BranchID, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if branchID, err = BranchIDFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse BranchID from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// BranchIDFromBase58 creates a BranchID from a base58 encoded string.
func BranchIDFromBase58(base58String string) (branchID BranchID, err error) {
	decodedBytes, err := base58.Decode(base58String)
	if err != nil {
		err = errors.Errorf("error while decoding base58 encoded BranchID (%v): %w", err, cerrors.ErrBase58DecodeFailed)
		return
	}

	if branchID, _, err = BranchIDFromBytes(decodedBytes); err != nil {
		err = errors.Errorf("failed to parse BranchID from bytes: %w", err)
		return
	}

	return
}

// BranchIDFromMarshalUtil unmarshals a BranchID using a MarshalUtil (for easier unmarshaling).
func BranchIDFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (branchID BranchID, err error) {
	branchIDBytes, err := marshalUtil.ReadBytes(BranchIDLength)
	if err != nil {
		err = errors.Errorf("failed to parse BranchID (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	copy(branchID[:], branchIDBytes)

	return
}

// BranchIDFromRandomness returns a random BranchID which can for example be used for unit tests.
func BranchIDFromRandomness() (branchID BranchID) {
	crypto.Randomness.Read(branchID[:])

	return
}

// TransactionID returns the TransactionID of its underlying conflicting Transaction.
func (b BranchID) TransactionID() (transactionID TransactionID) {
	copy(transactionID[:], b[:])

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

// String returns a human-readable version of the BranchID.
func (b BranchID) String() string {
	switch b {
	case UndefinedBranchID:
		return "BranchID(UndefinedBranchID)"
	case MasterBranchID:
		return "BranchID(MasterBranchID)"
	default:
		if branchIDAlias, exists := branchIDAliases[b]; exists {
			return "BranchID(" + branchIDAlias + ")"
		}

		return "BranchID(" + b.Base58() + ")"
	}
}

// branchIDAliases contains a list of aliases registered for a set of MessageIDs.
var branchIDAliases = make(map[BranchID]string)

// RegisterBranchIDAlias registers an alias that will modify the String() output of the BranchID to show a human
// readable string instead of the base58 encoded version of itself.
func RegisterBranchIDAlias(branchID BranchID, alias string) {
	branchIDAliases[branchID] = alias
}

// UnregisterBranchIDAliases removes all aliases registered through the RegisterBranchIDAlias function.
func UnregisterBranchIDAliases() {
	branchIDAliases = make(map[BranchID]string)
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
		err = errors.Errorf("failed to parse BranchIDs from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// BranchIDsFromMarshalUtil unmarshals a collection of BranchIDs using a MarshalUtil (for easier unmarshaling).
func BranchIDsFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (branchIDs BranchIDs, err error) {
	branchIDsCount, err := marshalUtil.ReadUint64()
	if err != nil {
		err = errors.Errorf("failed to parse BranchIDs count (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	branchIDs = make(BranchIDs)
	for i := uint64(0); i < branchIDsCount; i++ {
		branchID, branchIDErr := BranchIDFromMarshalUtil(marshalUtil)
		if branchIDErr != nil {
			err = errors.Errorf("failed to parse BranchID: %w", branchIDErr)
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

// AddAll adds all BranchIDs to the collection and returns the collection to enable chaining.
func (b BranchIDs) AddAll(branchIDs BranchIDs) BranchIDs {
	for branchID := range branchIDs {
		b.Add(branchID)
	}

	return b
}

// Subtract removes all other from the collection and returns the collection to enable chaining.
func (b BranchIDs) Subtract(other BranchIDs) BranchIDs {
	for branchID := range other {
		delete(b, branchID)
	}

	return b
}

// Intersect removes all BranchIDs from the collection that are not contained in the argument collection.
// It returns the collection to enable chaining.
func (b BranchIDs) Intersect(branchIDs BranchIDs) (res BranchIDs) {
	// Iterate over the smallest map to increase performance.
	target, source := branchIDs, b
	if len(source) < len(target) {
		target, source = source, target
	}

	res = NewBranchIDs()
	for branchID := range target {
		if source.Contains(branchID) {
			res.Add(branchID)
		}
	}

	return
}

// Contains checks if the given target BranchID is part of the BranchIDs.
func (b BranchIDs) Contains(targetBranchID BranchID) (contains bool) {
	_, contains = b[targetBranchID]
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

// Equals returns whether the BranchIDs and other BranchIDs are equal.
func (b BranchIDs) Equals(o BranchIDs) bool {
	if len(b) != len(o) {
		return false
	}

	for branchID := range b {
		if _, exists := o[branchID]; !exists {
			return false
		}
	}

	return true
}

// Bytes returns a marshaled version of the BranchIDs.
func (b BranchIDs) Bytes() []byte {
	marshalUtil := marshalutil.New(marshalutil.Uint64Size + len(b)*BranchIDLength)
	marshalUtil.WriteUint64(uint64(len(b)))
	for branchID := range b {
		marshalUtil.WriteBytes(branchID.Bytes())
	}

	return marshalUtil.Bytes()
}

// Base58 returns a slice of base58 BranchIDs.
func (b BranchIDs) Base58() (result []string) {
	result = make([]string, 0)
	for id := range b {
		result = append(result, id.Base58())
	}

	return
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

// BranchFromBytes unmarshals a Branch from a sequence of bytes.
func BranchFromBytes(bytes []byte) (conflictBranch *Branch, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if conflictBranch, err = BranchFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse Branch from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// BranchFromMarshalUtil unmarshals an Branch using a MarshalUtil (for easier unmarshaling).
func BranchFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (conflictBranch *Branch, err error) {
	conflictBranch = &Branch{}
	if conflictBranch.id, err = BranchIDFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse id: %w", err)
		return
	}
	if conflictBranch.parents, err = BranchIDsFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse Branch parents: %w", err)
		return
	}
	if conflictBranch.conflicts, err = ConflictIDsFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse conflicts: %w", err)
		return
	}
	if conflictBranch.inclusionState, err = InclusionStateFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse inclusionState: %w", err)
		return
	}

	return
}

// BranchFromObjectStorage is a factory method that creates a new Branch instance from a storage key of
// the object storage. It is used by the object storage, to create new instances of this entity.
func BranchFromObjectStorage(key, value []byte) (conflictBranch objectstorage.StorableObject, err error) {
	conflictBranch, _, err = BranchFromBytes(byteutils.ConcatBytes(key, value))
	return
}

// ID returns the identifier of the Branch.
func (c *Branch) ID() BranchID {
	return c.id
}

// InclusionState returns the InclusionState of the Branch.
func (c *Branch) InclusionState() (inclusionState InclusionState) {
	c.inclusionStateMutex.RLock()
	defer c.inclusionStateMutex.RUnlock()

	return c.inclusionState
}

// setInclusionState sets the InclusionState of the Branch (it is private because the InclusionState should be
// set through the corresponding method in the BranchDAG).
func (c *Branch) setInclusionState(inclusionState InclusionState) (modified bool) {
	c.inclusionStateMutex.Lock()
	defer c.inclusionStateMutex.Unlock()

	if modified = c.inclusionState != inclusionState; !modified {
		return
	}

	c.inclusionState = inclusionState
	c.SetModified()

	return
}

// Parents returns the BranchIDs of the Branches parents in the BranchDAG.
func (c *Branch) Parents() BranchIDs {
	c.parentsMutex.RLock()
	defer c.parentsMutex.RUnlock()

	return c.parents.Clone()
}

// SetParents updates the parents of the Branch.
func (c *Branch) SetParents(parentBranches BranchIDs) (modified bool) {
	c.parentsMutex.Lock()
	defer c.parentsMutex.Unlock()

	c.parents = parentBranches
	c.SetModified()
	modified = true

	return
}

// Conflicts returns the Conflicts that the Branch is part of.
func (c *Branch) Conflicts() (conflicts ConflictIDs) {
	c.conflictsMutex.RLock()
	defer c.conflictsMutex.RUnlock()

	conflicts = c.conflicts.Clone()

	return
}

// AddConflict registers the membership of the Branch in the given Conflict.
func (c *Branch) AddConflict(conflictID ConflictID) (added bool) {
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

// Bytes returns a marshaled version of the Branch.
func (c *Branch) Bytes() []byte {
	return c.ObjectStorageValue()
}

// String returns a human-readable version of the Branch.
func (c *Branch) String() string {
	return stringify.Struct("Branch",
		stringify.StructField("id", c.ID()),
		stringify.StructField("parents", c.Parents()),
		stringify.StructField("conflicts", c.Conflicts()),
	)
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (c *Branch) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (c *Branch) ObjectStorageKey() []byte {
	return c.ID().Bytes()
}

// ObjectStorageValue marshals the Branch into a sequence of bytes that are used as the value part in the
// object storage.
func (c *Branch) ObjectStorageValue() []byte {
	return marshalutil.New().
		Write(c.Parents()).
		Write(c.Conflicts()).
		Write(c.InclusionState()).
		Bytes()
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
func (c *CachedBranch) Unwrap() *Branch {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*Branch)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (c *CachedBranch) Consume(consumer func(branch *Branch), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Branch))
	}, forceRelease...)
}

// String returns a human-readable version of the CachedBranch.
func (c *CachedBranch) String() string {
	return stringify.Struct("CachedBranch",
		stringify.StructField("CachedObject", c.Unwrap()),
	)
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

// ChildBranchFromBytes unmarshals a ChildBranch from a sequence of bytes.
func ChildBranchFromBytes(bytes []byte) (childBranch *ChildBranch, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if childBranch, err = ChildBranchFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse ChildBranch from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ChildBranchFromMarshalUtil unmarshals an ChildBranch using a MarshalUtil (for easier unmarshaling).
func ChildBranchFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (childBranch *ChildBranch, err error) {
	childBranch = &ChildBranch{}
	if childBranch.parentBranchID, err = BranchIDFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse parent BranchID from MarshalUtil: %w", err)
		return
	}
	if childBranch.childBranchID, err = BranchIDFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse child BranchID from MarshalUtil: %w", err)
		return
	}

	return
}

// ChildBranchFromObjectStorage is a factory method that creates a new ChildBranch instance from a storage key of the
// object storage. It is used by the object storage, to create new instances of this entity.
func ChildBranchFromObjectStorage(key, value []byte) (result objectstorage.StorableObject, err error) {
	if result, _, err = ChildBranchFromBytes(byteutils.ConcatBytes(key, value)); err != nil {
		err = errors.Errorf("failed to parse ChildBranch from bytes: %w", err)
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

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (c *ChildBranch) Update(objectstorage.StorableObject) {
	panic("updates disabled")
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

// region ArithmeticBranchIDs //////////////////////////////////////////////////////////////////////////////////////////

// ArithmeticBranchIDs represents an arithmetic collection of BranchIDs that allows us to add and subtract them from
// each other.
type ArithmeticBranchIDs map[BranchID]int

// NewArithmeticBranchIDs returns a new ArithmeticBranchIDs object.
func NewArithmeticBranchIDs(optionalBranchIDs ...BranchIDs) (newArithmeticBranchIDs ArithmeticBranchIDs) {
	newArithmeticBranchIDs = make(ArithmeticBranchIDs)
	if len(optionalBranchIDs) >= 1 {
		newArithmeticBranchIDs.Add(optionalBranchIDs[0])
	}

	return newArithmeticBranchIDs
}

// Add adds all BranchIDs to the collection.
func (a ArithmeticBranchIDs) Add(branchIDs BranchIDs) {
	for branchID := range branchIDs {
		a[branchID]++
	}
}

// Subtract subtracts all BranchIDs from the collection.
func (a ArithmeticBranchIDs) Subtract(branchIDs BranchIDs) {
	for branchID := range branchIDs {
		a[branchID]--
	}
}

// BranchIDs returns the BranchIDs represented by this collection.
func (a ArithmeticBranchIDs) BranchIDs() (branchIDs BranchIDs) {
	branchIDs = NewBranchIDs()
	for branchID, value := range a {
		if value >= 1 {
			branchIDs.Add(branchID)
		}
	}

	return
}

// String returns a human-readable version of the ArithmeticBranchIDs.
func (a ArithmeticBranchIDs) String() string {
	if len(a) == 0 {
		return "ArithmeticBranchIDs() = " + a.BranchIDs().String()
	}

	result := "ArithmeticBranchIDs("
	i := 0
	for branchID, value := range a {
		switch {
		case value == 1:
			if i != 0 {
				result += " + "
			}

			result += branchID.String()
			i++
		case value > 1:
			if i != 0 {
				result += " + "
			}

			result += strconv.Itoa(value) + "*" + branchID.String()
			i++
		case value == 0:
		case value == -1:
			if i != 0 {
				result += " - "
			} else {
				result += "-"
			}

			result += branchID.String()
			i++
		case value < -1:
			if i != 0 {
				result += " - "
			}

			result += strconv.Itoa(-value) + "*" + branchID.String()
			i++
		}
	}
	result += ") = " + a.BranchIDs().String()

	return result
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
