package ledger

import (
	"github.com/iotaledger/hive.go/generics/objectstorage"

	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

type CachedOutputs = objectstorage.CachedObjects[*Output]

type CachedOutput = *objectstorage.CachedObject[*Output]

type CachedOutputMetadata = *objectstorage.CachedObject[*OutputMetadata]

type CachedOutputsMetadata = objectstorage.CachedObjects[*OutputMetadata]

type CachedConsumer = *objectstorage.CachedObject[*Consumer]

type CachedConsumers = objectstorage.CachedObjects[*Consumer]

type CachedTransaction = *objectstorage.CachedObject[*Transaction]

type CachedTransactions = *objectstorage.CachedObjects[*Transaction]

type Input = utxo.Input

type TransactionID = utxo.TransactionID

type TransactionIDs = utxo.TransactionIDs

type OutputID = utxo.OutputID

type OutputIDs = utxo.OutputIDs

var NewOutputIDs = utxo.NewOutputIDs

var NewTransactionIDs = utxo.NewTransactionIDs

var EmptyTransactionID = utxo.EmptyTransactionID
