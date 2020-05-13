package tangle

import (
	"github.com/iotaledger/goshimmer/dapps/fpctest/packages/payload"
	"github.com/iotaledger/hive.go/objectstorage"
)

const (
	// the following values are a list of prefixes defined as an enum
	_ byte = iota

	// prefixes used for the objectstorage
	osPayload
	osPayloadMetadata
)

func osPayloadFactory(key []byte) (objectstorage.StorableObject, int, error) {
	return payload.FromStorageKey(key)
}

func osPayloadMetadataFactory(key []byte) (objectstorage.StorableObject, int, error) {
	return PayloadMetadataFromStorageKey(key)
}
