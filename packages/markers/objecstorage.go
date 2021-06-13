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

// objectStorageOptions contains a list of default settings for the object storage.
var objectStorageOptions []objectstorage.Option

func initobjectStorageOptions() {
	objectStorageOptions = []objectstorage.Option{
		database.CacheTime(cacheTime),
	}
}
