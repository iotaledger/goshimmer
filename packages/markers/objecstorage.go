package markers

import (
	"time"

	"github.com/iotaledger/hive.go/objectstorage"
)

const (
	// PrefixSequence defines the storage prefix for the Sequence object storage.
	PrefixSequence byte = iota

	// PrefixSequenceAliasMapping defines the storage prefix for the SequenceAliasMapping object storage.
	PrefixSequenceAliasMapping

	// CacheTime defines how long objects are cached in the object storage.
	CacheTime = 60 * time.Second
)

// objectStorageOptions contains a list of default settings for the object storage.
var objectStorageOptions = []objectstorage.Option{
	objectstorage.CacheTime(CacheTime),
}
