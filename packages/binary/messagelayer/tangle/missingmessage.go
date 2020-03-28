package tangle

import (
	"time"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
)

type MissingMessage struct {
	objectstorage.StorableObjectFlags

	transactionId message.Id
	missingSince  time.Time
}

func NewMissingMessage(transactionId message.Id) *MissingMessage {
	return &MissingMessage{
		transactionId: transactionId,
		missingSince:  time.Now(),
	}
}

func MissingMessageFromStorageKey(key []byte, optionalTargetObject ...*MissingMessage) (result objectstorage.StorableObject, err error, consumedBytes int) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &Approver{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to MissingMessageFromStorageKey")
	}

	// parse the properties that are stored in the key
	marshalUtil := marshalutil.New(key)
	result.(*MissingMessage).transactionId, err = message.ParseId(marshalUtil)
	if err != nil {
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func (missingMessage *MissingMessage) GetTransactionId() message.Id {
	return missingMessage.transactionId
}

func (missingMessage *MissingMessage) GetMissingSince() time.Time {
	return missingMessage.missingSince
}

func (missingMessage *MissingMessage) Update(other objectstorage.StorableObject) {
	panic("missing transactions should never be overwritten and only stored once to optimize IO")
}

func (missingMessage *MissingMessage) ObjectStorageKey() []byte {
	return missingMessage.transactionId[:]
}

func (missingMessage *MissingMessage) ObjectStorageValue() (result []byte) {
	result, err := missingMessage.missingSince.MarshalBinary()
	if err != nil {
		panic(err)
	}

	return
}

func (missingMessage *MissingMessage) UnmarshalObjectStorageValue(data []byte) (err error, consumedBytes int) {
	marshalUtil := marshalutil.New(data)
	missingMessage.missingSince, err = marshalUtil.ReadTime()
	if err != nil {
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}
