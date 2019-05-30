package tangle

import (
    "github.com/iotaledger/goshimmer/packages/errors"
    "github.com/iotaledger/goshimmer/packages/ternary"
)

// region transaction api //////////////////////////////////////////////////////////////////////////////////////////////

func GetTransaction(transactionHash ternary.Trinary) (*Transaction, errors.IdentifiableError) {
    if transaction := getTransactionFromMemPool(transactionHash); transaction != nil {
        return transaction, nil
    }

    return getTransactionFromDatabase(transactionHash)
}

func ContainsTransaction(transactionHash ternary.Trinary) (bool, errors.IdentifiableError) {
    if memPoolContainsTransaction(transactionHash) {
        return true, nil
    }

    return databaseContainsTransaction(transactionHash)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region transactionmetadata api //////////////////////////////////////////////////////////////////////////////////////

func GetTransactionMetadata(transactionHash ternary.Trinary) (*TransactionMetadata, errors.IdentifiableError) {
    if transaction := getTransactionFromMemPool(transactionHash); transaction != nil {
        return transaction.GetMetaData()
    }

    return getTransactionMetadataFromDatabase(transactionHash)
}

func ContainsTransactionMetadata(transactionHash ternary.Trinary) (bool, errors.IdentifiableError) {
    if memPoolContainsTransaction(transactionHash) {
        return true, nil
    }

    return databaseContainsTransactionMetadata(transactionHash)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////