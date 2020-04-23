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
	referencedMessageId message.Id
	// the message which approved/referenced the given referenced message.
	approverMessageId message.Id
}

// NewApprover creates a new approver relation to the given approved/referenced message.
func NewApprover(referencedMessageId message.Id, approverMessageId message.Id) *Approver {
	approver := &Approver{
		referencedMessageId: referencedMessageId,
		approverMessageId:   approverMessageId,
	}
	return approver
}

// ApproverFromBytes parses the given bytes into an approver.
func ApproverFromBytes(bytes []byte, optionalTargetObject ...*Approver) (result *Approver, err error, consumedBytes int) {
	marshalUtil := marshalutil.New(bytes)
	result, err = ParseApprover(marshalUtil, optionalTargetObject...)
	consumedBytes = marshalUtil.ReadOffset()
	return
}

// ParseApprover parses a new approver from the given marshal util.
func ParseApprover(marshalUtil *marshalutil.MarshalUtil, optionalTargetObject ...*Approver) (result *Approver, err error) {
	if parsedObject, parseErr := marshalUtil.Parse(func(data []byte) (interface{}, error, int) {
		return ApproverFromStorageKey(data, optionalTargetObject...)
	}); parseErr != nil {
		err = parseErr
		return
	} else {
		result = parsedObject.(*Approver)
	}

	_, err = marshalUtil.Parse(func(data []byte) (parseResult interface{}, parseErr error, parsedBytes int) {
		parseErr, parsedBytes = result.UnmarshalObjectStorageValue(data)
		return
	})

	return
}

// ApproverFromStorageKey returns an approver for the given key.
func ApproverFromStorageKey(key []byte, optionalTargetObject ...*Approver) (result objectstorage.StorableObject, err error, consumedBytes int) {
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
	if result.(*Approver).referencedMessageId, err = message.ParseId(marshalUtil); err != nil {
		return
	}
	if result.(*Approver).approverMessageId, err = message.ParseId(marshalUtil); err != nil {
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ReferencedMessageId returns the id of the message which is referenced by the approver.
func (approver *Approver) ReferencedMessageId() message.Id {
	return approver.referencedMessageId
}

// ApproverMessageId returns the id of the message which referenced the given approved message.
func (approver *Approver) ApproverMessageId() message.Id {
	return approver.approverMessageId
}

func (approver *Approver) Bytes() []byte {
	return approver.ObjectStorageKey()
}

func (approver *Approver) String() string {
	return stringify.Struct("Approver",
		stringify.StructField("referencedMessageId", approver.ReferencedMessageId()),
		stringify.StructField("approverMessageId", approver.ApproverMessageId()),
	)
}

func (approver *Approver) ObjectStorageKey() []byte {
	return marshalutil.New().
		WriteBytes(approver.referencedMessageId.Bytes()).
		WriteBytes(approver.approverMessageId.Bytes()).
		Bytes()
}

func (approver *Approver) ObjectStorageValue() (result []byte) {
	return
}

func (approver *Approver) UnmarshalObjectStorageValue(data []byte) (err error, consumedBytes int) {
	return
}

func (approver *Approver) Update(other objectstorage.StorableObject) {
	panic("approvers should never be overwritten and only stored once to optimize IO")
}

// interface contract (allow the compiler to check if the implementation has all of the required methods).
var _ objectstorage.StorableObject = &Approver{}

type CachedApprover struct {
	objectstorage.CachedObject
}

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

func (cachedApprover *CachedApprover) Consume(consumer func(approver *Approver)) (consumed bool) {
	return cachedApprover.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Approver))
	})
}

type CachedApprovers []*CachedApprover

func (cachedApprovers CachedApprovers) Consume(consumer func(approver *Approver)) (consumed bool) {
	for _, cachedApprover := range cachedApprovers {
		consumed = cachedApprover.Consume(func(approver *Approver) {
			consumer(approver)
		}) || consumed
	}

	return
}
