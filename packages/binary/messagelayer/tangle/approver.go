package tangle

import (
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
)

// Approver is an approver of a given referenced message.
type Approver struct {
	objectstorage.StorableObjectFlags
	// the message which got referenced by the approver message.
	referencedMessageID message.ID
	// the message which approved/referenced the given referenced message.
	approverMessageID message.ID
}

// NewApprover creates a new approver relation to the given approved/referenced message.
func NewApprover(referencedMessageID message.ID, approverMessageID message.ID) *Approver {
	approver := &Approver{
		referencedMessageID: referencedMessageID,
		approverMessageID:   approverMessageID,
	}
	return approver
}

// ApproverFromBytes parses the given bytes into an approver.
func ApproverFromBytes(bytes []byte, optionalTargetObject ...*Approver) (result *Approver, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = ParseApprover(marshalUtil, optionalTargetObject...)
	consumedBytes = marshalUtil.ReadOffset()
	return
}

// ParseApprover parses a new approver from the given marshal util.
func ParseApprover(marshalUtil *marshalutil.MarshalUtil, optionalTargetObject ...*Approver) (result *Approver, err error) {
	parsedObject, parseErr := marshalUtil.Parse(func(data []byte) (interface{}, int, error) {
		return ApproverFromStorageKey(data, optionalTargetObject...)
	})
	if parseErr != nil {
		err = parseErr
		return
	}
	result = parsedObject.(*Approver)

	_, err = marshalUtil.Parse(func(data []byte) (parseResult interface{}, parsedBytes int, parseErr error) {
		parsedBytes, parseErr = result.UnmarshalObjectStorageValue(data)

		return
	})

	return
}

// ApproverFromStorageKey returns an approver for the given key.
func ApproverFromStorageKey(key []byte, optionalTargetObject ...*Approver) (result objectstorage.StorableObject, consumedBytes int, err error) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &Approver{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to ApproverFromStorageKey")
	}

	// parse the properties that are stored in the key
	marshalUtil := marshalutil.New(key)
	if result.(*Approver).referencedMessageID, err = message.ParseID(marshalUtil); err != nil {
		return
	}
	if result.(*Approver).approverMessageID, err = message.ParseID(marshalUtil); err != nil {
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ReferencedMessageID returns the ID of the message which is referenced by the approver.
func (approver *Approver) ReferencedMessageID() message.ID {
	return approver.referencedMessageID
}

// ApproverMessageID returns the ID of the message which referenced the given approved message.
func (approver *Approver) ApproverMessageID() message.ID {
	return approver.approverMessageID
}

// Bytes returns the bytes of the approver.
func (approver *Approver) Bytes() []byte {
	return approver.ObjectStorageKey()
}

// String returns the string representation of the approver.
func (approver *Approver) String() string {
	return stringify.Struct("Approver",
		stringify.StructField("referencedMessageID", approver.ReferencedMessageID()),
		stringify.StructField("approverMessageID", approver.ApproverMessageID()),
	)
}

// ObjectStorageKey marshals the keys of the stored approver into a byte array.
// This includes the referencedMessageID and the approverMessageID.
func (approver *Approver) ObjectStorageKey() []byte {
	return marshalutil.New().
		WriteBytes(approver.referencedMessageID.Bytes()).
		WriteBytes(approver.approverMessageID.Bytes()).
		Bytes()
}

// ObjectStorageValue returns the value of the stored approver object.
func (approver *Approver) ObjectStorageValue() (result []byte) {
	return
}

// UnmarshalObjectStorageValue unmarshals the stored bytes into an approver.
func (approver *Approver) UnmarshalObjectStorageValue(data []byte) (consumedBytes int, err error) {
	return
}

// Update updates the approver.
// This should should never happen and will panic if attempted.
func (approver *Approver) Update(other objectstorage.StorableObject) {
	panic("approvers should never be overwritten and only stored once to optimize IO")
}

// interface contract (allow the compiler to check if the implementation has all of the required methods).
var _ objectstorage.StorableObject = &Approver{}

// CachedApprover is a wrapper for a stored cached object representing an approver.
type CachedApprover struct {
	objectstorage.CachedObject
}

// Unwrap unwraps the cached approver into the underlying approver.
// If stored object cannot be cast into an approver or has been deleted, it returns nil.
func (cachedApprover *CachedApprover) Unwrap() *Approver {
	untypedObject := cachedApprover.Get()
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
func (cachedApprover *CachedApprover) Consume(consumer func(approver *Approver)) (consumed bool) {
	return cachedApprover.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Approver))
	})
}

// CachedApprovers defines a slice of *CachedApprover.
type CachedApprovers []*CachedApprover

// Consume calls *CachedApprover.Consume on element in the list.
func (cachedApprovers CachedApprovers) Consume(consumer func(approver *Approver)) (consumed bool) {
	for _, cachedApprover := range cachedApprovers {
		consumed = cachedApprover.Consume(func(approver *Approver) {
			consumer(approver)
		}) || consumed
	}

	return
}
