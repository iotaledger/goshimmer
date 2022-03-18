package ledger

import (
	"github.com/iotaledger/hive.go/generics/objectstorage"

	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

type CachedOutputs = objectstorage.CachedObjects[utxo.Output]

type CachedOutput = *objectstorage.CachedObject[utxo.Output]

type CachedOutputMetadata = *objectstorage.CachedObject[*OutputMetadata]

type CachedOutputsMetadata = objectstorage.CachedObjects[*OutputMetadata]
