package ledgerstate

import (
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/packages/binary/storageprefix"
)

type LedgerState struct {
	attachmentStorage *objectstorage.ObjectStorage

	outputStorage        *objectstorage.ObjectStorage
	consumerStorage      *objectstorage.ObjectStorage
	missingOutputStorage *objectstorage.ObjectStorage
	branchStorage        *objectstorage.ObjectStorage
}

func New(badgerInstance *badger.DB) (result *LedgerState) {
	osFactory := objectstorage.NewFactory(badgerInstance, storageprefix.ValueTransfers)

	result = &LedgerState{
		// transaction related storage
		attachmentStorage:    osFactory.New(osAttachment, osAttachmentFactory, objectstorage.CacheTime(time.Second)),
		outputStorage:        osFactory.New(osOutput, osOutputFactory, OutputKeyPartitions, objectstorage.CacheTime(time.Second)),
		missingOutputStorage: osFactory.New(osMissingOutput, osMissingOutputFactory, MissingOutputKeyPartitions, objectstorage.CacheTime(time.Second)),
		consumerStorage:      osFactory.New(osConsumer, osConsumerFactory, ConsumerPartitionKeys, objectstorage.CacheTime(time.Second)),

		Events: *newEvents(),
	}

	return
}
