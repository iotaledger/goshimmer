package tangle

import (
	"time"

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

func osTransactionFactory(key []byte, _ []byte) (objectstorage.StorableObject, int, error) {
	return transaction.FromStorageKey(key)
}

func osTransactionMetadataFactory(key []byte, _ []byte) (objectstorage.StorableObject, int, error) {
	return TransactionMetadataFromStorageKey(key)
}

func osAttachmentFactory(key []byte, _ []byte) (objectstorage.StorableObject, int, error) {
	return AttachmentFromStorageKey(key)
}

func osOutputFactory(key []byte, _ []byte) (objectstorage.StorableObject, int, error) {
	return OutputFromStorageKey(key)
}

func osConsumerFactory(key []byte, _ []byte) (objectstorage.StorableObject, int, error) {
	return ConsumerFromStorageKey(key)
}
