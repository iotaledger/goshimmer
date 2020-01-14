package tangle

import (
	"github.com/iotaledger/goshimmer/packages/database"
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
		return ErrDatabaseError.Derive(err, "failed to store tx for address in database")
	}
	log.Info("Stored Address:", address.Address)
	log.Info("TxHash:", address.TxHash)
	log.Info("txForAddr: ", trinary.MustBytesToTrytes(databaseKeyForHashPrefixedHash(address.Address, address.TxHash), 81))
	return nil
}

func DeleteTransactionHashForAddressInDatabase(address *TxHashForAddress) error {
	if err := transactionsHashesForAddressDatabase.Delete(
		databaseKeyForHashPrefixedHash(address.Address, address.TxHash),
	); err != nil {
		return ErrDatabaseError.Derive(err, "failed to delete tx for address")
	}

	return nil
}

func ReadTransactionHashesForAddressFromDatabase(address trinary.Hash) ([]trinary.Hash, error) {
	var transactionHashes []trinary.Hash
	err := transactionsHashesForAddressDatabase.ForEachWithPrefix(databaseKeyForHashPrefix(address), func(key []byte, value []byte) {
		log.Info("Len key:", len(key))
		txHash := trinary.MustBytesToTrytes(key)[81:]
		log.Info("Len ALL:", len(txHash))
		transactionHashes = append(transactionHashes, txHash)
	})

	if err != nil {
		return nil, ErrDatabaseError.Derive(err, "failed to read tx per address from database")
	} else {
		return transactionHashes, nil
	}
}
