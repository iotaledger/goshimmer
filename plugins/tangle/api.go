package tangle

import (
	"github.com/iotaledger/goshimmer/packages/datastructure"
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/ternary"
)

// region transaction api //////////////////////////////////////////////////////////////////////////////////////////////

var transactionCache = datastructure.NewLRUCache(TRANSACTION_CACHE_SIZE, &datastructure.LRUCacheOptions{
	EvictionCallback: func(key interface{}, value interface{}) {
		go func(evictedTransaction *value_transaction.ValueTransaction) {
			if err := storeTransactionInDatabase(evictedTransaction); err != nil {
				panic(err)
			}
		}(value.(*value_transaction.ValueTransaction))
	},
})

func StoreTransaction(transaction *value_transaction.ValueTransaction) {
	transactionCache.Set(transaction.GetHash(), transaction)
}

func GetTransaction(transactionHash ternary.Trinary, computeIfAbsent ...func(ternary.Trinary) *value_transaction.ValueTransaction) (result *value_transaction.ValueTransaction, err errors.IdentifiableError) {
	if cacheResult := transactionCache.ComputeIfAbsent(transactionHash, func() interface{} {
		if transaction, dbErr := getTransactionFromDatabase(transactionHash); dbErr != nil {
			err = dbErr

			return nil
		} else if transaction != nil {
			return transaction
		} else {
			if len(computeIfAbsent) >= 1 {
				return computeIfAbsent[0](transactionHash)
			}

			return nil
		}
	}); cacheResult != nil {
		result = cacheResult.(*value_transaction.ValueTransaction)
	}

	return
}

func ContainsTransaction(transactionHash ternary.Trinary) (result bool, err errors.IdentifiableError) {
	if transactionCache.Get(transactionHash) != nil {
		result = true
	} else {
		result, err = databaseContainsTransaction(transactionHash)
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region transactionmetadata api //////////////////////////////////////////////////////////////////////////////////////

var metadataCache = datastructure.NewLRUCache(METADATA_CACHE_SIZE, &datastructure.LRUCacheOptions{
	EvictionCallback: func(key interface{}, value interface{}) {
		go func(evictedMetadata *TransactionMetadata) {
			if err := storeTransactionMetadataInDatabase(evictedMetadata); err != nil {
				panic(err)
			}
		}(value.(*TransactionMetadata))
	},
})

func GetTransactionMetadata(transactionHash ternary.Trinary, computeIfAbsent ...func(ternary.Trinary) *TransactionMetadata) (result *TransactionMetadata, err errors.IdentifiableError) {
	if transactionMetadata := metadataCache.ComputeIfAbsent(transactionHash, func() interface{} {
		if result, err = getTransactionMetadataFromDatabase(transactionHash); err == nil && result == nil && len(computeIfAbsent) >= 1 {
			result = computeIfAbsent[0](transactionHash)
		}

		return result
	}); transactionMetadata != nil && transactionMetadata.(*TransactionMetadata) != nil {
		result = transactionMetadata.(*TransactionMetadata)
	}

	return
}

func ContainsTransactionMetadata(transactionHash ternary.Trinary) (result bool, err errors.IdentifiableError) {
	if metadataCache.Get(transactionHash) != nil {
		result = true
	} else {
		result, err = databaseContainsTransactionMetadata(transactionHash)
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

const (
	TRANSACTION_CACHE_SIZE = 10000
	METADATA_CACHE_SIZE    = 10000
)
