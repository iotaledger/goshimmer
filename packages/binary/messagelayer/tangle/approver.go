package tangle

import (
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
)

// region Approver /////////////////////////////////////////////////////////////////////////////////////////////////////

type Approver struct {
	objectstorage.StorableObjectFlags

	referencedMessageId message.Id
	approvingMessageId  message.Id
}

func NewApprover(referencedTransaction message.Id, approvingTransaction message.Id) *Approver {
	approver := &Approver{
		referencedMessageId: referencedTransaction,
		approvingMessageId:  approvingTransaction,
	}

	return approver
}

func ApproverFromBytes(bytes []byte, optionalTargetObject ...*Approver) (result *Approver, err error, consumedBytes int) {
	marshalUtil := marshalutil.New(bytes)
	result, err = ParseApprover(marshalUtil, optionalTargetObject...)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func ParseApprover(marshalUtil *marshalutil.MarshalUtil, optionalTargetObject ...*Approver) (result *Approver, err error) {
	if parsedObject, parseErr := marshalUtil.Parse(func(data []byte) (interface{}, error, int) {
		return ApproverFromStorageKey(data, optionalTargetObject...)
	}); parseErr != nil {
		err = parseErr

		return
	} else {
		result = parsedObject.(*Approver)
	}

	if _, err = marshalUtil.Parse(func(data []byte) (parseResult interface{}, parseErr error, parsedBytes int) {
		parseErr, parsedBytes = result.UnmarshalObjectStorageValue(data)

		return
	}); err != nil {
		return
	}

	return
}

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
	result.(*Approver).referencedMessageId, err = message.ParseId(marshalUtil)
	if err != nil {
		return
	}
	result.(*Approver).approvingMessageId, err = message.ParseId(marshalUtil)
	if err != nil {
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func (approver *Approver) ReferencedMessageId() message.Id {
	return approver.approvingMessageId
}

func (approver *Approver) ApprovingMessageId() message.Id {
	return approver.approvingMessageId
}

func (approver *Approver) Bytes() []byte {
	return approver.ObjectStorageKey()
}

func (approver *Approver) String() string {
	return stringify.Struct("Approver",
		stringify.StructField("referencedMessageId", approver.ReferencedMessageId()),
		stringify.StructField("approvingMessageId", approver.ApprovingMessageId()),
	)
}

func (approver *Approver) ObjectStorageKey() []byte {
	return marshalutil.New().
		WriteBytes(approver.referencedMessageId.Bytes()).
		WriteBytes(approver.approvingMessageId.Bytes()).
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedApprover ///////////////////////////////////////////////////////////////////////////////////////////////

type CachedApprover struct {
	objectstorage.CachedObject
}

func (cachedApprover *CachedApprover) Unwrap() *Approver {
	if untypedObject := cachedApprover.Get(); untypedObject == nil {
		return nil
	} else {
		if typedObject := untypedObject.(*Approver); typedObject == nil || typedObject.IsDeleted() {
			return nil
		} else {
			return typedObject
		}
	}
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
