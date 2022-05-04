package ledgerstate

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/crypto"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/serix"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/types"
	"github.com/mr-tron/base58"
)

func init() {
	err := serix.DefaultAPI.RegisterTypeSettings(BranchIDs{}, serix.TypeSettings{}.WithLengthPrefixType(serix.LengthPrefixTypeAsUint32))
	if err != nil {
		panic(fmt.Errorf("error registering GenericDataPayload type settings: %w", err))
	}
}

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
func BranchIDFromBytes(data []byte) (branchID BranchID, consumedBytes int, err error) {
	_, err = serix.DefaultAPI.Decode(context.Background(), data, &branchID, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse BranchID: %w", err)
		return
	}
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

// Is checks if the given target BranchID is the only BranchID within BranchIDs.
func (b BranchIDs) Is(targetBranch BranchID) (is bool) {
	return len(b) == 1 && b.Contains(targetBranch)
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
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), b, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		return nil
	}
	return objBytes
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
	branchInner `serix:"0"`
}
type branchInner struct {
	id                  BranchID
	Parents             BranchIDs      `serix:"0"`
	Conflicts           ConflictIDs    `serix:"1"`
	InclusionState      InclusionState `serix:"2"`
	parentsMutex        sync.RWMutex
	conflictsMutex      sync.RWMutex
	inclusionStateMutex sync.RWMutex
	objectstorage.StorableObjectFlags
}

// NewBranch creates a new Branch from the given details.
func NewBranch(id BranchID, parents BranchIDs, conflicts ConflictIDs) *Branch {
	c := &Branch{
		branchInner{
			id:        id,
			Parents:   parents.Clone(),
			Conflicts: conflicts.Clone(),
		},
	}

	c.SetModified()
	c.Persist()

	return c
}

// FromObjectStorage creates an Branch from sequences of key and bytes.
func (b *Branch) FromObjectStorage(key, value []byte) (conflictBranch objectstorage.StorableObject, err error) {
	result, err := b.FromBytes(byteutils.ConcatBytes(key, value))
	if err != nil {
		err = errors.Errorf("failed to parse Branch from bytes: %w", err)
	}
	return result, err
}

// FromBytes unmarshals an Branch from a sequence of bytes.
func (b *Branch) FromBytes(data []byte) (branch *Branch, err error) {
	if branch = b; branch == nil {
		branch = new(Branch)
	}

	branchID := new(BranchID)
	bytesRead, err := serix.DefaultAPI.Decode(context.Background(), data, branchID, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse Branch.id: %w", err)
		return
	}

	_, err = serix.DefaultAPI.Decode(context.Background(), data[bytesRead:], branch, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse Branch: %w", err)
		return
	}
	branch.branchInner.id = *branchID
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

	return b.branchInner.InclusionState
}

// setInclusionState sets the InclusionState of the Branch (it is private because the InclusionState should be
// set through the corresponding method in the BranchDAG).
func (b *Branch) setInclusionState(inclusionState InclusionState) (modified bool) {
	b.inclusionStateMutex.Lock()
	defer b.inclusionStateMutex.Unlock()

	if modified = b.branchInner.InclusionState != inclusionState; !modified {
		return
	}

	b.branchInner.InclusionState = inclusionState
	b.SetModified()

	return
}

// Parents returns the BranchIDs of the Branches parents in the BranchDAG.
func (b *Branch) Parents() BranchIDs {
	b.parentsMutex.RLock()
	defer b.parentsMutex.RUnlock()

	return b.branchInner.Parents.Clone()
}

// SetParents updates the parents of the Branch.
func (b *Branch) SetParents(parentBranches BranchIDs) (modified bool) {
	b.parentsMutex.Lock()
	defer b.parentsMutex.Unlock()

	b.branchInner.Parents = parentBranches
	b.SetModified()
	modified = true

	return
}

// Conflicts returns the Conflicts that the Branch is part of.
func (b *Branch) Conflicts() (conflicts ConflictIDs) {
	b.conflictsMutex.RLock()
	defer b.conflictsMutex.RUnlock()

	conflicts = b.branchInner.Conflicts.Clone()

	return
}

// AddConflict registers the membership of the Branch in the given Conflict.
func (b *Branch) AddConflict(conflictID ConflictID) (added bool) {
	b.conflictsMutex.Lock()
	defer b.conflictsMutex.Unlock()

	if _, exists := b.branchInner.Conflicts[conflictID]; exists {
		return
	}

	b.branchInner.Conflicts[conflictID] = types.Void
	b.SetModified()
	added = true

	return
}

// Bytes returns a marshaled version of the Branch.
func (b *Branch) Bytes() []byte {
	return b.ObjectStorageValue()
}

// String returns a human-readable version of the Branch.
func (b *Branch) String() string {
	return stringify.Struct("Branch",
		stringify.StructField("id", b.ID()),
		stringify.StructField("Parents", b.Parents()),
		stringify.StructField("Conflicts", b.Conflicts()),
		stringify.StructField("InclusionState", b.InclusionState()),
	)
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (b *Branch) ObjectStorageKey() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), b.ID(), serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// ObjectStorageValue marshals the Branch into a sequence of bytes that are used as the value part in the
// object storage.
func (b *Branch) ObjectStorageValue() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), b, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ChildBranch //////////////////////////////////////////////////////////////////////////////////////////////////

// ChildBranchKeyPartition defines the partition of the storage key of the ChildBranch model.
var ChildBranchKeyPartition = objectstorage.PartitionKey(BranchIDLength, BranchIDLength)

// ChildBranch represents the relationship between a Branch and its children. Since a Branch can have a potentially
// unbounded amount of child Branches, we store this as a separate k/v pair instead of a marshaled list of children
// inside the Branch.
type ChildBranch struct {
	conflictBranchInner `serix:"0"`
}
type conflictBranchInner struct {
	ParentBranchID BranchID `serix:"0"`
	ChildBranchID  BranchID `serix:"1"`

	objectstorage.StorableObjectFlags
}

// NewChildBranch is the constructor of the ChildBranch reference.
func NewChildBranch(parentBranchID, childBranchID BranchID) *ChildBranch {
	return &ChildBranch{
		conflictBranchInner{
			ParentBranchID: parentBranchID,
			ChildBranchID:  childBranchID,
		},
	}
}

// FromObjectStorage creates an ChildBranch from sequences of key and bytes.
func (c *ChildBranch) FromObjectStorage(key, value []byte) (childBranch objectstorage.StorableObject, err error) {
	result, err := c.FromBytes(byteutils.ConcatBytes(key, value))
	if err != nil {
		err = errors.Errorf("failed to parse ChildBranch from bytes: %w", err)
		return result, err
	}
	return result, err
}

// FromBytes unmarshals a ChildBranch from a sequence of bytes.
func (c *ChildBranch) FromBytes(data []byte) (childBranch *ChildBranch, err error) {
	if childBranch = c; childBranch == nil {
		childBranch = new(ChildBranch)
	}
	_, err = serix.DefaultAPI.Decode(context.Background(), data, childBranch, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse ChildBranch: %w", err)
		return
	}
	return
}

// ParentBranchID returns the BranchID of the parent Branch in the BranchDAG.
func (c *ChildBranch) ParentBranchID() (parentBranchID BranchID) {
	return c.conflictBranchInner.ParentBranchID
}

// ChildBranchID returns the BranchID of the child Branch in the BranchDAG.
func (c *ChildBranch) ChildBranchID() (childBranchID BranchID) {
	return c.conflictBranchInner.ChildBranchID
}

// Bytes returns a marshaled version of the ChildBranch.
func (c *ChildBranch) Bytes() (marshaledChildBranch []byte) {
	return byteutils.ConcatBytes(c.ObjectStorageKey(), c.ObjectStorageValue())
}

// String returns a human readable version of the ChildBranch.
func (c *ChildBranch) String() (humanReadableChildBranch string) {
	return stringify.Struct("ChildBranch",
		stringify.StructField("ParentBranchID", c.ParentBranchID()),
		stringify.StructField("ChildBranchID", c.ChildBranchID()),
	)
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (c *ChildBranch) ObjectStorageKey() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), c, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// ObjectStorageValue marshals the Branch into a sequence of bytes that are used as the value part in the
// object storage.
func (c *ChildBranch) ObjectStorageValue() []byte {
	return []byte{}

}

// code contract (make sure the struct implements all required methods)
var _ objectstorage.StorableObject = new(ChildBranch)

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
