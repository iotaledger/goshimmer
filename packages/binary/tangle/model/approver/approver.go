package approver

import (
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message"
)

type Approver struct {
	objectstorage.StorableObjectFlags

	referencedTransaction message.Id
	approvingTransaction  message.Id
}

func New(referencedTransaction message.Id, approvingTransaction message.Id) *Approver {
	approver := &Approver{
		referencedTransaction: referencedTransaction,
		approvingTransaction:  approvingTransaction,
	}

	return approver
}

func Parse(marshalUtil *marshalutil.MarshalUtil) (result *Approver, err error) {
	if parsedObject, parseErr := marshalUtil.Parse(func(data []byte) (interface{}, error, int) {
		return StorableObjectFromKey(data)
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

func StorableObjectFromKey(key []byte) (result objectstorage.StorableObject, err error, consumedBytes int) {
	result = &Approver{}

	marshalUtil := marshalutil.New(key)
	result.(*Approver).referencedTransaction, err = message.ParseId(marshalUtil)
	if err != nil {
		return
	}
	result.(*Approver).approvingTransaction, err = message.ParseId(marshalUtil)
	if err != nil {
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func (approver *Approver) GetApprovingTransactionId() message.Id {
	return approver.approvingTransaction
}

func (approver *Approver) ObjectStorageKey() []byte {
	return marshalutil.New().
		WriteBytes(approver.referencedTransaction.Bytes()).
		WriteBytes(approver.approvingTransaction.Bytes()).
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
