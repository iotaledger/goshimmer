package marker

import (
	"time"

	"github.com/iotaledger/hive.go/objectstorage"
)

const (
	// PrefixSequence defines the storage prefix for the Sequence.
	PrefixSequence byte = iota

	// PrefixSequenceAliasMapping defines the storage prefix for the SequenceAliasMapping.
	PrefixSequenceAliasMapping
)

var objectStorageOptions = []objectstorage.Option{
	objectstorage.CacheTime(60 * time.Second),
}
