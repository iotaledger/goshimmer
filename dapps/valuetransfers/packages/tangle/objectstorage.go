package tangle

import (
	"time"

	"github.com/iotaledger/hive.go/objectstorage"
)

const (
	// the following values are a list of prefixes defined as an enum
	_ byte = iota

	// prefixes used for the objectstorage
	osPayload
	osPayloadMetadata
	osMissingPayload
	osApprover
	osTransaction
	osTransactionMetadata
	osAttachment
	osOutput
	osConsumer

	cacheTime = 20 * time.Second
)

var (
	osLeakDetectionOption = objectstorage.LeakDetectionEnabled(false, objectstorage.LeakDetectionOptions{
		MaxConsumersPerObject: 20,
		MaxConsumerHoldTime:   10 * time.Second,
	})
)
