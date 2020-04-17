package ledgerstate

import (
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/transaction"
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

func osTransactionFactory(key []byte) (objectstorage.StorableObject, error, int) {
	return transaction.FromStorageKey(key)
}

func osTransactionMetadataFactory(key []byte) (objectstorage.StorableObject, error, int) {
	return TransactionMetadataFromStorageKey(key)
}

func osAttachmentFactory(key []byte) (objectstorage.StorableObject, error, int) {
	return AttachmentFromStorageKey(key)
}

func osOutputFactory(key []byte) (objectstorage.StorableObject, error, int) {
	return OutputFromStorageKey(key)
}

func osConsumerFactory(key []byte) (objectstorage.StorableObject, error, int) {
	return ConsumerFromStorageKey(key)
}
