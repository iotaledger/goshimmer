package tangle

import (
	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/hive.go/lru_cache"
	"github.com/iotaledger/hive.go/typeutils"
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

var transactionCache = lru_cache.NewLRUCache(TRANSACTION_CACHE_SIZE, &lru_cache.LRUCacheOptions{
	EvictionCallback:  onEvictTransactions,
	EvictionBatchSize: 200,
})

func onEvictTransactions(_ interface{}, values interface{}) {
	// TODO: replace with apply
	for _, obj := range values.([]interface{}) {
		if tx := obj.(*value_transaction.ValueTransaction); tx.GetModified() {
			if err := storeTransactionInDatabase(tx); err != nil {
				panic(err)
			}
		}
	}
}

func FlushTransactionCache() {
	transactionCache.DeleteAll()
}

const (
	TRANSACTION_CACHE_SIZE = 500
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region database /////////////////////////////////////////////////////////////////////////////////////////////////////

var transactionDatabase database.Database

func configureTransactionDatabase() {
	if db, err := database.Get("transaction"); err != nil {
		panic(err)
	} else {
		transactionDatabase = db
	}
}

func storeTransactionInDatabase(transaction *value_transaction.ValueTransaction) errors.IdentifiableError {
	if transaction.GetModified() {
		if err := transactionDatabase.Set(typeutils.StringToBytes(transaction.GetHash()), transaction.MetaTransaction.GetBytes()); err != nil {
			return ErrDatabaseError.Derive(err, "failed to store transaction")
		}

		transaction.SetModified(false)
	}

	return nil
}

func getTransactionFromDatabase(transactionHash trinary.Trytes) (*value_transaction.ValueTransaction, errors.IdentifiableError) {
	txData, err := transactionDatabase.Get(typeutils.StringToBytes(transactionHash))
	if err != nil {
		if err == database.ErrKeyNotFound {
			return nil, nil
		} else {
			return nil, ErrDatabaseError.Derive(err, "failed to retrieve transaction")
		}
	}

	return value_transaction.FromBytes(txData), nil
}

func databaseContainsTransaction(transactionHash trinary.Trytes) (bool, errors.IdentifiableError) {
	if contains, err := transactionDatabase.Contains(typeutils.StringToBytes(transactionHash)); err != nil {
		return contains, ErrDatabaseError.Derive(err, "failed to check if the transaction exists")
	} else {
		return contains, nil
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
