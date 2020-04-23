package branchmanager

import (
	"time"

	"github.com/iotaledger/hive.go/objectstorage"
)

const (
	// the following values are a list of prefixes defined as an enum
	_ byte = iota

	// prefixes used for the objectstorage
	osBranch
)

var (
	osLeakDetectionOption = objectstorage.LeakDetectionEnabled(false, objectstorage.LeakDetectionOptions{
		MaxConsumersPerObject: 10,
		MaxConsumerHoldTime:   10 * time.Second,
	})

	osBranchOptions = []objectstorage.Option{
		objectstorage.CacheTime(60 * time.Second),
		osLeakDetectionOption,
	}
)

func osBranchFactory(key []byte) (objectstorage.StorableObject, error, int) {
	return BranchFromStorageKey(key)
}
