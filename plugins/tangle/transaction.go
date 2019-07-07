package tangle

import (
	"github.com/dgraph-io/badger"
	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/datastructure"
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/typeutils"
	"github.com/iotaledger/goshimmer/packages/unsafeconvert"
	"github.com/iotaledger/iota.go/trinary"
)

// region public api ///////////////////////////////////////////////////////////////////////////////////////////////////

func GetTransaction(transactionHash trinary.Trytes, computeIfAbsent ...func(trinary.Trytes) *value_transaction.ValueTransaction) (result *value_transaction.ValueTransaction, err errors.IdentifiableError) {
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
	}); !typeutils.IsInterfaceNil(cacheResult) {
		result = cacheResult.(*value_transaction.ValueTransaction)
	}

	return
}

func ContainsTransaction(transactionHash trinary.Trytes) (result bool, err errors.IdentifiableError) {
	if transactionCache.Contains(transactionHash) {
		result = true
	} else {
		result, err = databaseContainsTransaction(transactionHash)
	}

	return
}

func StoreTransaction(transaction *value_transaction.ValueTransaction) {
	transactionCache.Set(transaction.GetHash(), transaction)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region lru cache ////////////////////////////////////////////////////////////////////////////////////////////////////

var transactionCache = datastructure.NewLRUCache(TRANSACTION_CACHE_SIZE, &datastructure.LRUCacheOptions{
	EvictionCallback: onEvictTransaction,
})

func onEvictTransaction(_ interface{}, value interface{}) {
	if evictedTransaction := value.(*value_transaction.ValueTransaction); evictedTransaction.GetModified() {
		go func(evictedTransaction *value_transaction.ValueTransaction) {
			if err := storeTransactionInDatabase(evictedTransaction); err != nil {
				panic(err)
			}
		}(evictedTransaction)
	}
}

const (
	TRANSACTION_CACHE_SIZE = 50000
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region database /////////////////////////////////////////////////////////////////////////////////////////////////////

var transactionDatabase database.Database

func configureTransactionDatabase(plugin *node.Plugin) {
	if db, err := database.Get("transaction"); err != nil {
		panic(err)
	} else {
		transactionDatabase = db
	}
}

func storeTransactionInDatabase(transaction *value_transaction.ValueTransaction) errors.IdentifiableError {
	if transaction.GetModified() {
		if err := transactionDatabase.Set(unsafeconvert.StringToBytes(transaction.GetHash()), transaction.MetaTransaction.GetBytes()); err != nil {
			return ErrDatabaseError.Derive(err, "failed to store transaction")
		}

		transaction.SetModified(false)
	}

	return nil
}

func getTransactionFromDatabase(transactionHash trinary.Trytes) (*value_transaction.ValueTransaction, errors.IdentifiableError) {
	txData, err := transactionDatabase.Get(unsafeconvert.StringToBytes(transactionHash))
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return nil, nil
		} else {
			return nil, ErrDatabaseError.Derive(err, "failed to retrieve transaction")
		}
	}

	return value_transaction.FromBytes(txData), nil
}

func databaseContainsTransaction(transactionHash trinary.Trytes) (bool, errors.IdentifiableError) {
	if contains, err := transactionDatabase.Contains(unsafeconvert.StringToBytes(transactionHash)); err != nil {
		return contains, ErrDatabaseError.Derive(err, "failed to check if the transaction exists")
	} else {
		return contains, nil
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
