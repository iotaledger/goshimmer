package markers

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/database"

	"github.com/iotaledger/hive.go/objectstorage"
)

const (
	// PrefixSequence defines the storage prefix for the Sequence object storage.
	PrefixSequence byte = iota

	// PrefixSequenceAliasMapping defines the storage prefix for the SequenceAliasMapping object storage.
	PrefixSequenceAliasMapping

	// cacheTime defines the number of seconds objects are cached in the object storage.
	cacheTime = 60 * time.Second
)

type storageOptions struct {
	// objectStorageOptions contains a list of default settings for the object storage.
	objectStorageOptions []objectstorage.Option
}

func buildObjectStorageOptions(cacheTimeManager *database.CacheTimeProvider) *storageOptions {
	options := storageOptions{}

	options.objectStorageOptions = []objectstorage.Option{
		cacheTimeManager.CacheTime(cacheTime),
	}

	return &options
}
