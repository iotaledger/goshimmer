package tangle

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
)

type MessageMetadata struct {
	objectstorage.StorableObjectFlags

	messageId          message.Id
	receivedTime       time.Time
	solid              bool
	solidificationTime time.Time

	solidMutex              sync.RWMutex
	solidificationTimeMutex sync.RWMutex
}

func NewMessageMetadata(messageId message.Id) *MessageMetadata {
	return &MessageMetadata{
		messageId:    messageId,
		receivedTime: time.Now(),
	}
}

func MessageMetadataFromBytes(bytes []byte) (result *MessageMetadata, err error, consumedBytes int) {
	marshalUtil := marshalutil.New(bytes)
	result, err = ParseMessageMetadata(marshalUtil)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func ParseMessageMetadata(marshalUtil *marshalutil.MarshalUtil) (result *MessageMetadata, err error) {
	if parsedObject, parseErr := marshalUtil.Parse(func(data []byte) (interface{}, error, int) {
		return MessageMetadataFromStorageKey(data)
	}); parseErr != nil {
		err = parseErr

		return
	} else {
		result = parsedObject.(*MessageMetadata)
	}

	if _, err = marshalUtil.Parse(func(data []byte) (parseResult interface{}, parseErr error, parsedBytes int) {
		parseErr, parsedBytes = result.UnmarshalObjectStorageValue(data)

		return
	}); err != nil {
		return
	}

	return
}

func MessageMetadataFromStorageKey(key []byte) (result objectstorage.StorableObject, err error, consumedBytes int) {
	result = &MessageMetadata{}

	marshalUtil := marshalutil.New(key)
	result.(*MessageMetadata).messageId, err = message.ParseId(marshalUtil)
	if err != nil {
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func (messageMetadata *MessageMetadata) IsSolid() (result bool) {
	messageMetadata.solidMutex.RLock()
	result = messageMetadata.solid
	messageMetadata.solidMutex.RUnlock()

	return
}

func (messageMetadata *MessageMetadata) SetSolid(solid bool) (modified bool) {
	messageMetadata.solidMutex.RLock()
	if messageMetadata.solid != solid {
		messageMetadata.solidMutex.RUnlock()

		messageMetadata.solidMutex.Lock()
		if messageMetadata.solid != solid {
			messageMetadata.solid = solid
			if solid {
				messageMetadata.solidificationTimeMutex.Lock()
				messageMetadata.solidificationTime = time.Now()
				messageMetadata.solidificationTimeMutex.Unlock()
			}

			messageMetadata.SetModified()

			modified = true
		}
		messageMetadata.solidMutex.Unlock()

	} else {
		messageMetadata.solidMutex.RUnlock()
	}

	return
}

func (messageMetadata *MessageMetadata) SoldificationTime() time.Time {
	messageMetadata.solidificationTimeMutex.RLock()
	defer messageMetadata.solidificationTimeMutex.RUnlock()

	return messageMetadata.solidificationTime
}

func (messageMetadata *MessageMetadata) ObjectStorageKey() []byte {
	return messageMetadata.messageId.Bytes()
}

func (messageMetadata *MessageMetadata) ObjectStorageValue() []byte {
	return marshalutil.New().
		WriteTime(messageMetadata.receivedTime).
		WriteTime(messageMetadata.solidificationTime).
		WriteBool(messageMetadata.solid).
		Bytes()
}

func (messageMetadata *MessageMetadata) UnmarshalObjectStorageValue(data []byte) (err error, consumedBytes int) {
	marshalUtil := marshalutil.New(data)

	if messageMetadata.receivedTime, err = marshalUtil.ReadTime(); err != nil {
		return
	}
	if messageMetadata.solidificationTime, err = marshalUtil.ReadTime(); err != nil {
		return
	}
	if messageMetadata.solid, err = marshalUtil.ReadBool(); err != nil {
		return
	}

	consumedBytes = marshalUtil.ReadOffset()

	return
}

func (messageMetadata *MessageMetadata) Update(other objectstorage.StorableObject) {
	panic("updates disabled")
}

var _ objectstorage.StorableObject = &MessageMetadata{}

type CachedMessageMetadata struct {
	objectstorage.CachedObject
}

func (cachedMessageMetadata *CachedMessageMetadata) Retain() *CachedMessageMetadata {
	return &CachedMessageMetadata{cachedMessageMetadata.CachedObject.Retain()}
}

func (cachedMessageMetadata *CachedMessageMetadata) Unwrap() *MessageMetadata {
	if untypedObject := cachedMessageMetadata.Get(); untypedObject == nil {
		return nil
	} else {
		if typedObject := untypedObject.(*MessageMetadata); typedObject == nil || typedObject.IsDeleted() {
			return nil
		} else {
			return typedObject
		}
	}
}
