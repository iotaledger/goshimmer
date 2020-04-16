package ledgerstate

import (
	"github.com/iotaledger/hive.go/objectstorage"
)

const (
	// the following values are a list of prefixes defined as an enum
	_ byte = iota

	// prefixes used for the objectstorage
	osAttachment
	osOutput
	osMissingOutput
	osConsumer
)

func osAttachmentFactory(key []byte) (objectstorage.StorableObject, error, int) {
	return AttachmentFromStorageKey(key)
}

func osOutputFactory(key []byte) (objectstorage.StorableObject, error, int) {
	return OutputFromStorageKey(key)
}

func osMissingOutputFactory(key []byte) (objectstorage.StorableObject, error, int) {
	return MissingOutputFromStorageKey(key)
}

func osConsumerFactory(key []byte) (objectstorage.StorableObject, error, int) {
	return ConsumerFromStorageKey(key)
}
