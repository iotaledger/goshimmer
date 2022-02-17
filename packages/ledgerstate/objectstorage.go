package ledgerstate

import (
	"time"

	genericobjectstorage "github.com/iotaledger/hive.go/generics/objectstorage"

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
	branchStorageOptions []genericobjectstorage.Option

	// childBranchStorageOptions contains a list of default settings for the ChildBranch object storage.
	childBranchStorageOptions []genericobjectstorage.Option

	// conflictStorageOptions contains a list of default settings for the Conflict object storage.
	conflictStorageOptions []genericobjectstorage.Option

	// conflictMemberStorageOptions contains a list of default settings for the ConflictMember object storage.
	conflictMemberStorageOptions []genericobjectstorage.Option

	// transactionStorageOptions contains a list of default settings for the Transaction object storage.
	transactionStorageOptions []genericobjectstorage.Option

	// transactionMetadataStorageOptions contains a list of default settings for the TransactionMetadata object storage.
	transactionMetadataStorageOptions []genericobjectstorage.Option

	// outputStorageOptions contains a list of default settings for the Output object storage.
	outputStorageOptions []genericobjectstorage.Option

	// outputMetadataStorageOptions contains a list of default settings for the OutputMetadata object storage.
	outputMetadataStorageOptions []genericobjectstorage.Option

	// consumerStorageOptions contains a list of default settings for the Consumer object storage.
	consumerStorageOptions []genericobjectstorage.Option

	// addressOutputMappingStorageOptions contains a list of default settings for the AddressOutputMapping object storage.
	addressOutputMappingStorageOptions []genericobjectstorage.Option
}

func buildObjectStorageOptions(cacheProvider *database.CacheTimeProvider) *storageOptions {
	options := storageOptions{}

	options.branchStorageOptions = []genericobjectstorage.Option{
		cacheProvider.CacheTime(branchCacheTime),
		genericobjectstorage.LeakDetectionEnabled(false),
	}

	options.childBranchStorageOptions = []genericobjectstorage.Option{
		ChildBranchKeyPartition,
		cacheProvider.CacheTime(branchCacheTime),
		genericobjectstorage.LeakDetectionEnabled(false),
		genericobjectstorage.StoreOnCreation(true),
	}

	options.conflictStorageOptions = []genericobjectstorage.Option{
		cacheProvider.CacheTime(consumerCacheTime),
		genericobjectstorage.LeakDetectionEnabled(false),
	}

	options.conflictMemberStorageOptions = []genericobjectstorage.Option{
		ConflictMemberKeyPartition,
		cacheProvider.CacheTime(conflictCacheTime),
		genericobjectstorage.LeakDetectionEnabled(false),
		genericobjectstorage.StoreOnCreation(true),
	}

	options.transactionStorageOptions = []genericobjectstorage.Option{
		cacheProvider.CacheTime(transactionCacheTime),
		genericobjectstorage.LeakDetectionEnabled(false),
		genericobjectstorage.StoreOnCreation(true),
	}

	options.transactionMetadataStorageOptions = []genericobjectstorage.Option{
		cacheProvider.CacheTime(transactionCacheTime),
		genericobjectstorage.LeakDetectionEnabled(false),
	}

	options.outputStorageOptions = []genericobjectstorage.Option{
		cacheProvider.CacheTime(outputCacheTime),
		genericobjectstorage.LeakDetectionEnabled(false),
		genericobjectstorage.StoreOnCreation(true),
	}

	options.outputMetadataStorageOptions = []genericobjectstorage.Option{
		cacheProvider.CacheTime(outputCacheTime),
		genericobjectstorage.LeakDetectionEnabled(false),
	}

	options.consumerStorageOptions = []genericobjectstorage.Option{
		ConsumerPartitionKeys,
		cacheProvider.CacheTime(consumerCacheTime),
		genericobjectstorage.LeakDetectionEnabled(false),
	}

	options.addressOutputMappingStorageOptions = []genericobjectstorage.Option{
		cacheProvider.CacheTime(addressCacheTime),
		genericobjectstorage.PartitionKey(AddressLength, OutputIDLength),
		genericobjectstorage.LeakDetectionEnabled(false),
		genericobjectstorage.StoreOnCreation(true),
	}

	return &options
}
