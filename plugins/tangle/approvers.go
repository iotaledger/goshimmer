package tangle

import (
	"github.com/dgraph-io/badger"
	"github.com/iotaledger/goshimmer/packages/datastructure"
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/model/approvers"
	"github.com/iotaledger/goshimmer/packages/ternary"
)

// region global public api ////////////////////////////////////////////////////////////////////////////////////////////

var approversCache = datastructure.NewLRUCache(METADATA_CACHE_SIZE)

func StoreApprovers(approvers *approvers.Approvers) {
	hash := approvers.GetHash()

	approversCache.Set(hash, approvers)
}

func GetApprovers(transactionHash ternary.Trinary, computeIfAbsent ...func(ternary.Trinary) *approvers.Approvers) (result *approvers.Approvers, err errors.IdentifiableError) {
	if appr := approversCache.ComputeIfAbsent(transactionHash, func() interface{} {
		if result, err = getApproversFromDatabase(transactionHash); err == nil && result == nil && len(computeIfAbsent) >= 1 {
			result = computeIfAbsent[0](transactionHash)
		}

		return result
	}); appr != nil && appr.(*approvers.Approvers) != nil {
		result = appr.(*approvers.Approvers)
	}

	return
}

func ContainsApprovers(transactionHash ternary.Trinary) (result bool, err errors.IdentifiableError) {
	if approversCache.Contains(transactionHash) {
		result = true
	} else {
		result, err = databaseContainsApprovers(transactionHash)
	}

	return
}

func getApproversFromDatabase(transactionHash ternary.Trinary) (result *approvers.Approvers, err errors.IdentifiableError) {
	approversData, dbErr := approversDatabase.Get(transactionHash.CastToBytes())
	if dbErr != nil {
		if dbErr != badger.ErrKeyNotFound {
			err = ErrDatabaseError.Derive(err, "failed to retrieve transaction")
		}

		return
	}

	result = approvers.New(transactionHash)
	if err = result.Unmarshal(approversData); err != nil {
		result = nil
	}

	return
}

func databaseContainsApprovers(transactionHash ternary.Trinary) (bool, errors.IdentifiableError) {
	result, err := approversDatabase.Contains(transactionHash.CastToBytes())
	if err != nil {
		return false, ErrDatabaseError.Derive(err, "failed to check if the transaction exists")
	}

	return result, nil
}
