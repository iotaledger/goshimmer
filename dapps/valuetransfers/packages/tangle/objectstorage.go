package tangle

import (
	"time"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/hive.go/objectstorage"
)

const (
	// the following values are a list of prefixes defined as an enum
	_ byte = iota

	// prefixes used for the objectstorage
	osPayload
	osPayloadMetadata
	osMissingPayload
	osApprover
	osTransaction
	osTransactionMetadata
	osAttachment
	osOutput
	osConsumer

	cacheTime = 20 * time.Second
)

var (
	osLeakDetectionOption = objectstorage.LeakDetectionEnabled(false, objectstorage.LeakDetectionOptions{
		MaxConsumersPerObject: 20,
		MaxConsumerHoldTime:   10 * time.Second,
	})
)

func osPayloadFactory(key []byte) (objectstorage.StorableObject, int, error) {
	return payload.FromStorageKey(key)
}

func osPayloadMetadataFactory(key []byte) (objectstorage.StorableObject, int, error) {
	return PayloadMetadataFromStorageKey(key)
}

func osMissingPayloadFactory(key []byte) (objectstorage.StorableObject, int, error) {
	return MissingPayloadFromStorageKey(key)
}

func osPayloadApproverFactory(key []byte) (objectstorage.StorableObject, int, error) {
	return PayloadApproverFromStorageKey(key)
}

func osTransactionFactory(key []byte) (objectstorage.StorableObject, int, error) {
	return transaction.FromStorageKey(key)
}

func osTransactionMetadataFactory(key []byte) (objectstorage.StorableObject, int, error) {
	return TransactionMetadataFromStorageKey(key)
}

func osAttachmentFactory(key []byte) (objectstorage.StorableObject, int, error) {
	return AttachmentFromStorageKey(key)
}

func osOutputFactory(key []byte) (objectstorage.StorableObject, int, error) {
	return OutputFromStorageKey(key)
}

func osConsumerFactory(key []byte) (objectstorage.StorableObject, int, error) {
	return ConsumerFromStorageKey(key)
}
