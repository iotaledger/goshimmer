package tangle

import (
	"fmt"

	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/xerrors"
)

// region ApproverType /////////////////////////////////////////////////////////////////////////////////////////////////

const (
	// StrongApprover is the ApproverType that represents references formed by strong parents.
	StrongApprover ApproverType = iota

	// WeakApprover is the ApproverType that represents references formed by weak parents.
	WeakApprover
)

// ApproverTypeLength contains the amount of bytes that a marshaled version of the ApproverType contains.
const ApproverTypeLength = 1

// ApproverType is a type that represents the different kind of reverse mapping that we have for references formed by
// strong and weak parents.
type ApproverType uint8

// ApproverTypeFromBytes unmarshals an ApproverType from a sequence of bytes.
func ApproverTypeFromBytes(bytes []byte) (approverType ApproverType, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if approverType, err = ApproverTypeFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse ApproverType from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ApproverTypeFromMarshalUtil unmarshals an ApproverType using a MarshalUtil (for easier unmarshaling).
func ApproverTypeFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (approverType ApproverType, err error) {
	untypedApproverType, err := marshalUtil.ReadUint8()
	if err != nil {
		err = xerrors.Errorf("failed to parse ApproverType (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if approverType = ApproverType(untypedApproverType); approverType != StrongApprover && approverType != WeakApprover {
		err = xerrors.Errorf("invalid ApproverType(%X): %w", approverType, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// Bytes returns a marshaled version of the ApproverType.
func (a ApproverType) Bytes() []byte {
	return []byte{byte(a)}
}

// String returns a human readable version of the ApproverType.
func (a ApproverType) String() string {
	switch a {
	case StrongApprover:
		return "ApproverType(StrongApprover)"
	case WeakApprover:
		return "ApproverType(WeakApprover)"
	default:
		return fmt.Sprintf("ApproverType(%X)", uint8(a))
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Approver /////////////////////////////////////////////////////////////////////////////////////////////////////

// Approver is an approver of a given referenced message.
type Approver struct {
	// approverType defines if the reference was create by a strong or a weak parent reference.
	approverType ApproverType

	// the message which got referenced by the approver message.
	referencedMessageID MessageID

	// the message which approved/referenced the given referenced message.
	approverMessageID MessageID

	objectstorage.StorableObjectFlags
}

// NewApprover creates a new approver relation to the given approved/referenced message.
func NewApprover(approverType ApproverType, referencedMessageID MessageID, approverMessageID MessageID) *Approver {
	approver := &Approver{
		approverType:        approverType,
		referencedMessageID: referencedMessageID,
		approverMessageID:   approverMessageID,
	}
	return approver
}

// ApproverFromBytes parses the given bytes into an approver.
func ApproverFromBytes(bytes []byte) (result *Approver, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = ApproverFromMarshalUtil(marshalUtil)
	consumedBytes = marshalUtil.ReadOffset()
	return
}

// ApproverFromMarshalUtil parses a new approver from the given marshal util.
func ApproverFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (result *Approver, err error) {
	result = &Approver{}
	if result.referencedMessageID, err = MessageIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse referenced MessageID from MarshalUtil: %w", err)
		return
	}
	if result.approverType, err = ApproverTypeFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse ApproverType from MarshalUtil: %w", err)
		return
	}
	if result.approverMessageID, err = MessageIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse approver MessageID from MarshalUtil: %w", err)
		return
	}

	return
}

// ApproverFromObjectStorage is the factory method for Approvers stored in the ObjectStorage.
func ApproverFromObjectStorage(key []byte, _ []byte) (result objectstorage.StorableObject, err error) {
	if result, _, err = ApproverFromBytes(key); err != nil {
		err = xerrors.Errorf("failed to parse Approver from bytes: %w", err)
		return
	}

	return
}

// Type returns the type of the Approver reference.
func (a *Approver) Type() ApproverType {
	return a.approverType
}

// ReferencedMessageID returns the ID of the message which is referenced by the approver.
func (a *Approver) ReferencedMessageID() MessageID {
	return a.referencedMessageID
}

// ApproverMessageID returns the ID of the message which referenced the given approved message.
func (a *Approver) ApproverMessageID() MessageID {
	return a.approverMessageID
}

// Bytes returns the bytes of the approver.
func (a *Approver) Bytes() []byte {
	return a.ObjectStorageKey()
}

// String returns the string representation of the approver.
func (a *Approver) String() string {
	return stringify.Struct("Approver",
		stringify.StructField("referencedMessageID", a.ReferencedMessageID()),
		stringify.StructField("approverMessageID", a.ApproverMessageID()),
	)
}

// ObjectStorageKey marshals the keys of the stored approver into a byte array.
// This includes the referencedMessageID and the approverMessageID.
func (a *Approver) ObjectStorageKey() []byte {
	return marshalutil.New().
		Write(a.referencedMessageID).
		Write(a.approverType).
		Write(a.approverMessageID).
		Bytes()
}

// ObjectStorageValue returns the value of the stored approver object.
func (a *Approver) ObjectStorageValue() (result []byte) {
	return
}

// Update updates the approver.
// This should should never happen and will panic if attempted.
func (a *Approver) Update(other objectstorage.StorableObject) {
	panic("approvers should never be overwritten and only stored once to optimize IO")
}

// interface contract (allow the compiler to check if the implementation has all of the required methods).
var _ objectstorage.StorableObject = &Approver{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedApprover ///////////////////////////////////////////////////////////////////////////////////////////////

// CachedApprover is a wrapper for a stored cached object representing an approver.
type CachedApprover struct {
	objectstorage.CachedObject
}

// Unwrap unwraps the cached approver into the underlying approver.
// If stored object cannot be cast into an approver or has been deleted, it returns nil.
func (c *CachedApprover) Unwrap() *Approver {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*Approver)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject

}

// Consume consumes the cachedApprover.
// It releases the object when the callback is done.
// It returns true if the callback was called.
func (c *CachedApprover) Consume(consumer func(approver *Approver)) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Approver))
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedApprovers //////////////////////////////////////////////////////////////////////////////////////////////

// CachedApprovers defines a slice of *CachedApprover.
type CachedApprovers []*CachedApprover

// Consume calls *CachedApprover.Consume on element in the list.
func (c CachedApprovers) Consume(consumer func(approver *Approver)) (consumed bool) {
	for _, cachedApprover := range c {
		consumed = cachedApprover.Consume(func(approver *Approver) {
			consumer(approver)
		}) || consumed
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
