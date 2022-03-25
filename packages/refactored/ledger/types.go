package ledger

import (
	"github.com/iotaledger/hive.go/generics/objectstorage"
)

type CachedOutputs = objectstorage.CachedObjects[*Output]

type CachedOutput = *objectstorage.CachedObject[*Output]

type CachedOutputMetadata = *objectstorage.CachedObject[*OutputMetadata]

type CachedOutputsMetadata = objectstorage.CachedObjects[*OutputMetadata]

type CachedConsumer = *objectstorage.CachedObject[*Consumer]

type CachedConsumers = objectstorage.CachedObjects[*Consumer]

type CachedTransaction = *objectstorage.CachedObject[*Transaction]

type CachedTransactions = *objectstorage.CachedObjects[*Transaction]
