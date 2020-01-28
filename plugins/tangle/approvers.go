package tangle

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/model/approvers"
	"github.com/iotaledger/hive.go/lru_cache"
	"github.com/iotaledger/hive.go/typeutils"
	"github.com/iotaledger/iota.go/trinary"
)

// region global public api ////////////////////////////////////////////////////////////////////////////////////////////

// GetApprovers retrieves approvers from the database.
func GetApprovers(transactionHash trinary.Trytes, computeIfAbsent ...func(trinary.Trytes) *approvers.Approvers) (result *approvers.Approvers, err error) {
	if cacheResult := approversCache.ComputeIfAbsent(transactionHash, func() interface{} {
		if dbApprovers, dbErr := getApproversFromDatabase(transactionHash); dbErr != nil {
			err = dbErr

			return nil
		} else if dbApprovers != nil {
			return dbApprovers
		} else {
			if len(computeIfAbsent) >= 1 {
				return computeIfAbsent[0](transactionHash)
			}

			return nil
		}
	}); cacheResult != nil && cacheResult.(*approvers.Approvers) != nil {
		result = cacheResult.(*approvers.Approvers)
	}

	return
}

func ContainsApprovers(transactionHash trinary.Trytes) (result bool, err error) {
	if approversCache.Contains(transactionHash) {
		result = true
	} else {
		result, err = databaseContainsApprovers(transactionHash)
	}

	return
}

func StoreApprovers(approvers *approvers.Approvers) {
	approversCache.Set(approvers.GetHash(), approvers)
}

// region lru cache ////////////////////////////////////////////////////////////////////////////////////////////////////

var approversCache = lru_cache.NewLRUCache(APPROVERS_CACHE_SIZE, &lru_cache.LRUCacheOptions{
	EvictionCallback:  onEvictApprovers,
	EvictionBatchSize: 100,
})

func onEvictApprovers(_ interface{}, values interface{}) {
	// TODO: replace with apply
	for _, obj := range values.([]interface{}) {
		if approvers := obj.(*approvers.Approvers); approvers.GetModified() {
			if err := storeApproversInDatabase(approvers); err != nil {
				panic(err)
			}
		}
	}
}

func FlushApproversCache() {
	approversCache.DeleteAll()
}

const (
	APPROVERS_CACHE_SIZE = 50000
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region database /////////////////////////////////////////////////////////////////////////////////////////////////////

var approversDatabase database.Database

func configureApproversDatabase() {
	if db, err := database.Get(database.DBPrefixApprovers, database.GetBadgerInstance()); err != nil {
		panic(err)
	} else {
		approversDatabase = db
	}
}

func storeApproversInDatabase(approvers *approvers.Approvers) error {
	if approvers.GetModified() {
		if err := approversDatabase.Set(database.Entry{Key: typeutils.StringToBytes(approvers.GetHash()), Value: approvers.Marshal()}); err != nil {
			return fmt.Errorf("%w: failed to store approvers: %s", ErrDatabaseError, err)
		}
		approvers.SetModified(false)
	}

	return nil
}

func getApproversFromDatabase(transactionHash trinary.Trytes) (*approvers.Approvers, error) {
	approversData, err := approversDatabase.Get(typeutils.StringToBytes(transactionHash))
	if err != nil {
		if err == database.ErrKeyNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("%w: failed to retrieve approvers: %s", ErrDatabaseError, err)
	}

	var result approvers.Approvers
	if err = result.Unmarshal(approversData.Value); err != nil {
		panic(err)
	}

	return &result, nil
}

func databaseContainsApprovers(transactionHash trinary.Trytes) (bool, error) {
	if contains, err := approversDatabase.Contains(typeutils.StringToBytes(transactionHash)); err != nil {
		return false, fmt.Errorf("%w: failed to check if the approvers exist: %s", ErrDatabaseError, err)
	} else {
		return contains, nil
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
