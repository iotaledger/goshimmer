package tangle

import (
	"fmt"

	"github.com/dgraph-io/badger"
	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/ternary"
	"github.com/iotaledger/goshimmer/packages/transaction"
)

// region plugin module setup //////////////////////////////////////////////////////////////////////////////////////////

func configureDatabase(plugin *node.Plugin) {
	if db, err := database.Get("transaction"); err != nil {
		panic(err)
	} else {
		transactionDatabase = db
	}

	if db, err := database.Get("transactionMetadata"); err != nil {
		panic(err)
	} else {
		transactionMetadataDatabase = db
	}

	if db, err := database.Get("approvers"); err != nil {
		panic(err)
	} else {
		approversDatabase = db
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region internal utility functions ///////////////////////////////////////////////////////////////////////////////////

func getTransactionFromDatabase(transactionHash ternary.Trinary) (*Transaction, errors.IdentifiableError) {
	txData, err := transactionDatabase.Get(transactionHash.CastToBytes())
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return nil, nil
		} else {
			return nil, ErrDatabaseError.Derive(err, "failed to retrieve transaction")
		}
	}

	return &Transaction{
		rawTransaction: transaction.FromBytes(txData),
	}, nil
}

func databaseContainsTransaction(transactionHash ternary.Trinary) (bool, errors.IdentifiableError) {
	if contains, err := transactionDatabase.Contains(transactionHash.CastToBytes()); err != nil {
		return contains, ErrDatabaseError.Derive(err, "failed to check if the transaction exists")
	} else {
		return contains, nil
	}
}

func getTransactionMetadataFromDatabase(transactionHash ternary.Trinary) (*TransactionMetadata, errors.IdentifiableError) {
	txMetadata, err := transactionMetadataDatabase.Get(transactionHash.CastToBytes())
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return nil, nil
		} else {
			return nil, ErrDatabaseError.Derive(err, "failed to retrieve transaction")
		}
	}

	if false {
		fmt.Println(txMetadata)
	}

	return &TransactionMetadata{}, nil
}

func databaseContainsTransactionMetadata(transactionHash ternary.Trinary) (bool, errors.IdentifiableError) {
	if contains, err := transactionMetadataDatabase.Contains(transactionHash.CastToBytes()); err != nil {
		return contains, ErrDatabaseError.Derive(err, "failed to check if the transaction metadata exists")
	} else {
		return contains, nil
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region constants and variables //////////////////////////////////////////////////////////////////////////////////////

var transactionDatabase database.Database

var transactionMetadataDatabase database.Database

var approversDatabase database.Database

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
