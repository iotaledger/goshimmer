package ledgerstate

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/database"

	"github.com/iotaledger/hive.go/objectstorage"
)

const (
	// PrefixBranchStorage defines the storage prefix for the Branch object storage.
	PrefixBranchStorage byte = iota

	// PrefixChildBranchStorage defines the storage prefix for the ChildBranch object storage.
	PrefixChildBranchStorage

	// PrefixConflictStorage defines the storage prefix for the Conflict object storage.
	PrefixConflictStorage

	// PrefixConflictMemberStorage defines the storage prefix for the ConflictMember object storage.
	PrefixConflictMemberStorage

	// PrefixTransactionStorage defines the storage prefix for the Transaction object storage.
	PrefixTransactionStorage

	// PrefixTransactionMetadataStorage defines the storage prefix for the TransactionMetadata object storage.
	PrefixTransactionMetadataStorage

	// PrefixOutputStorage defines the storage prefix for the Output object storage.
	PrefixOutputStorage

	// PrefixOutputMetadataStorage defines the storage prefix for the OutputMetadata object storage.
	PrefixOutputMetadataStorage

	// PrefixConsumerStorage defines the storage prefix for the Consumer object storage.
	PrefixConsumerStorage

	// PrefixAddressOutputMappingStorage defines the storage prefix for the AddressOutputMapping object storage.
	PrefixAddressOutputMappingStorage
)

// block of default cache time
const (
	branchCacheTime      = 60 * time.Second
	conflictCacheTime    = 60 * time.Second
	transactionCacheTime = 10 * time.Second
	outputCacheTime      = 10 * time.Second
	consumerCacheTime    = 10 * time.Second
	addressCacheTime     = 10 * time.Second
)

// branchStorageOptions contains a list of default settings for the Branch object storage.
var branchStorageOptions = []objectstorage.Option{
	database.CacheTime(branchCacheTime),
	objectstorage.LeakDetectionEnabled(false),
}

// childBranchStorageOptions contains a list of default settings for the ChildBranch object storage.
var childBranchStorageOptions = []objectstorage.Option{
	ChildBranchKeyPartition,
	database.CacheTime(branchCacheTime),
	objectstorage.LeakDetectionEnabled(false),
}

// conflictStorageOptions contains a list of default settings for the Conflict object storage.
var conflictStorageOptions = []objectstorage.Option{
	database.CacheTime(consumerCacheTime),
	objectstorage.LeakDetectionEnabled(false),
}

// conflictMemberStorageOptions contains a list of default settings for the ConflictMember object storage.
var conflictMemberStorageOptions = []objectstorage.Option{
	ConflictMemberKeyPartition,
	database.CacheTime(conflictCacheTime),
	objectstorage.LeakDetectionEnabled(false),
}

// transactionStorageOptions contains a list of default settings for the Transaction object storage.
var transactionStorageOptions = []objectstorage.Option{
	database.CacheTime(transactionCacheTime),
	objectstorage.LeakDetectionEnabled(false),
}

// transactionMetadataStorageOptions contains a list of default settings for the TransactionMetadata object storage.
var transactionMetadataStorageOptions = []objectstorage.Option{
	database.CacheTime(transactionCacheTime),
	objectstorage.LeakDetectionEnabled(false),
}

// outputStorageOptions contains a list of default settings for the Output object storage.
var outputStorageOptions = []objectstorage.Option{
	database.CacheTime(outputCacheTime),
	objectstorage.LeakDetectionEnabled(false),
}

// outputMetadataStorageOptions contains a list of default settings for the OutputMetadata object storage.
var outputMetadataStorageOptions = []objectstorage.Option{
	database.CacheTime(outputCacheTime),
	objectstorage.LeakDetectionEnabled(false),
}

// consumerStorageOptions contains a list of default settings for the OutputMetadata object storage.
var consumerStorageOptions = []objectstorage.Option{
	ConsumerPartitionKeys,
	database.CacheTime(consumerCacheTime),
	objectstorage.LeakDetectionEnabled(false),
}

// addressOutputMappingStorageOptions contains a list of default settings for the AddressOutputMapping object storage.
var addressOutputMappingStorageOptions = []objectstorage.Option{
	database.CacheTime(addressCacheTime),
	objectstorage.PartitionKey(AddressLength, OutputIDLength),
	objectstorage.LeakDetectionEnabled(false),
}
