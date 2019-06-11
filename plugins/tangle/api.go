package tangle

import (
	"github.com/iotaledger/goshimmer/packages/datastructure"
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/ternary"
)

// region transaction api //////////////////////////////////////////////////////////////////////////////////////////////

var transactionCache = datastructure.NewLRUCache(TRANSACTION_CACHE_SIZE)

func GetTransaction(transactionHash ternary.Trinary, computeIfAbsent ...func(ternary.Trinary) *Transaction) (result *Transaction, err errors.IdentifiableError) {
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
		result = cacheResult.(*Transaction)
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

var metadataCache = datastructure.NewLRUCache(METADATA_CACHE_SIZE)

func GetTransactionMetadata(transactionHash ternary.Trinary) (result *TransactionMetadata, err errors.IdentifiableError) {
	result = metadataCache.ComputeIfAbsent(transactionHash, func() interface{} {
		if metadata, dbErr := getTransactionMetadataFromDatabase(transactionHash); dbErr != nil {
			err = dbErr

			return nil
		} else if metadata != nil {
			return metadata
		}

		return nil
	}).(*TransactionMetadata)

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
	TRANSACTION_CACHE_SIZE = 1000
	METADATA_CACHE_SIZE    = 1000
)
