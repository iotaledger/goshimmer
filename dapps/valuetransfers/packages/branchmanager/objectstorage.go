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
	osChildBranch
	osConflict
	osConflictMember
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

	osChildBranchOptions = []objectstorage.Option{
		objectstorage.CacheTime(60 * time.Second),
		objectstorage.PartitionKey(BranchIDLength, BranchIDLength),
		osLeakDetectionOption,
	}

	osConflictOptions = []objectstorage.Option{
		objectstorage.CacheTime(60 * time.Second),
		osLeakDetectionOption,
	}

	osConflictMemberOptions = []objectstorage.Option{
		objectstorage.CacheTime(60 * time.Second),
		objectstorage.PartitionKey(ConflictIDLength, BranchIDLength),
		osLeakDetectionOption,
	}
)

func osBranchFactory(key []byte) (objectstorage.StorableObject, int, error) {
	return BranchFromStorageKey(key)
}

func osChildBranchFactory(key []byte) (objectstorage.StorableObject, int, error) {
	return ChildBranchFromStorageKey(key)
}

func osConflictFactory(key []byte) (objectstorage.StorableObject, int, error) {
	return ConflictFromStorageKey(key)
}

func osConflictMemberFactory(key []byte) (objectstorage.StorableObject, int, error) {
	return ConflictMemberFromStorageKey(key)
}
