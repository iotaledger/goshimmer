package ledgerstate

import (
	"time"

	"github.com/iotaledger/hive.go/generics/objectstorage"

	"github.com/iotaledger/goshimmer/packages/database"
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

// block of default cache time.
const (
	branchCacheTime      = 60 * time.Second
	conflictCacheTime    = 60 * time.Second
	transactionCacheTime = 10 * time.Second
	outputCacheTime      = 10 * time.Second
	consumerCacheTime    = 10 * time.Second
	addressCacheTime     = 10 * time.Second
)

type storageOptions struct {
	// branchStorageOptions contains a list of default settings for the Branch object storage.
	branchStorageOptions []objectstorage.Option

	// childBranchStorageOptions contains a list of default settings for the ChildBranch object storage.
	childBranchStorageOptions []objectstorage.Option

	// conflictStorageOptions contains a list of default settings for the Conflict object storage.
	conflictStorageOptions []objectstorage.Option

	// conflictMemberStorageOptions contains a list of default settings for the ConflictMember object storage.
	conflictMemberStorageOptions []objectstorage.Option

	// transactionStorageOptions contains a list of default settings for the Transaction object storage.
	transactionStorageOptions []objectstorage.Option

	// transactionMetadataStorageOptions contains a list of default settings for the TransactionMetadata object storage.
	transactionMetadataStorageOptions []objectstorage.Option

	// outputStorageOptions contains a list of default settings for the Output object storage.
	outputStorageOptions []objectstorage.Option

	// outputMetadataStorageOptions contains a list of default settings for the OutputMetadata object storage.
	outputMetadataStorageOptions []objectstorage.Option

	// consumerStorageOptions contains a list of default settings for the Consumer object storage.
	consumerStorageOptions []objectstorage.Option

	// addressOutputMappingStorageOptions contains a list of default settings for the AddressOutputMapping object storage.
	addressOutputMappingStorageOptions []objectstorage.Option
}

func buildObjectStorageOptions(cacheProvider *database.CacheTimeProvider) *storageOptions {
	options := storageOptions{}

	options.branchStorageOptions = []objectstorage.Option{
		cacheProvider.CacheTime(branchCacheTime),
		objectstorage.LeakDetectionEnabled(false),
	}

	options.childBranchStorageOptions = []objectstorage.Option{
		ChildBranchKeyPartition,
		cacheProvider.CacheTime(branchCacheTime),
		objectstorage.LeakDetectionEnabled(false),
		objectstorage.StoreOnCreation(true),
	}

	options.conflictStorageOptions = []objectstorage.Option{
		cacheProvider.CacheTime(consumerCacheTime),
		objectstorage.LeakDetectionEnabled(false),
	}

	options.conflictMemberStorageOptions = []objectstorage.Option{
		ConflictMemberKeyPartition,
		cacheProvider.CacheTime(conflictCacheTime),
		objectstorage.LeakDetectionEnabled(false),
		objectstorage.StoreOnCreation(true),
	}

	options.transactionStorageOptions = []objectstorage.Option{
		cacheProvider.CacheTime(transactionCacheTime),
		objectstorage.LeakDetectionEnabled(false),
		objectstorage.StoreOnCreation(true),
	}

	options.transactionMetadataStorageOptions = []objectstorage.Option{
		cacheProvider.CacheTime(transactionCacheTime),
		objectstorage.LeakDetectionEnabled(false),
	}

	options.outputStorageOptions = []objectstorage.Option{
		cacheProvider.CacheTime(outputCacheTime),
		objectstorage.LeakDetectionEnabled(false),
		objectstorage.StoreOnCreation(true),
		objectstorage.WithObjectFactory(OutputFromObjectStorage),
	}

	options.outputMetadataStorageOptions = []objectstorage.Option{
		cacheProvider.CacheTime(outputCacheTime),
		objectstorage.LeakDetectionEnabled(false),
	}

	options.consumerStorageOptions = []objectstorage.Option{
		ConsumerPartitionKeys,
		cacheProvider.CacheTime(consumerCacheTime),
		objectstorage.LeakDetectionEnabled(false),
	}

	options.addressOutputMappingStorageOptions = []objectstorage.Option{
		cacheProvider.CacheTime(addressCacheTime),
		objectstorage.PartitionKey(AddressLength, OutputIDLength),
		objectstorage.LeakDetectionEnabled(false),
		objectstorage.StoreOnCreation(true),
	}

	return &options
}
