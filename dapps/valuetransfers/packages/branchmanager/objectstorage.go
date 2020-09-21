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

	cacheTime = 30 * time.Second
)

var (
	osLeakDetectionOption = objectstorage.LeakDetectionEnabled(false, objectstorage.LeakDetectionOptions{
		MaxConsumersPerObject: 10,
		MaxConsumerHoldTime:   10 * time.Second,
	})

	osBranchOptions = []objectstorage.Option{
		objectstorage.CacheTime(cacheTime),
		osLeakDetectionOption,
	}

	osChildBranchOptions = []objectstorage.Option{
		objectstorage.CacheTime(cacheTime),
		objectstorage.PartitionKey(BranchIDLength, BranchIDLength),
		osLeakDetectionOption,
	}

	osConflictOptions = []objectstorage.Option{
		objectstorage.CacheTime(cacheTime),
		osLeakDetectionOption,
	}

	osConflictMemberOptions = []objectstorage.Option{
		objectstorage.CacheTime(cacheTime),
		objectstorage.PartitionKey(ConflictIDLength, BranchIDLength),
		osLeakDetectionOption,
	}
)
