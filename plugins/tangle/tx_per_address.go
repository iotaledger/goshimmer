package tangle

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/hive.go/typeutils"
	"github.com/iotaledger/iota.go/trinary"
)

var (
	transactionsHashesForAddressDatabase database.Database
)

func configureTransactionHashesForAddressDatabase() {
	if db, err := database.Get("transactionsHashesForAddress"); err != nil {
		panic(err)
	} else {
		transactionsHashesForAddressDatabase = db
	}
}

type TxHashForAddress struct {
	Address trinary.Hash
	TxHash  trinary.Hash
}

func StoreTransactionHashForAddressInDatabase(address *TxHashForAddress) error {
	if err := transactionsHashesForAddressDatabase.Set(
		databaseKeyForHashPrefixedHash(address.Address, address.TxHash),
		[]byte{},
	); err != nil {
		return fmt.Errorf("%w: failed to store tx for address in database: %s", ErrDatabaseError, err)
	}
	return nil
}

func DeleteTransactionHashForAddressInDatabase(address *TxHashForAddress) error {
	if err := transactionsHashesForAddressDatabase.Delete(
		databaseKeyForHashPrefixedHash(address.Address, address.TxHash),
	); err != nil {
		return fmt.Errorf("%w: failed to delete tx for address: %s", ErrDatabaseError, err)
	}

	return nil
}

func ReadTransactionHashesForAddressFromDatabase(address trinary.Hash) ([]trinary.Hash, error) {
	var transactionHashes []trinary.Hash
	err := transactionsHashesForAddressDatabase.ForEachWithPrefix(databaseKeyForHashPrefix(address), func(key []byte, value []byte) {
		k := typeutils.BytesToString(key)
		if len(k) > 81 {
			transactionHashes = append(transactionHashes, k[81:])
		}
	})

	if err != nil {
		return nil, fmt.Errorf("%w: failed to read tx per address from database: %s", ErrDatabaseError, err)
	}
	return transactionHashes, nil
}
