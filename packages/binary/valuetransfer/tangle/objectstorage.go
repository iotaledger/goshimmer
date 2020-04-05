package tangle

import (
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/transaction"
)

const (
	// the following values are a list of prefixes defined as an enum
	_ byte = iota

	// prefixes used for the objectstorage
	osPayload
	osPayloadMetadata
	osMissingPayload
	osApprover
	osAttachment
	osOutput
	osMissingOutput
	osConsumer
)

func osMissingPayloadFactory(key []byte) (objectstorage.StorableObject, error, int) {
	return MissingPayloadFromStorageKey(key)
}

func osPayloadApproverFactory(key []byte) (objectstorage.StorableObject, error, int) {
	return PayloadApproverFromStorageKey(key)
}

func osAttachmentFactory(key []byte) (objectstorage.StorableObject, error, int) {
	return AttachmentFromStorageKey(key)
}

func osOutputFactory(key []byte) (objectstorage.StorableObject, error, int) {
	return transaction.OutputFromStorageKey(key)
}

func osMissingOutputFactory(key []byte) (objectstorage.StorableObject, error, int) {
	return MissingOutputFromStorageKey(key)
}

func osConsumerFactory(key []byte) (objectstorage.StorableObject, error, int) {
	return ConsumerFromStorageKey(key)
}
