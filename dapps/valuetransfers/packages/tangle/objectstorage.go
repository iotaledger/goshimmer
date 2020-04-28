package tangle

import (
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
)

const (
	// the following values are a list of prefixes defined as an enum
	_ byte = iota

	// prefixes used for the objectstorage
	osPayload
	osPayloadMetadata
	osMissingPayload
	osApprover
)

func osPayloadFactory(key []byte) (objectstorage.StorableObject, error, int) {
	return payload.FromStorageKey(key)
}

func osPayloadMetadataFactory(key []byte) (objectstorage.StorableObject, error, int) {
	return PayloadMetadataFromStorageKey(key)
}

func osMissingPayloadFactory(key []byte) (objectstorage.StorableObject, error, int) {
	return MissingPayloadFromStorageKey(key)
}

func osPayloadApproverFactory(key []byte) (objectstorage.StorableObject, error, int) {
	return PayloadApproverFromStorageKey(key)
}
