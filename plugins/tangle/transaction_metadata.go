package tangle

import (
	"github.com/dgraph-io/badger"
	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/datastructure"
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/model/transactionmetadata"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/typeutils"
	"github.com/iotaledger/goshimmer/packages/unsafeconvert"
	"github.com/iotaledger/iota.go/trinary"
)

// region public api ///////////////////////////////////////////////////////////////////////////////////////////////////

func GetTransactionMetadata(transactionHash trinary.Trytes, computeIfAbsent ...func(trinary.Trytes) *transactionmetadata.TransactionMetadata) (result *transactionmetadata.TransactionMetadata, err errors.IdentifiableError) {
	if cacheResult := transactionMetadataCache.ComputeIfAbsent(transactionHash, func() interface{} {
		if transactionMetadata, dbErr := getTransactionMetadataFromDatabase(transactionHash); dbErr != nil {
			err = dbErr

			return nil
		} else if transactionMetadata != nil {
			return transactionMetadata
		} else {
			if len(computeIfAbsent) >= 1 {
				return computeIfAbsent[0](transactionHash)
			}

			return nil
		}
	}); !typeutils.IsInterfaceNil(cacheResult) {
		result = cacheResult.(*transactionmetadata.TransactionMetadata)
	}

	return
}

func ContainsTransactionMetadata(transactionHash trinary.Trytes) (result bool, err errors.IdentifiableError) {
	if transactionMetadataCache.Contains(transactionHash) {
		result = true
	} else {
		result, err = databaseContainsTransactionMetadata(transactionHash)
	}

	return
}

func StoreTransactionMetadata(transactionMetadata *transactionmetadata.TransactionMetadata) {
	transactionMetadataCache.Set(transactionMetadata.GetHash(), transactionMetadata)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region lru cache ////////////////////////////////////////////////////////////////////////////////////////////////////

var transactionMetadataCache = datastructure.NewLRUCache(TRANSACTION_METADATA_CACHE_SIZE, &datastructure.LRUCacheOptions{
	EvictionCallback: onEvictTransactionMetadata,
})

func onEvictTransactionMetadata(_ interface{}, value interface{}) {
	if evictedTransactionMetadata := value.(*transactionmetadata.TransactionMetadata); evictedTransactionMetadata.GetModified() {
		go func(evictedTransactionMetadata *transactionmetadata.TransactionMetadata) {
			if err := storeTransactionMetadataInDatabase(evictedTransactionMetadata); err != nil {
				panic(err)
			}
		}(evictedTransactionMetadata)
	}
}

const (
	TRANSACTION_METADATA_CACHE_SIZE = 50000
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region database /////////////////////////////////////////////////////////////////////////////////////////////////////

var transactionMetadataDatabase database.Database

func configureTransactionMetaDataDatabase(plugin *node.Plugin) {
	if db, err := database.Get("transactionMetadata"); err != nil {
		panic(err)
	} else {
		transactionMetadataDatabase = db
	}
}

func storeTransactionMetadataInDatabase(metadata *transactionmetadata.TransactionMetadata) errors.IdentifiableError {
	if metadata.GetModified() {
		if marshaledMetadata, err := metadata.Marshal(); err != nil {
			return err
		} else {
			if err := transactionMetadataDatabase.Set(unsafeconvert.StringToBytes(metadata.GetHash()), marshaledMetadata); err != nil {
				return ErrDatabaseError.Derive(err, "failed to store transaction metadata")
			}

			metadata.SetModified(false)
		}
	}

	return nil
}

func getTransactionMetadataFromDatabase(transactionHash trinary.Trytes) (*transactionmetadata.TransactionMetadata, errors.IdentifiableError) {
	txMetadata, err := transactionMetadataDatabase.Get(unsafeconvert.StringToBytes(transactionHash))
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return nil, nil
		} else {
			return nil, ErrDatabaseError.Derive(err, "failed to retrieve transaction")
		}
	}

	var result transactionmetadata.TransactionMetadata
	if err := result.Unmarshal(txMetadata); err != nil {
		panic(err)
	}

	return &result, nil
}

func databaseContainsTransactionMetadata(transactionHash trinary.Trytes) (bool, errors.IdentifiableError) {
	if contains, err := transactionMetadataDatabase.Contains(unsafeconvert.StringToBytes(transactionHash)); err != nil {
		return contains, ErrDatabaseError.Derive(err, "failed to check if the transaction metadata exists")
	} else {
		return contains, nil
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
