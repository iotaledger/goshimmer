package utxodag

import (
	"time"

	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
)

const (
	// the following values are a list of prefixes defined as an enum
	_ byte = iota

	// prefixes used for the objectstorage
	osTransaction
	osTransactionMetadata
	osAttachment
	osOutput
	osConsumer
)

var (
	osLeakDetectionOption = objectstorage.LeakDetectionEnabled(true, objectstorage.LeakDetectionOptions{
		MaxConsumersPerObject: 10,
		MaxConsumerHoldTime:   10 * time.Second,
	})
)

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
